// AnemoneSync - SMB Synchronization Client
// Entry point for the desktop application and CLI.
package main

import (
	"fmt"
	"os"

	"github.com/juste-un-gars/anemone_sync_windows/internal/app"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Initialize logger
	logger := initLogger()
	defer logger.Sync()

	// Check CLI mode first
	if opts := parseCLIArgs(os.Args[1:]); opts != nil {
		if err := runCLI(opts, logger); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// GUI mode - check for --autostart flag
	isAutoStart := false
	for _, arg := range os.Args[1:] {
		if arg == "--autostart" {
			isAutoStart = true
			break
		}
	}

	if isAutoStart {
		logger.Info("Application started via Windows autostart")
	}

	// Create and run application
	application := app.New(logger)
	application.SetAutoStartMode(isAutoStart)
	application.Run()
}

// initLogger creates a configured zap logger.
func initLogger() *zap.Logger {
	// Development config for now - switch to production later
	config := zap.NewDevelopmentConfig()

	// Log to file in user's app data directory
	logPath := getLogPath()
	if logPath != "" {
		config.OutputPaths = []string{"stdout", logPath}
	}

	// Set log level
	config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)

	logger, err := config.Build()
	if err != nil {
		// Fallback to basic logger
		logger, _ = zap.NewDevelopment()
	}

	return logger
}

// getLogPath returns the path for the log file.
func getLogPath() string {
	// Use %LOCALAPPDATA%\AnemoneSync\logs on Windows
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return ""
	}

	logDir := localAppData + "\\AnemoneSync\\logs"

	// Create directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return ""
	}

	return logDir + "\\anemonesync.log"
}
