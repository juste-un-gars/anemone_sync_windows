//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cloudfiles"
	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	"go.uber.org/zap"
	"golang.org/x/sys/windows"
)

// Runner executes test scenarios.
type Runner struct {
	cfg       *Config
	verbose   bool
	logger    *zap.Logger
	smbClient *smb.SMBClient

	// Cloud Files provider (handles sync root, callbacks, hydration, dehydration)
	provider *cloudfiles.CloudFilesProvider
}

// smbClientWrapper adapts smb.SMBClient to cloudfiles.SMBFileClient interface.
type smbClientWrapper struct {
	client *smb.SMBClient
}

func (w *smbClientWrapper) OpenFile(remotePath string) (io.ReadCloser, error) {
	return w.client.OpenFile(remotePath)
}

func (w *smbClientWrapper) ReadFile(remotePath string) ([]byte, error) {
	return w.client.ReadFile(remotePath)
}

func (w *smbClientWrapper) ListRemote(remotePath string) ([]cloudfiles.SMBRemoteFileInfo, error) {
	infos, err := w.client.ListRemote(remotePath)
	if err != nil {
		return nil, err
	}
	result := make([]cloudfiles.SMBRemoteFileInfo, len(infos))
	for i, info := range infos {
		result[i] = cloudfiles.SMBRemoteFileInfo{
			Name:    info.Name,
			Path:    info.Path,
			Size:    info.Size,
			ModTime: info.ModTime,
			IsDir:   info.IsDir,
		}
	}
	return result, nil
}

func (w *smbClientWrapper) IsConnected() bool {
	return w.client.IsConnected()
}

// NewRunner creates a new test runner.
func NewRunner(cfg *Config, verbose bool) (*Runner, error) {
	// Create logger
	var logger *zap.Logger
	if verbose {
		logger, _ = zap.NewDevelopment()
	} else {
		logger = zap.NewNop()
	}

	r := &Runner{
		cfg:     cfg,
		verbose: verbose,
		logger:  logger,
	}

	// Connect to SMB
	smbCfg := &smb.ClientConfig{
		Server:   cfg.RemoteHost,
		Share:    cfg.RemoteShare,
		Username: cfg.Username,
		Password: cfg.Password,
		Domain:   cfg.Domain,
	}

	client, err := smb.NewSMBClient(smbCfg, logger)
	if err != nil {
		return nil, fmt.Errorf("création client SMB échouée: %w", err)
	}

	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("connexion SMB échouée: %w", err)
	}
	r.smbClient = client

	// Ensure local test directory exists
	if err := os.MkdirAll(cfg.LocalTestPath(), 0755); err != nil {
		client.Disconnect()
		return nil, fmt.Errorf("création dossier local échouée: %w", err)
	}

	return r, nil
}

// Close releases resources.
func (r *Runner) Close() {
	if r.provider != nil {
		r.provider.Close()
	}
	if r.smbClient != nil {
		r.smbClient.Disconnect()
	}
}

// InitCloudFiles initializes Cloud Files API for the test directory.
func (r *Runner) InitCloudFiles() error {
	localPath := r.cfg.LocalTestPath()

	r.Log("Initialisation Cloud Files...")

	// Create CloudFilesProvider (handles everything: sync root, callbacks, hydration)
	providerCfg := cloudfiles.ProviderConfig{
		LocalPath:    localPath,
		RemotePath:   r.cfg.RemoteTestPath(),
		ProviderName: "AnemoneSync-Test",
		Logger:       r.logger,
		UseCGOBridge: true,
	}

	provider, err := cloudfiles.NewCloudFilesProvider(providerCfg)
	if err != nil {
		return fmt.Errorf("création provider échouée: %w", err)
	}
	r.provider = provider

	// Set up SMB data source for hydration
	wrapper := &smbClientWrapper{client: r.smbClient}
	adapter := cloudfiles.NewSMBClientAdapter(wrapper, r.cfg.RemoteTestPath(), r.logger)
	provider.SetDataSource(adapter)

	// Initialize (register sync root + connect)
	ctx := context.Background()
	if err := provider.Initialize(ctx); err != nil {
		return fmt.Errorf("initialisation provider échouée: %w", err)
	}
	r.Log("  Provider initialisé")

	r.Log("  Cloud Files initialisé")
	return nil
}

// RunTests runs the specified tests.
func (r *Runner) RunTests(ctx context.Context, testIDs []string) []TestResult {
	var results []TestResult

	// Clean up test directory first (remove stale files from previous runs)
	r.Log("Nettoyage préalable du répertoire de test...")
	if err := r.CleanupTestDir(); err != nil {
		r.Logf("  Avertissement nettoyage: %v", err)
	}

	// Initialize Cloud Files
	if err := r.InitCloudFiles(); err != nil {
		// Return error for all tests
		for _, id := range testIDs {
			s := GetScenarioByID(id)
			results = append(results, TestResult{
				ID:     id,
				Name:   s.Name,
				Passed: false,
				Error:  fmt.Sprintf("Init Cloud Files: %v", err),
			})
		}
		return results
	}

	for _, id := range testIDs {
		if ctx.Err() != nil {
			break
		}

		s := GetScenarioByID(id)
		if s == nil {
			continue
		}

		result := r.runSingleTest(ctx, s)
		results = append(results, result)
	}

	return results
}

// runSingleTest runs a single test scenario.
func (r *Runner) runSingleTest(ctx context.Context, s *Scenario) TestResult {
	fmt.Printf("\n─────────────────────────────────────────────────────────────\n")
	fmt.Printf("  %s: %s\n", s.ID, s.Name)
	fmt.Printf("─────────────────────────────────────────────────────────────\n")

	start := time.Now()
	err := s.Run(ctx, r)
	duration := time.Since(start)

	result := TestResult{
		ID:       s.ID,
		Name:     s.Name,
		Duration: duration.Milliseconds(),
		Passed:   err == nil,
	}

	if err != nil {
		result.Error = err.Error()
		fmt.Printf("\n  ✗ ÉCHEC: %v\n", err)
	} else {
		fmt.Printf("\n  ✓ SUCCÈS (%.2fs)\n", duration.Seconds())
	}

	return result
}

// Cleanup removes the test directory.
func (r *Runner) Cleanup() error {
	// Remove local test directory
	if err := os.RemoveAll(r.cfg.LocalTestPath()); err != nil {
		return fmt.Errorf("suppression locale échouée: %w", err)
	}

	// Remove remote test files (one by one)
	remoteFiles, err := r.listRemoteFilesRecursive(context.Background(), r.cfg.RemoteTestPath())
	if err == nil {
		// Delete files first, then directories (reverse order)
		for i := len(remoteFiles) - 1; i >= 0; i-- {
			rf := remoteFiles[i]
			fullPath := filepath.Join(r.cfg.RemoteTestPath(), rf.RelPath)
			_ = r.smbClient.Delete(fullPath)
		}
	}

	return nil
}

// CleanupTestDir cleans up the test directory before running tests.
// Removes all local and remote files to start fresh.
func (r *Runner) CleanupTestDir() error {
	localPath := r.cfg.LocalTestPath()

	// Remove all local files (but keep the directory)
	entries, err := os.ReadDir(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist, nothing to clean
		}
		return err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(localPath, entry.Name())
		if err := os.RemoveAll(fullPath); err != nil {
			r.Logf("  Impossible de supprimer %s: %v", entry.Name(), err)
		}
	}

	// Remove all remote test files
	remoteFiles, err := r.listRemoteFilesRecursive(context.Background(), r.cfg.RemoteTestPath())
	if err != nil {
		return nil // Remote dir might not exist
	}

	// Delete files first, then directories (reverse order)
	for i := len(remoteFiles) - 1; i >= 0; i-- {
		rf := remoteFiles[i]
		fullPath := filepath.Join(r.cfg.RemoteTestPath(), rf.RelPath)
		_ = r.smbClient.Delete(fullPath)
	}

	return nil
}

// Log prints a message if verbose.
func (r *Runner) Log(msg string) {
	if r.verbose {
		fmt.Printf("    %s\n", msg)
	}
}

// Logf prints a formatted message if verbose.
func (r *Runner) Logf(format string, args ...interface{}) {
	if r.verbose {
		fmt.Printf("    "+format+"\n", args...)
	}
}

// Errorf returns a formatted error.
func (r *Runner) Errorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// ===== File Operations =====

// CreateRemoteFile creates a file on the remote server using Upload.
func (r *Runner) CreateRemoteFile(relPath string, content []byte) error {
	// Write to temp file first
	tmpFile, err := os.CreateTemp("", "cftest_*.tmp")
	if err != nil {
		return fmt.Errorf("création fichier temp échouée: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(content); err != nil {
		tmpFile.Close()
		return fmt.Errorf("écriture fichier temp échouée: %w", err)
	}
	tmpFile.Close()

	// Upload to remote (Upload creates parent directories automatically)
	fullPath := filepath.Join(r.cfg.RemoteTestPath(), relPath)
	return r.smbClient.Upload(tmpPath, fullPath)
}

// ReadRemoteFile reads a file from the remote server.
func (r *Runner) ReadRemoteFile(relPath string) ([]byte, error) {
	fullPath := filepath.Join(r.cfg.RemoteTestPath(), relPath)
	return r.smbClient.ReadFile(fullPath)
}

// DeleteRemoteFile deletes a file on the remote server.
func (r *Runner) DeleteRemoteFile(relPath string) error {
	fullPath := filepath.Join(r.cfg.RemoteTestPath(), relPath)
	return r.smbClient.Delete(fullPath)
}

// CopyFileToRemote copies a local file to the remote server.
func (r *Runner) CopyFileToRemote(srcPath, dstRelPath string) error {
	fullPath := filepath.Join(r.cfg.RemoteTestPath(), dstRelPath)
	return r.smbClient.Upload(srcPath, fullPath)
}

// CreateLocalFile creates a file in the local sync root.
func (r *Runner) CreateLocalFile(relPath string, content []byte) error {
	fullPath := filepath.Join(r.cfg.LocalTestPath(), relPath)

	// Ensure parent directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(fullPath, content, 0644)
}

// ReadLocalFile reads a file from the local sync root.
func (r *Runner) ReadLocalFile(relPath string) ([]byte, error) {
	fullPath := filepath.Join(r.cfg.LocalTestPath(), relPath)
	return os.ReadFile(fullPath)
}

// WriteLocalFile writes to an existing file in the local sync root.
func (r *Runner) WriteLocalFile(relPath string, content []byte) error {
	fullPath := filepath.Join(r.cfg.LocalTestPath(), relPath)
	return os.WriteFile(fullPath, content, 0644)
}

// ===== Verification Functions =====

// VerifyPlaceholderExists verifies a placeholder file exists.
func (r *Runner) VerifyPlaceholderExists(relPath string) error {
	fullPath := filepath.Join(r.cfg.LocalTestPath(), relPath)

	_, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("fichier n'existe pas: %s", relPath)
	}

	// Check it's a placeholder
	state, err := r.getPlaceholderState(fullPath)
	if err != nil {
		return err
	}

	if state&cloudfiles.CF_PLACEHOLDER_STATE_PLACEHOLDER == 0 {
		return fmt.Errorf("fichier n'est pas un placeholder: state=0x%X", state)
	}

	return nil
}

// VerifyFileHydrated verifies a file is hydrated (not PARTIAL).
func (r *Runner) VerifyFileHydrated(relPath string) error {
	fullPath := filepath.Join(r.cfg.LocalTestPath(), relPath)

	state, err := r.getPlaceholderState(fullPath)
	if err != nil {
		return err
	}

	isPlaceholder := state&cloudfiles.CF_PLACEHOLDER_STATE_PLACEHOLDER != 0
	isPartial := state&cloudfiles.CF_PLACEHOLDER_STATE_PARTIAL != 0

	if !isPlaceholder {
		// Not a placeholder = fully local = hydrated
		return nil
	}

	if isPartial {
		return fmt.Errorf("fichier est déshydraté (PARTIAL): state=0x%X", state)
	}

	return nil
}

// VerifyFileDehydrated verifies a file is dehydrated (PARTIAL).
// Includes retry logic to handle race conditions.
func (r *Runner) VerifyFileDehydrated(relPath string) error {
	fullPath := filepath.Join(r.cfg.LocalTestPath(), relPath)

	// Retry up to 5 times with 200ms delay to handle race conditions
	var lastErr error
	for i := 0; i < 5; i++ {
		if i > 0 {
			time.Sleep(200 * time.Millisecond)
		}

		state, err := r.getPlaceholderState(fullPath)
		if err != nil {
			lastErr = err
			continue
		}

		isPlaceholder := state&cloudfiles.CF_PLACEHOLDER_STATE_PLACEHOLDER != 0
		isPartial := state&cloudfiles.CF_PLACEHOLDER_STATE_PARTIAL != 0

		if !isPlaceholder {
			lastErr = fmt.Errorf("fichier n'est pas un placeholder: state=0x%X", state)
			continue
		}

		if !isPartial {
			lastErr = fmt.Errorf("fichier n'est pas déshydraté (pas PARTIAL): state=0x%X", state)
			continue
		}

		return nil // Success
	}

	return lastErr
}

// VerifyFileDeleted verifies a file no longer exists locally.
func (r *Runner) VerifyFileDeleted(relPath string) error {
	fullPath := filepath.Join(r.cfg.LocalTestPath(), relPath)

	_, err := os.Stat(fullPath)
	if err == nil {
		return fmt.Errorf("fichier existe encore: %s", relPath)
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("erreur vérification: %w", err)
	}

	return nil
}

// getPlaceholderState returns the Cloud Files placeholder state.
func (r *Runner) getPlaceholderState(fullPath string) (cloudfiles.CF_PLACEHOLDER_STATE, error) {
	handle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(fullPath),
		0, // Query only
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return 0, fmt.Errorf("impossible d'ouvrir fichier: %w", err)
	}
	defer windows.CloseHandle(handle)

	var fileInfo windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &fileInfo); err != nil {
		return 0, fmt.Errorf("impossible de lire attributs: %w", err)
	}

	return cloudfiles.GetPlaceholderState(fileInfo.FileAttributes, cloudfiles.IO_REPARSE_TAG_CLOUD), nil
}

// ===== Sync Operations =====

// RunSync runs a bidirectional sync operation.
// 1. Upload local files to remote
// 2. Create/update placeholders from remote files
// 3. Delete local files not on remote
func (r *Runner) RunSync(ctx context.Context) error {
	r.Log("  Exécution sync...")

	// Step 1: List remote files
	remoteFiles, err := r.listRemoteFilesRecursive(ctx, r.cfg.RemoteTestPath())
	if err != nil {
		return fmt.Errorf("lecture dossier distant échouée: %w", err)
	}
	remoteMap := make(map[string]remoteFileEntry)
	for _, rf := range remoteFiles {
		remoteMap[rf.RelPath] = rf
	}

	// Step 2: List local files
	localFiles, err := r.listLocalFilesRecursive(ctx, r.cfg.LocalTestPath())
	if err != nil {
		return fmt.Errorf("lecture dossier local échouée: %w", err)
	}
	localMap := make(map[string]localFileEntry)
	for _, lf := range localFiles {
		localMap[lf.RelPath] = lf
	}

	// Step 3: Upload local files that don't exist on remote or are modified
	// Note: Skip DEHYDRATED placeholders (PARTIAL) - they have no local content
	// But hydrated placeholders (placeholder without PARTIAL) can be uploaded
	for relPath, lf := range localMap {
		if lf.IsDir || lf.IsDehydrated {
			continue // Skip directories and dehydrated (PARTIAL) placeholders
		}
		rf, existsOnRemote := remoteMap[relPath]
		needsUpload := false

		if !existsOnRemote {
			r.Logf("  Upload (nouveau): %s", relPath)
			needsUpload = true
		} else if lf.ModTime.After(rf.ModTime) {
			// Local is newer - upload
			r.Logf("  Upload (local plus récent): %s", relPath)
			needsUpload = true
		} else if lf.Size != rf.Size && !rf.ModTime.After(lf.ModTime) {
			// Size differs AND remote is NOT newer - upload local changes
			r.Logf("  Upload (taille différente): %s", relPath)
			needsUpload = true
		}
		// Note: if remote is newer or size differs with remote newer, Step 5 handles download

		if needsUpload {
			localPath := filepath.Join(r.cfg.LocalTestPath(), relPath)
			remotePath := filepath.Join(r.cfg.RemoteTestPath(), relPath)
			if err := r.smbClient.Upload(localPath, remotePath); err != nil {
				return fmt.Errorf("upload échoué pour %s: %w", relPath, err)
			}
		}
	}

	// Step 4: Refresh remote list after uploads
	remoteFiles, err = r.listRemoteFilesRecursive(ctx, r.cfg.RemoteTestPath())
	if err != nil {
		return fmt.Errorf("lecture dossier distant échouée: %w", err)
	}
	remoteMap = make(map[string]remoteFileEntry)
	for _, rf := range remoteFiles {
		remoteMap[rf.RelPath] = rf
	}

	// Step 5: Handle remote modifications on hydrated files
	// If a local file is hydrated (real file or hydrated placeholder) but remote is different and newer, update it
	for relPath, rf := range remoteMap {
		if rf.IsDir {
			continue
		}
		lf, existsLocally := localMap[relPath]
		// Check if local file has data (either real file or hydrated placeholder)
		localHasData := existsLocally && !lf.IsDir && !lf.IsDehydrated
		if localHasData {
			// Check if remote is different AND remote is newer
			sizeChanged := rf.Size != lf.Size
			remoteIsNewer := rf.ModTime.After(lf.ModTime)

			if sizeChanged && remoteIsNewer {
				r.Logf("  Mise à jour (distant modifié): %s (size: %d->%d, remote newer)",
					relPath, lf.Size, rf.Size)
				// Download new content from remote
				localPath := filepath.Join(r.cfg.LocalTestPath(), relPath)
				content, err := r.ReadRemoteFile(relPath)
				if err != nil {
					return fmt.Errorf("lecture distante échouée pour %s: %w", relPath, err)
				}
				if err := os.WriteFile(localPath, content, 0644); err != nil {
					return fmt.Errorf("écriture locale échouée pour %s: %w", relPath, err)
				}
			}
		}
	}

	// Step 6: Create/update placeholders from remote
	var placeholderInfos []cloudfiles.RemoteFileInfo
	for _, rf := range remoteFiles {
		placeholderInfos = append(placeholderInfos, cloudfiles.RemoteFileInfo{
			Path:        rf.RelPath,
			Size:        rf.Size,
			ModTime:     rf.ModTime,
			IsDirectory: rf.IsDir,
		})
	}
	if err := r.provider.SyncPlaceholders(ctx, placeholderInfos); err != nil {
		return fmt.Errorf("création placeholders échouée: %w", err)
	}

	// Step 7: Delete local files not on remote (only dehydrated placeholders or dirs)
	// Don't delete hydrated files - user might have local modifications
	for relPath, lf := range localMap {
		if _, existsOnRemote := remoteMap[relPath]; !existsOnRemote {
			localPath := filepath.Join(r.cfg.LocalTestPath(), relPath)
			if lf.IsDehydrated || lf.IsDir {
				r.Logf("  Suppression locale: %s", relPath)
				os.Remove(localPath)
			}
		}
	}

	r.Logf("  Sync terminé: %d fichiers distants", len(remoteFiles))
	return nil
}

// localFileEntry represents a local file.
type localFileEntry struct {
	RelPath       string
	Size          int64
	ModTime       time.Time
	IsDir         bool
	IsPlaceholder bool
	IsDehydrated  bool // PARTIAL flag = no local content
}

// listLocalFilesRecursive lists files recursively from the local sync root.
func (r *Runner) listLocalFilesRecursive(ctx context.Context, basePath string) ([]localFileEntry, error) {
	var result []localFileEntry

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if path == basePath {
			return nil // Skip root
		}

		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return nil
		}

		entry := localFileEntry{
			RelPath: relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
		}

		// Check if it's a placeholder and/or dehydrated
		if !info.IsDir() {
			state, err := r.getPlaceholderState(path)
			if err == nil {
				entry.IsPlaceholder = state&cloudfiles.CF_PLACEHOLDER_STATE_PLACEHOLDER != 0
				entry.IsDehydrated = entry.IsPlaceholder && (state&cloudfiles.CF_PLACEHOLDER_STATE_PARTIAL != 0)
			}
		}

		result = append(result, entry)
		return nil
	})

	return result, err
}

// remoteFileEntry represents a remote file.
type remoteFileEntry struct {
	RelPath string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

// listRemoteFilesRecursive lists files recursively from the remote.
func (r *Runner) listRemoteFilesRecursive(ctx context.Context, basePath string) ([]remoteFileEntry, error) {
	var result []remoteFileEntry

	err := r.walkRemote(ctx, basePath, "", &result)
	return result, err
}

func (r *Runner) walkRemote(ctx context.Context, basePath, relPath string, result *[]remoteFileEntry) error {
	fullPath := basePath
	if relPath != "" {
		fullPath = filepath.Join(basePath, relPath)
	}

	entries, err := r.smbClient.ListRemote(fullPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		entryRelPath := entry.Name
		if relPath != "" {
			entryRelPath = filepath.Join(relPath, entry.Name)
		}

		if entry.IsDir {
			// Create directory entry
			*result = append(*result, remoteFileEntry{
				RelPath: entryRelPath,
				Size:    0,
				ModTime: entry.ModTime,
				IsDir:   true,
			})

			// Recurse
			if err := r.walkRemote(ctx, basePath, entryRelPath, result); err != nil {
				return err
			}
		} else {
			*result = append(*result, remoteFileEntry{
				RelPath: entryRelPath,
				Size:    entry.Size,
				ModTime: entry.ModTime,
				IsDir:   false,
			})
		}
	}

	return nil
}

// ===== Dehydration =====

// DehydrateFile dehydrates a file.
func (r *Runner) DehydrateFile(relPath string) error {
	return r.provider.DehydrateFile(context.Background(), relPath)
}

// ===== Large Files =====

// FindLargeSourceFile finds a large file in the source directory.
func (r *Runner) FindLargeSourceFile(minSize int64) (string, int64, error) {
	var foundPath string
	var foundSize int64

	err := filepath.Walk(r.cfg.SourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil
		}
		if info.Size() >= minSize && info.Size() > foundSize {
			// Prefer smaller files that still meet minimum
			if foundPath == "" || info.Size() < foundSize*2 {
				foundPath = path
				foundSize = info.Size()
			}
		}
		return nil
	})

	if err != nil {
		return "", 0, err
	}
	if foundPath == "" {
		return "", 0, fmt.Errorf("aucun fichier >= %d bytes trouvé", minSize)
	}

	return foundPath, foundSize, nil
}

// HydrateFileWithProgress hydrates a file and shows progress.
func (r *Runner) HydrateFileWithProgress(ctx context.Context, relPath string) error {
	fullPath := filepath.Join(r.cfg.LocalTestPath(), relPath)

	// Open file (this triggers hydration)
	f, err := os.Open(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Read in chunks to show progress
	buf := make([]byte, 1024*1024) // 1MB chunks
	var totalRead int64

	fi, _ := f.Stat()
	totalSize := fi.Size()

	for {
		n, err := f.Read(buf)
		if n > 0 {
			totalRead += int64(n)
			if r.verbose {
				pct := float64(totalRead) / float64(totalSize) * 100
				fmt.Printf("\r    Progress: %.1f%% (%d / %d bytes)", pct, totalRead, totalSize)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	if r.verbose {
		fmt.Println()
	}

	return nil
}
