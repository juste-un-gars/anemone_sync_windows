package app

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cache"
	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	"github.com/juste-un-gars/anemone_sync_windows/internal/sync"
	"go.uber.org/zap"
)

// TrustSource defines which side is the source of truth for conflicts
type TrustSource string

const (
	TrustSourceAsk      TrustSource = "ask"       // Ask user on each conflict
	TrustSourceServer   TrustSource = "server"    // Server always wins
	TrustSourceLocal    TrustSource = "local"     // Local always wins
	TrustSourceRecent   TrustSource = "recent"    // Most recent wins
	TrustSourceKeepBoth TrustSource = "keep_both" // Keep both versions (rename server file)
)

// FirstSyncMode defines how to handle the initial sync
type FirstSyncMode string

const (
	FirstSyncModeMerge       FirstSyncMode = "merge"        // Keep all files from both sides
	FirstSyncModeServerWins  FirstSyncMode = "server_wins"  // Server is reference, delete local extras
	FirstSyncModeLocalWins   FirstSyncMode = "local_wins"   // Local is reference, delete server extras
	FirstSyncModeManual      FirstSyncMode = "manual"       // User chooses each file
)

// FileDifference represents a difference between local and remote
type FileDifference struct {
	Path         string
	LocalSize    int64
	RemoteSize   int64
	LocalMTime   time.Time
	RemoteMTime  time.Time
	LocalHash    string
	RemoteHash   string
	Type         DifferenceType
}

// DifferenceType indicates the type of difference
type DifferenceType string

const (
	DiffTypeLocalOnly    DifferenceType = "local_only"    // File exists only locally
	DiffTypeRemoteOnly   DifferenceType = "remote_only"   // File exists only on server
	DiffTypeContentDiff  DifferenceType = "content_diff"  // File exists on both but content differs
	DiffTypeSame         DifferenceType = "same"          // Files are identical
)

// FirstSyncAnalysis contains the result of analyzing differences
type FirstSyncAnalysis struct {
	LocalPath       string
	RemotePath      string
	LocalFileCount  int
	RemoteFileCount int
	LocalTotalSize  int64
	RemoteTotalSize int64

	LocalOnlyFiles    []FileDifference
	RemoteOnlyFiles   []FileDifference
	ConflictFiles     []FileDifference
	SameFiles         int

	AnalysisDuration  time.Duration
}

// FirstSyncAnalyzer analyzes differences between local and remote
type FirstSyncAnalyzer struct {
	app    *App
	logger *zap.Logger
}

// NewFirstSyncAnalyzer creates a new analyzer
func NewFirstSyncAnalyzer(app *App) *FirstSyncAnalyzer {
	return &FirstSyncAnalyzer{
		app:    app,
		logger: app.Logger().Named("first-sync"),
	}
}

// Analyze compares local and remote files and returns the differences
func (a *FirstSyncAnalyzer) Analyze(ctx context.Context, job *SyncJob) (*FirstSyncAnalysis, error) {
	start := time.Now()

	a.logger.Info("starting first sync analysis",
		zap.String("job", job.Name),
		zap.String("local", job.LocalPath),
		zap.String("remote", job.FullRemotePath()))

	// Connect to SMB using keyring credentials
	smbClient, err := smb.NewSMBClientFromKeyring(job.RemoteHost, job.RemoteShare, a.logger.Named("smb"))
	if err != nil {
		return nil, fmt.Errorf("failed to create SMB client: %w", err)
	}
	defer smbClient.Disconnect()

	// Actually connect to the SMB server
	if err := smbClient.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to SMB: %w", err)
	}

	// Scan local files
	localFiles, err := a.scanLocal(ctx, job.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to scan local: %w", err)
	}

	// Scan remote files (try manifest first, fallback to SMB)
	remoteFiles, err := a.scanRemote(ctx, smbClient, job.RemotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to scan remote: %w", err)
	}

	// Compare and build analysis
	analysis := &FirstSyncAnalysis{
		LocalPath:        job.LocalPath,
		RemotePath:       job.FullRemotePath(),
		LocalFileCount:   len(localFiles),
		RemoteFileCount:  len(remoteFiles),
		LocalOnlyFiles:   make([]FileDifference, 0),
		RemoteOnlyFiles:  make([]FileDifference, 0),
		ConflictFiles:    make([]FileDifference, 0),
	}

	// Track all paths
	allPaths := make(map[string]bool)
	for path := range localFiles {
		allPaths[path] = true
	}
	for path := range remoteFiles {
		allPaths[path] = true
	}

	// Compare each file
	for path := range allPaths {
		local := localFiles[path]
		remote := remoteFiles[path]

		if local != nil {
			analysis.LocalTotalSize += local.Size
		}
		if remote != nil {
			analysis.RemoteTotalSize += remote.Size
		}

		diff := a.compareFiles(path, local, remote)

		switch diff.Type {
		case DiffTypeLocalOnly:
			analysis.LocalOnlyFiles = append(analysis.LocalOnlyFiles, diff)
		case DiffTypeRemoteOnly:
			analysis.RemoteOnlyFiles = append(analysis.RemoteOnlyFiles, diff)
		case DiffTypeContentDiff:
			analysis.ConflictFiles = append(analysis.ConflictFiles, diff)
		case DiffTypeSame:
			analysis.SameFiles++
		}
	}

	analysis.AnalysisDuration = time.Since(start)

	a.logger.Info("first sync analysis completed",
		zap.Int("local_only", len(analysis.LocalOnlyFiles)),
		zap.Int("remote_only", len(analysis.RemoteOnlyFiles)),
		zap.Int("conflicts", len(analysis.ConflictFiles)),
		zap.Int("same", analysis.SameFiles),
		zap.Duration("duration", analysis.AnalysisDuration))

	return analysis, nil
}

// scanLocal scans local files
func (a *FirstSyncAnalyzer) scanLocal(ctx context.Context, basePath string) (map[string]*cache.FileInfo, error) {
	files := make(map[string]*cache.FileInfo)

	err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		info, err := os.Stat(path)
		if err != nil {
			return nil
		}

		files[relPath] = &cache.FileInfo{
			Path:  relPath,
			Size:  info.Size(),
			MTime: info.ModTime(),
			Hash:  "", // No hash for speed
		}

		return nil
	})

	return files, err
}

// scanRemote scans remote files via manifest or SMB
func (a *FirstSyncAnalyzer) scanRemote(ctx context.Context, smbClient *smb.SMBClient, remotePath string) (map[string]*cache.FileInfo, error) {
	// Try manifest first
	manifestReader := sync.NewManifestReader(smbClient, a.logger.Named("manifest"))
	result := manifestReader.ReadManifest(ctx, remotePath)
	if result.Manifest != nil && len(result.Manifest.Files) > 0 {
		a.logger.Info("using manifest for remote scan", zap.Int("files", len(result.Manifest.Files)))
		// Use the helper method that handles Unix timestamp conversion
		return result.Manifest.ToFileInfoMap(), nil
	}

	// Fallback to SMB scan
	a.logger.Info("manifest not available, scanning via SMB")
	scanner := sync.NewRemoteScanner(smbClient, a.logger.Named("remote-scanner"), nil)
	scanResult, err := scanner.Scan(ctx, remotePath)
	if err != nil {
		return nil, err
	}

	return scanResult.Files, nil
}

// compareFiles compares two files and returns the difference type
func (a *FirstSyncAnalyzer) compareFiles(path string, local, remote *cache.FileInfo) FileDifference {
	diff := FileDifference{Path: path}

	if local != nil {
		diff.LocalSize = local.Size
		diff.LocalMTime = local.MTime
		diff.LocalHash = local.Hash
	}
	if remote != nil {
		diff.RemoteSize = remote.Size
		diff.RemoteMTime = remote.MTime
		diff.RemoteHash = remote.Hash
	}

	// Determine difference type
	if local == nil && remote != nil {
		diff.Type = DiffTypeRemoteOnly
	} else if local != nil && remote == nil {
		diff.Type = DiffTypeLocalOnly
	} else if local != nil && remote != nil {
		// Both exist - check if same
		if local.Size == remote.Size {
			if local.Hash != "" && remote.Hash != "" {
				if local.Hash == remote.Hash {
					diff.Type = DiffTypeSame
				} else {
					diff.Type = DiffTypeContentDiff
				}
			} else {
				// No hash, assume same if size matches
				diff.Type = DiffTypeSame
			}
		} else {
			diff.Type = DiffTypeContentDiff
		}
	}

	return diff
}

// HasDifferences returns true if there are any differences
func (a *FirstSyncAnalysis) HasDifferences() bool {
	return len(a.LocalOnlyFiles) > 0 || len(a.RemoteOnlyFiles) > 0 || len(a.ConflictFiles) > 0
}

// TotalDifferences returns the total number of differences
func (a *FirstSyncAnalysis) TotalDifferences() int {
	return len(a.LocalOnlyFiles) + len(a.RemoteOnlyFiles) + len(a.ConflictFiles)
}
