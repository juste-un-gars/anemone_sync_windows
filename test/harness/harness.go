package harness

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"
	"github.com/juste-un-gars/anemone_sync_windows/internal/config"
	"github.com/juste-un-gars/anemone_sync_windows/internal/database"
	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	syncpkg "github.com/juste-un-gars/anemone_sync_windows/internal/sync"
	"go.uber.org/zap"
)

// Harness orchestrates the test execution.
type Harness struct {
	Config   *Config
	Reporter *Reporter
	Verbose  bool

	// Direct SMB connection for file operations (setup/cleanup)
	smbConn    net.Conn
	smbSession *smb2.Session
	smbShare   *smb2.Share

	// Real sync engine components
	db     *database.DB
	engine *syncpkg.Engine
	logger *zap.Logger
	jobs   map[string]int64 // Job name -> DB ID
}

// New creates a new test harness.
func New(cfg *Config, verbose bool) (*Harness, error) {
	// Create results directory
	resultsDir := ResultsDir(cfg.LocalBase)
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create results directory: %w", err)
	}

	// Create logger
	var logger *zap.Logger
	var err error
	if verbose {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		logger = zap.NewNop()
	}

	return &Harness{
		Config:   cfg,
		Reporter: NewReporter(cfg.LocalBase, verbose),
		Verbose:  verbose,
		logger:   logger,
		jobs:     make(map[string]int64),
	}, nil
}

// Close releases resources.
func (h *Harness) Close() {
	if h.smbShare != nil {
		h.smbShare.Umount()
	}
	if h.smbSession != nil {
		h.smbSession.Logoff()
	}
	if h.smbConn != nil {
		h.smbConn.Close()
	}
	if h.engine != nil {
		h.engine.Close()
	}
	if h.db != nil {
		h.db.Close()
	}
	if h.logger != nil {
		h.logger.Sync()
	}
}

// TestConnection tests the SMB connection or mapped drive.
func (h *Harness) TestConnection() error {
	fmt.Print("Test de connexion... ")

	// If using mapped drive, just check if the path exists
	if h.Config.UseMappedDrive && h.Config.RemoteBase != "" {
		if err := os.MkdirAll(h.Config.RemoteBase, 0755); err != nil {
			fmt.Println("✗")
			return fmt.Errorf("lecteur mappé inaccessible %s: %w", h.Config.RemoteBase, err)
		}
		fmt.Printf("✓ Connecté (lecteur mappé: %s)\n", h.Config.RemoteBase)
		return nil
	}

	// Otherwise use SMB
	if err := h.connectSMB(); err != nil {
		fmt.Println("✗")
		return fmt.Errorf("connexion échouée: %w", err)
	}

	fmt.Println("✓ Connecté")
	return nil
}

// InitEngine initializes the real sync engine.
func (h *Harness) InitEngine() error {
	fmt.Print("Initialisation du moteur de sync... ")

	// Create/open test database
	dbPath := DBPath(h.Config.LocalBase)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		fmt.Println("✗")
		return fmt.Errorf("cannot create db directory: %w", err)
	}

	// Remove old test database to start fresh
	os.Remove(dbPath)

	dbCfg := database.Config{
		Path:             dbPath,
		EncryptionKey:    "test_harness_key_2026",
		CreateIfNotExist: true,
	}
	db, err := database.Open(dbCfg)
	if err != nil {
		fmt.Println("✗")
		return fmt.Errorf("cannot open database: %w", err)
	}
	h.db = db

	// Store credentials in keyring
	credMgr := smb.NewCredentialManager(h.logger.Named("cred"))
	cred := &smb.Credentials{
		Server:   h.Config.RemoteHost,
		Username: h.Config.Username,
		Password: h.Config.Password,
		Domain:   h.Config.Domain,
	}
	if err := credMgr.Save(cred); err != nil {
		fmt.Println("✗")
		return fmt.Errorf("cannot save credentials: %w", err)
	}

	// Create test jobs in database
	for _, jobCfg := range h.Config.Jobs() {
		localPath := h.Config.LocalPath(jobCfg.Name)
		remotePath := h.Config.UNCPath(jobCfg.Name)

		// Ensure local directory exists
		if err := os.MkdirAll(localPath, 0755); err != nil {
			fmt.Println("✗")
			return fmt.Errorf("cannot create local directory %s: %w", localPath, err)
		}

		// Create job in database
		job := &database.SyncJob{
			Name:               jobCfg.Name,
			LocalPath:          localPath,
			RemotePath:         remotePath,
			SyncMode:           jobCfg.Mode,
			ConflictResolution: "recent",
			TriggerMode:        "manual",
			Enabled:            true,
		}

		if err := h.db.CreateSyncJob(job); err != nil {
			fmt.Println("✗")
			return fmt.Errorf("cannot create job %s: %w", jobCfg.Name, err)
		}
		h.jobs[jobCfg.Name] = job.ID
	}

	// Create sync engine config
	cfg := &config.Config{
		Sync: config.SyncConfig{
			DefaultMode:               "mirror",
			DefaultConflictResolution: "recent",
			Performance: config.PerformanceConfig{
				ParallelTransfers: 4,
				BufferSizeMB:      8,
				HashAlgorithm:     "sha256",
			},
		},
	}

	// Create sync engine
	engine, err := syncpkg.NewEngine(cfg, db, h.logger.Named("engine"))
	if err != nil {
		fmt.Println("✗")
		return fmt.Errorf("cannot create engine: %w", err)
	}
	h.engine = engine

	fmt.Println("✓ Prêt")
	return nil
}

// connectSMB establishes SMB connection for file operations.
func (h *Harness) connectSMB() error {
	// Close existing connection if any
	if h.smbShare != nil {
		h.smbShare.Umount()
		h.smbShare = nil
	}
	if h.smbSession != nil {
		h.smbSession.Logoff()
		h.smbSession = nil
	}
	if h.smbConn != nil {
		h.smbConn.Close()
		h.smbConn = nil
	}

	// Connect
	conn, err := net.DialTimeout("tcp", h.Config.RemoteHost+":445", 10*time.Second)
	if err != nil {
		return fmt.Errorf("cannot connect to %s:445: %w", h.Config.RemoteHost, err)
	}
	h.smbConn = conn

	// Create session
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     h.Config.Username,
			Password: h.Config.Password,
			Domain:   h.Config.Domain,
		},
	}

	session, err := d.DialContext(context.Background(), conn)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SMB session failed: %w", err)
	}
	h.smbSession = session

	// Mount share
	share, err := session.Mount(h.Config.RemoteShare)
	if err != nil {
		session.Logoff()
		conn.Close()
		return fmt.Errorf("cannot mount share %s: %w", h.Config.RemoteShare, err)
	}
	h.smbShare = share

	return nil
}

// Run executes all tests.
func (h *Harness) Run(ctx context.Context, jobFilter, scenarioFilter string) error {
	h.Reporter.Start()

	// Get scenarios
	scenarios := GetAllScenarios()

	// Filter by job if specified
	if jobFilter != "" {
		filtered := make([]Scenario, 0)
		for _, s := range scenarios {
			if s.Job == jobFilter {
				filtered = append(filtered, s)
			}
		}
		scenarios = filtered
	}

	// Filter by scenario ID if specified
	if scenarioFilter != "" {
		filtered := make([]Scenario, 0)
		for _, s := range scenarios {
			if s.ID == scenarioFilter {
				filtered = append(filtered, s)
			}
		}
		scenarios = filtered
	}

	if len(scenarios) == 0 {
		return fmt.Errorf("aucun scénario trouvé")
	}

	// Run scenarios
	for _, scenario := range scenarios {
		if ctx.Err() != nil {
			break
		}

		result := h.runScenario(ctx, scenario)
		h.Reporter.AddResult(result)
	}

	// Generate report
	return h.Reporter.Finish()
}

// runScenario executes a single test scenario.
func (h *Harness) runScenario(ctx context.Context, scenario Scenario) *TestResult {
	result := &TestResult{
		ID:          scenario.ID,
		Name:        scenario.Name,
		Job:         scenario.Job,
		Description: scenario.Description,
		StartTime:   time.Now(),
	}

	h.Reporter.LogScenarioStart(scenario)

	// 1. Setup - Clean directories (unless SkipSync is set for continuation tests)
	if !scenario.SkipSync {
		if err := h.cleanJobDirectories(scenario.Job); err != nil {
			result.Error = fmt.Sprintf("setup failed: %v", err)
			result.Passed = false
			result.Duration = time.Since(result.StartTime)
			h.Reporter.LogScenarioEnd(result)
			return result
		}
	}

	// 2. Run setup actions (create initial files)
	if err := h.executeActions(ctx, scenario.Job, scenario.Setup); err != nil {
		result.Error = fmt.Sprintf("setup actions failed: %v", err)
		result.Passed = false
		result.Duration = time.Since(result.StartTime)
		h.Reporter.LogScenarioEnd(result)
		return result
	}

	// 3. If there are setup actions and not skipping sync, run initial sync to establish baseline
	if len(scenario.Setup) > 0 && !scenario.SkipSync {
		if err := h.runSync(ctx, scenario.Job, scenario.Mode); err != nil {
			// For ExpectError scenarios during setup, this is unexpected
			result.Error = fmt.Sprintf("initial sync failed: %v", err)
			result.Passed = false
			result.Duration = time.Since(result.StartTime)
			h.Reporter.LogScenarioEnd(result)
			return result
		}
		// Small delay after initial sync
		time.Sleep(200 * time.Millisecond)
	}

	// 4. Execute test actions (may include wait_user for interactive tests)
	result.Actions = scenario.Actions
	if err := h.executeActions(ctx, scenario.Job, scenario.Actions); err != nil {
		result.Error = fmt.Sprintf("actions failed: %v", err)
		result.Passed = false
		result.Duration = time.Since(result.StartTime)
		h.Reporter.LogScenarioEnd(result)
		return result
	}

	// 5. Small delay to let filesystem settle
	time.Sleep(200 * time.Millisecond)

	// 6. Run sync
	syncErr := h.runSync(ctx, scenario.Job, scenario.Mode)

	// Handle expected errors for resilience tests
	if scenario.ExpectError {
		if syncErr != nil {
			fmt.Printf("  [EXPECTED] Sync error: %v\n", syncErr)
			// This is expected, continue to validation
		} else {
			fmt.Println("  [WARNING] Expected sync to fail but it succeeded")
		}
	} else {
		if syncErr != nil {
			result.Error = fmt.Sprintf("sync failed: %v", syncErr)
			result.Passed = false
			result.Duration = time.Since(result.StartTime)
			h.Reporter.LogScenarioEnd(result)
			return result
		}
	}

	// 7. Validate results
	validations, err := h.validate(scenario)
	result.Validations = validations
	if err != nil {
		result.Error = err.Error()
		result.Passed = false
	} else {
		result.Passed = true
	}

	result.Duration = time.Since(result.StartTime)
	h.Reporter.LogScenarioEnd(result)
	return result
}

// cleanJobDirectories cleans both local and remote directories for a job.
func (h *Harness) cleanJobDirectories(job string) error {
	// Clean local
	localPath := h.Config.LocalPath(job)
	if err := h.cleanDirectory(localPath); err != nil {
		return fmt.Errorf("clean local: %w", err)
	}

	// Clean remote
	remotePath := h.Config.RemotePathForJob(job)
	if err := h.cleanRemoteDirectory(remotePath); err != nil {
		return fmt.Errorf("clean remote: %w", err)
	}

	// Clear cache for this job in database
	if h.db != nil {
		jobID, ok := h.jobs[job]
		if ok {
			// Clear the cache for this job by deleting files_state entries
			if err := h.db.ClearFilesState(jobID); err != nil {
				h.logger.Warn("failed to clear cache", zap.Error(err))
			}
		}
	}

	return nil
}

// cleanDirectory removes all files from a local directory.
func (h *Harness) cleanDirectory(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(path, 0755)
		}
		return err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		if err := os.RemoveAll(fullPath); err != nil {
			return err
		}
	}
	return nil
}

// cleanRemoteDirectory removes all files from a remote directory.
func (h *Harness) cleanRemoteDirectory(path string) error {
	// Use local filesystem operations for mapped drive
	if h.Config.UseMappedDrive {
		return h.cleanDirectory(path)
	}

	if h.smbShare == nil {
		if err := h.connectSMB(); err != nil {
			return err
		}
	}

	entries, err := h.smbShare.ReadDir(path)
	if err != nil {
		// Directory might not exist, try to create it
		return h.smbShare.MkdirAll(path, 0755)
	}

	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		fullPath = filepath.ToSlash(fullPath)
		if entry.IsDir() {
			if err := h.removeRemoteDir(fullPath); err != nil {
				return err
			}
		} else {
			if err := h.smbShare.Remove(fullPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// removeRemoteDir recursively removes a remote directory.
func (h *Harness) removeRemoteDir(path string) error {
	entries, err := h.smbShare.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fullPath := filepath.ToSlash(filepath.Join(path, entry.Name()))
		if entry.IsDir() {
			if err := h.removeRemoteDir(fullPath); err != nil {
				return err
			}
		} else {
			if err := h.smbShare.Remove(fullPath); err != nil {
				return err
			}
		}
	}

	return h.smbShare.Remove(path)
}

// runSync executes sync using the real sync engine.
func (h *Harness) runSync(ctx context.Context, job, mode string) error {
	// Get job ID
	jobID, ok := h.jobs[job]
	if !ok {
		return fmt.Errorf("job %s not found", job)
	}

	// Convert mode to sync mode
	var syncMode syncpkg.SyncMode
	switch mode {
	case "mirror":
		syncMode = syncpkg.SyncModeMirror
	case "upload", "local_to_remote":
		syncMode = syncpkg.SyncModeUpload
	case "download", "remote_to_local":
		syncMode = syncpkg.SyncModeDownload
	default:
		syncMode = syncpkg.SyncModeMirror
	}

	// Create sync request
	req := &syncpkg.SyncRequest{
		JobID:              jobID,
		LocalPath:          h.Config.LocalPath(job),
		RemotePath:         h.Config.UNCPath(job),
		Mode:               syncMode,
		ConflictResolution: "recent",
	}

	// Execute sync
	result, err := h.engine.Sync(ctx, req)
	if err != nil {
		return err
	}

	if result.Status == syncpkg.SyncStatusFailed {
		if len(result.Errors) > 0 {
			return fmt.Errorf("sync failed: %v", result.Errors[0].Error)
		}
		return fmt.Errorf("sync failed")
	}

	return nil
}

// FileInfo holds file metadata for comparison.
type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
	Hash    string
}

// listLocalFiles returns all files in a local directory with metadata.
func (h *Harness) listLocalFiles(basePath string) (map[string]*FileInfo, error) {
	files := make(map[string]*FileInfo)

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(basePath, path)
		relPath = filepath.ToSlash(relPath)

		hash, _ := h.hashLocalFile(path)

		files[relPath] = &FileInfo{
			Path:    relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Hash:    hash,
		}
		return nil
	})

	return files, err
}

// listRemoteFiles returns all files in a remote directory with metadata.
func (h *Harness) listRemoteFiles(basePath string) (map[string]*FileInfo, error) {
	// Use local filesystem operations for mapped drive
	if h.Config.UseMappedDrive {
		return h.listLocalFiles(basePath)
	}

	files := make(map[string]*FileInfo)

	var walkDir func(string) error
	walkDir = func(dir string) error {
		entries, err := h.smbShare.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		for _, entry := range entries {
			fullPath := filepath.ToSlash(filepath.Join(dir, entry.Name()))
			if entry.IsDir() {
				if err := walkDir(fullPath); err != nil {
					return err
				}
			} else {
				info, err := h.smbShare.Stat(fullPath)
				if err != nil {
					continue
				}

				relPath, _ := filepath.Rel(basePath, fullPath)
				relPath = filepath.ToSlash(relPath)

				hash, _ := h.hashRemoteFile(fullPath)

				files[relPath] = &FileInfo{
					Path:    relPath,
					Size:    info.Size(),
					ModTime: info.ModTime(),
					Hash:    hash,
				}
			}
		}
		return nil
	}

	err := walkDir(basePath)
	return files, err
}

// hashLocalFile computes SHA256 hash of a local file.
func (h *Harness) hashLocalFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// hashRemoteFile computes SHA256 hash of a remote file.
func (h *Harness) hashRemoteFile(path string) (string, error) {
	f, err := h.smbShare.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// executeActions executes a list of test actions.
func (h *Harness) executeActions(ctx context.Context, job string, actions []Action) error {
	writer := NewWriter(h.Config, h.smbShare, h.Config.UseMappedDrive)

	for _, action := range actions {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Handle special action types
		if action.Type == "wait_user" {
			h.waitForUser(action.Message)
			continue
		}

		if err := writer.Execute(job, action); err != nil {
			return fmt.Errorf("action %s failed: %w", action.Type, err)
		}

		// Small delay between actions
		if action.Delay > 0 {
			time.Sleep(action.Delay)
		}
	}

	return nil
}

// waitForUser waits for user input (for interactive tests).
func (h *Harness) waitForUser(message string) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  ACTION MANUELLE REQUISE                                     ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  %s\n", message)
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Print("Appuyez sur Entree pour continuer...")
	fmt.Scanln()
	fmt.Println()
}

// validate checks if the scenario expectations are met.
func (h *Harness) validate(scenario Scenario) ([]Validation, error) {
	validator := NewValidator(h.Config, h.smbShare, h.Config.UseMappedDrive)
	return validator.ValidateAll(scenario.Job, scenario.Expect)
}

// Helper to fix paths
func fixPath(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
