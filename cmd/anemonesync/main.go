// AnemoneSync - SMB Synchronization Client
// Entry point for the desktop application and CLI.
package main

import (
	"fmt"
	"os"

	"github.com/juste-un-gars/anemone_sync_windows/internal/app"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	// Initialize logger with dynamic level support
	logger, logLevel := initLogger()
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
	application := app.New(logger, logLevel)
	application.SetAutoStartMode(isAutoStart)
	application.Run()
}

// initLogger creates a configured zap logger with a dynamic log level and file rotation.
// Returns the logger and the AtomicLevel for runtime level changes.
func initLogger() (*zap.Logger, zap.AtomicLevel) {
	// Dynamic log level (default: Info, can be changed at runtime)
	atomicLevel := zap.NewAtomicLevelAt(zapcore.InfoLevel)

	// Encoder config for human-readable output
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Console encoder for stdout
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	// Create cores: stdout + file with rotation
	var cores []zapcore.Core

	// Always log to stdout
	cores = append(cores, zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		atomicLevel,
	))

	// Log to file with rotation (lumberjack)
	logPath := getLogPath()
	if logPath != "" {
		fileWriter := &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    10,   // 10 MB max before rotation
			MaxBackups: 10,   // Keep 10 old files
			MaxAge:     30,   // Delete files older than 30 days
			Compress:   true, // Compress rotated files (.gz)
		}
		cores = append(cores, zapcore.NewCore(
			consoleEncoder,
			zapcore.AddSync(fileWriter),
			atomicLevel,
		))
	}

	// Combine cores
	core := zapcore.NewTee(cores...)

	// Build logger with caller info
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, atomicLevel
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
