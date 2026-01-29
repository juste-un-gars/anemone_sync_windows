// CLI interface for AnemoneSync.
// Allows running sync operations from the command line without GUI.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/app"
	"github.com/juste-un-gars/anemone_sync_windows/internal/cloudfiles"
	"github.com/juste-un-gars/anemone_sync_windows/internal/config"
	"github.com/juste-un-gars/anemone_sync_windows/internal/database"
	"github.com/juste-un-gars/anemone_sync_windows/internal/sync"
	"go.uber.org/zap"
)

// CLIOptions represents parsed command-line options.
type CLIOptions struct {
	ListJobs       bool
	SyncJobID      int64 // 0 = not set
	SyncAll        bool
	DehydrateJobID int64 // 0 = not set
	DehydrateDays  int   // -1 = not set (use job default), 0 = all files
	Help           bool
}

// parseCLIArgs parses command-line arguments.
// Returns nil if no CLI arguments are present (GUI mode).
func parseCLIArgs(args []string) *CLIOptions {
	opts := &CLIOptions{
		DehydrateDays: -1, // -1 means use job default
	}
	hasCliArg := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch arg {
		case "-h", "--help":
			opts.Help = true
			hasCliArg = true

		case "-l", "--list-jobs":
			opts.ListJobs = true
			hasCliArg = true

		case "-a", "--sync-all":
			opts.SyncAll = true
			hasCliArg = true

		case "-s", "--sync":
			hasCliArg = true
			// Get next argument as job ID
			if i+1 < len(args) {
				i++
				id, err := strconv.ParseInt(args[i], 10, 64)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: invalid job ID '%s'\n", args[i])
					os.Exit(1)
				}
				opts.SyncJobID = id
			} else {
				fmt.Fprintf(os.Stderr, "Error: --sync requires a job ID\n")
				os.Exit(1)
			}

		case "-d", "--dehydrate":
			hasCliArg = true
			// Get next argument as job ID
			if i+1 < len(args) {
				i++
				id, err := strconv.ParseInt(args[i], 10, 64)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: invalid job ID '%s'\n", args[i])
					os.Exit(1)
				}
				opts.DehydrateJobID = id
			} else {
				fmt.Fprintf(os.Stderr, "Error: --dehydrate requires a job ID\n")
				os.Exit(1)
			}

		case "--days":
			// Get next argument as days count
			if i+1 < len(args) {
				i++
				days, err := strconv.Atoi(args[i])
				if err != nil || days < 0 {
					fmt.Fprintf(os.Stderr, "Error: invalid days value '%s' (must be >= 0)\n", args[i])
					os.Exit(1)
				}
				opts.DehydrateDays = days
			} else {
				fmt.Fprintf(os.Stderr, "Error: --days requires a number\n")
				os.Exit(1)
			}

		case "--autostart":
			// Ignore autostart flag, it's handled separately for GUI mode
			continue

		default:
			// Unknown flag - could be GUI mode or error
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Error: unknown option '%s'\n", arg)
				fmt.Fprintf(os.Stderr, "Run 'anemonesync --help' for usage.\n")
				os.Exit(1)
			}
		}
	}

	if !hasCliArg {
		return nil // GUI mode
	}

	return opts
}

// runCLI executes the CLI command.
func runCLI(opts *CLIOptions, logger *zap.Logger) error {
	// Handle help first
	if opts.Help {
		printHelp()
		return nil
	}

	// Open database
	db, err := openDatabase()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Handle list-jobs
	if opts.ListJobs {
		return runListJobs(db)
	}

	// Handle dehydrate
	if opts.DehydrateJobID > 0 {
		return runDehydrate(db, opts.DehydrateJobID, opts.DehydrateDays, logger)
	}

	// For sync operations, we need the engine
	if opts.SyncJobID > 0 || opts.SyncAll {
		cfg, err := config.Load("")
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		engine, err := sync.NewEngine(cfg, db, logger)
		if err != nil {
			return fmt.Errorf("failed to create sync engine: %w", err)
		}
		defer engine.Close()

		if opts.SyncJobID > 0 {
			return runSyncJob(db, engine, opts.SyncJobID, logger)
		}
		if opts.SyncAll {
			return runSyncAll(db, engine, logger)
		}
	}

	// No action specified
	printHelp()
	return nil
}

// openDatabase opens the encrypted SQLite database.
func openDatabase() (*database.DB, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = "."
	}
	dbPath := filepath.Join(localAppData, "AnemoneSync", "data", "anemonesync.db")

	cfg := database.Config{
		Path:             dbPath,
		EncryptionKey:    "AnemoneSync_DefaultKey_ChangeMe", // Same as GUI
		CreateIfNotExist: false,                             // CLI shouldn't create new DB
	}

	return database.Open(cfg)
}

// printHelp displays usage information.
func printHelp() {
	fmt.Println(`AnemoneSync CLI

Usage:
  anemonesync [options]

Options:
  -l, --list-jobs          List all configured sync jobs
  -s, --sync <id>          Sync a specific job by ID
  -a, --sync-all           Sync all enabled jobs
  -d, --dehydrate <id>     Free up space by dehydrating files (Files On Demand)
      --days <n>           Only dehydrate files not accessed for N days (default: job setting, 0 = all)
  -h, --help               Show this help message

Without options, starts the GUI application.

Examples:
  anemonesync --list-jobs
  anemonesync --sync 1
  anemonesync --sync-all
  anemonesync --dehydrate 1              # Use job's auto-dehydrate setting
  anemonesync --dehydrate 1 --days 30    # Files not accessed for 30+ days
  anemonesync --dehydrate 1 --days 0     # All hydrated files`)
}

// runListJobs lists all configured sync jobs.
func runListJobs(db *database.DB) error {
	jobs, err := db.GetAllSyncJobs()
	if err != nil {
		return fmt.Errorf("failed to get jobs: %w", err)
	}

	if len(jobs) == 0 {
		fmt.Println("No sync jobs configured.")
		fmt.Println("Use the GUI to create sync jobs.")
		return nil
	}

	fmt.Println("AnemoneSync - Configured Jobs")
	fmt.Println()

	// Print header
	fmt.Printf("%-4s %-20s %-35s %-35s %-8s %s\n",
		"ID", "Name", "Local Path", "Remote", "Enabled", "Last Sync")
	fmt.Println(strings.Repeat("-", 140))

	enabledCount := 0
	for _, job := range jobs {
		enabled := "No"
		if job.Enabled {
			enabled = "Yes"
			enabledCount++
		}

		lastSync := "Never"
		if job.LastRun != nil {
			lastSync = job.LastRun.Format("2006-01-02 15:04")
		}

		// Truncate paths if too long
		localPath := truncatePath(job.LocalPath, 35)
		remotePath := truncatePath(job.RemotePath, 35)
		name := truncateString(job.Name, 20)

		fmt.Printf("%-4d %-20s %-35s %-35s %-8s %s\n",
			job.ID, name, localPath, remotePath, enabled, lastSync)
	}

	fmt.Println()
	fmt.Printf("Total: %d jobs (%d enabled)\n", len(jobs), enabledCount)

	return nil
}

// runSyncJob syncs a specific job by ID.
func runSyncJob(db *database.DB, engine *sync.Engine, jobID int64, logger *zap.Logger) error {
	job, err := db.GetSyncJob(jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}
	if job == nil {
		return fmt.Errorf("job with ID %d not found", jobID)
	}

	fmt.Printf("Syncing \"%s\" (ID: %d)\n", job.Name, job.ID)
	fmt.Printf("  Local:  %s\n", job.LocalPath)
	fmt.Printf("  Remote: %s\n", job.RemotePath)
	fmt.Println()

	req := buildSyncRequest(job, createCLIProgressCallback(job.Name))

	ctx := context.Background()
	startTime := time.Now()

	result, err := engine.Sync(ctx, req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return err
	}

	duration := time.Since(startTime)

	// Print summary
	fmt.Println()
	printSyncSummary(result, duration)

	return nil
}

// runSyncAll syncs all enabled jobs.
func runSyncAll(db *database.DB, engine *sync.Engine, logger *zap.Logger) error {
	jobs, err := db.GetAllSyncJobs()
	if err != nil {
		return fmt.Errorf("failed to get jobs: %w", err)
	}

	// Filter enabled jobs
	var enabledJobs []*database.SyncJob
	for _, job := range jobs {
		if job.Enabled {
			enabledJobs = append(enabledJobs, job)
		}
	}

	if len(enabledJobs) == 0 {
		fmt.Println("No enabled jobs to sync.")
		return nil
	}

	fmt.Printf("Syncing all enabled jobs (%d of %d)\n", len(enabledJobs), len(jobs))
	fmt.Println()

	totalStartTime := time.Now()
	totalFiles := 0
	errorCount := 0
	jobsSynced := 0

	for i, job := range enabledJobs {
		fmt.Printf("[%d/%d] Syncing \"%s\"...\n", i+1, len(enabledJobs), job.Name)

		req := buildSyncRequest(job, createCLIProgressCallback(job.Name))

		ctx := context.Background()
		startTime := time.Now()

		result, err := engine.Sync(ctx, req)
		duration := time.Since(startTime)

		if err != nil {
			fmt.Printf("      Error: %v\n", err)
			errorCount++
			continue
		}

		filesProcessed := result.FilesUploaded + result.FilesDownloaded + result.FilesDeleted
		totalFiles += filesProcessed
		jobsSynced++

		fmt.Printf("      Complete (%.1fs, %d files)\n", duration.Seconds(), filesProcessed)
		fmt.Println()
	}

	totalDuration := time.Since(totalStartTime)

	fmt.Println("All syncs completed.")
	fmt.Printf("  Total duration: %.1fs\n", totalDuration.Seconds())
	fmt.Printf("  Jobs synced: %d\n", jobsSynced)
	fmt.Printf("  Total files: %d\n", totalFiles)
	fmt.Printf("  Errors: %d\n", errorCount)

	return nil
}

// buildSyncRequest creates a SyncRequest from a database SyncJob.
func buildSyncRequest(job *database.SyncJob, progressCb sync.ProgressCallback) *sync.SyncRequest {
	mode := sync.SyncMode(job.SyncMode)
	if !mode.IsValid() {
		mode = sync.SyncModeMirror
	}

	conflictRes := job.ConflictResolution
	if conflictRes == "" {
		conflictRes = "recent"
	}

	return &sync.SyncRequest{
		JobID:              job.ID,
		LocalPath:          job.LocalPath,
		RemotePath:         job.RemotePath,
		Mode:               mode,
		ConflictResolution: conflictRes,
		ProgressCallback:   progressCb,
	}
}

// createCLIProgressCallback creates a progress callback for terminal output.
func createCLIProgressCallback(jobName string) sync.ProgressCallback {
	lastPhase := ""
	return func(progress *sync.SyncProgress) {
		if progress.Phase != lastPhase {
			lastPhase = progress.Phase
			switch progress.Phase {
			case "scanning":
				fmt.Printf("[Scanning]     %s\n", progress.Message)
			case "detecting":
				fmt.Printf("[Detecting]    %s\n", progress.Message)
			case "executing":
				// Will be updated with progress bar
			case "finalizing":
				fmt.Printf("[Finalizing]   %s\n", progress.Message)
			}
		}

		// Show progress bar during execution phase
		if progress.Phase == "executing" && progress.FilesTotal > 0 {
			printProgressBar(progress.FilesProcessed, progress.FilesTotal)
		}
	}
}

// printProgressBar prints a progress bar to the terminal.
func printProgressBar(current, total int) {
	const barWidth = 32

	percent := float64(current) / float64(total)
	filled := int(percent * barWidth)

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	fmt.Printf("\r[Executing]    %s %d/%d (%.0f%%)", bar, current, total, percent*100)

	if current >= total {
		fmt.Println()
	}
}

// printSyncSummary prints a sync result summary.
func printSyncSummary(result *sync.SyncResult, duration time.Duration) {
	fmt.Printf("[Complete]     Duration: %.1fs\n", duration.Seconds())
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  Uploaded:    %d files\n", result.FilesUploaded)
	fmt.Printf("  Downloaded:  %d files\n", result.FilesDownloaded)
	fmt.Printf("  Deleted:     %d files\n", result.FilesDeleted)
	fmt.Printf("  Skipped:     %d files\n", result.FilesSkipped)
	fmt.Printf("  Errors:      %d\n", result.FilesError)

	if result.BytesTransferred > 0 {
		fmt.Printf("  Transferred: %s\n", formatBytes(result.BytesTransferred))
	}
}

// truncatePath truncates a path to maxLen, preserving the end.
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

// truncateString truncates a string to maxLen.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// formatBytes formats bytes as human-readable string.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// runDehydrate dehydrates files for a job with Files On Demand enabled.
func runDehydrate(db *database.DB, jobID int64, days int, logger *zap.Logger) error {
	// Get job
	job, err := db.GetSyncJob(jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}
	if job == nil {
		return fmt.Errorf("job with ID %d not found", jobID)
	}

	// Parse job options
	opts := app.ParseJobOptions(job.NetworkConditions)
	if !opts.FilesOnDemand {
		return fmt.Errorf("job \"%s\" does not have Files On Demand enabled", job.Name)
	}

	// Determine days threshold
	daysThreshold := days
	if days < 0 {
		// Use job's auto-dehydrate setting
		daysThreshold = opts.AutoDehydrateDays
		if daysThreshold == 0 {
			fmt.Println("Note: Job has no auto-dehydrate setting. Use --days 0 to dehydrate all files.")
			return nil
		}
	}

	fmt.Printf("Dehydrating \"%s\" (ID: %d)\n", job.Name, job.ID)
	fmt.Printf("  Local path: %s\n", job.LocalPath)
	if daysThreshold > 0 {
		fmt.Printf("  Threshold:  Files not accessed for %d+ days\n", daysThreshold)
	} else {
		fmt.Printf("  Threshold:  All hydrated files\n")
	}
	fmt.Println()

	// Create sync root manager
	syncRootConfig := cloudfiles.SyncRootConfig{
		Path:            job.LocalPath,
		ProviderName:    "AnemoneSync",
		ProviderVersion: "1.0.0",
	}

	syncRoot, err := cloudfiles.NewSyncRootManager(syncRootConfig)
	if err != nil {
		return fmt.Errorf("failed to create sync root manager: %w", err)
	}

	// Create dehydration manager with policy
	policy := cloudfiles.DehydrationPolicy{
		MaxAgeDays:  daysThreshold,
		MinFileSize: 0, // No minimum size
	}

	dm := cloudfiles.NewDehydrationManager(syncRoot, policy, logger)

	// Scan for hydrated files
	ctx := context.Background()
	fmt.Println("[Scanning]     Looking for hydrated files...")

	hydratedFiles, err := dm.ScanHydratedFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to scan: %w", err)
	}

	if len(hydratedFiles) == 0 {
		fmt.Println("[Complete]     No hydrated files found.")
		return nil
	}

	// Filter eligible files
	var eligible []cloudfiles.HydratedFileInfo
	var totalSize int64
	for _, f := range hydratedFiles {
		if daysThreshold == 0 || f.DaysSinceAccess >= daysThreshold {
			eligible = append(eligible, f)
			totalSize += f.Size
		}
	}

	if len(eligible) == 0 {
		fmt.Printf("[Complete]     Found %d hydrated files, but none meet the criteria.\n", len(hydratedFiles))
		return nil
	}

	fmt.Printf("[Found]        %d files eligible for dehydration (%s)\n", len(eligible), formatBytes(totalSize))
	fmt.Println()

	// Dehydrate files
	dehydrated := 0
	var freedBytes int64
	errors := 0

	for i, file := range eligible {
		// Progress
		percent := float64(i+1) / float64(len(eligible)) * 100
		fmt.Printf("\r[Dehydrating]  %d/%d (%.0f%%) - %s", i+1, len(eligible), percent, truncateString(file.Path, 40))

		if err := dm.DehydrateFile(ctx, file.Path); err != nil {
			errors++
			logger.Warn("failed to dehydrate file",
				zap.String("path", file.Path),
				zap.Error(err),
			)
			continue
		}

		dehydrated++
		freedBytes += file.Size
	}

	fmt.Println()
	fmt.Println()

	// Summary
	fmt.Println("[Complete]     Dehydration finished.")
	fmt.Printf("  Files dehydrated: %d\n", dehydrated)
	fmt.Printf("  Space freed:      %s\n", formatBytes(freedBytes))
	if errors > 0 {
		fmt.Printf("  Errors:           %d\n", errors)
	}

	return nil
}
