package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger représente le système de logging de l'application
type Logger struct {
	zap *zap.Logger
}

// Config contient la configuration du logger
type Config struct {
	Level      string // debug, info, warning, error, critical
	OutputPath string // Chemin du fichier de log
	MaxSizeMB  int    // Taille maximale d'un fichier de log
	MaxFiles   int    // Nombre de fichiers à conserver
	Compress   bool   // Compresser les anciens logs
}

// New crée un nouveau logger
func New(cfg Config) (*Logger, error) {
	// Créer le répertoire de logs si nécessaire
	logDir := filepath.Dir(cfg.OutputPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("impossible de créer le répertoire de logs: %w", err)
	}

	// Configuration de l'encodeur
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Niveau de log
	level := parseLogLevel(cfg.Level)

	// Configuration Zap
	zapConfig := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      false,
		Encoding:         "json",
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{cfg.OutputPath, "stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	// Créer le logger
	zapLogger, err := zapConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("impossible de créer le logger: %w", err)
	}

	return &Logger{
		zap: zapLogger,
	}, nil
}

// Debug log un message de debug
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.zap.Debug(msg, fields...)
}

// Info log un message d'information
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.zap.Info(msg, fields...)
}

// Warn log un avertissement
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.zap.Warn(msg, fields...)
}

// Error log une erreur
func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.zap.Error(msg, fields...)
}

// Fatal log une erreur fatale et arrête l'application
func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.zap.Fatal(msg, fields...)
}

// Sync flush les buffers de log
func (l *Logger) Sync() error {
	return l.zap.Sync()
}

// With crée un logger dérivé avec des champs supplémentaires
func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{
		zap: l.zap.With(fields...),
	}
}

// parseLogLevel convertit une string en niveau de log zap
func parseLogLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warning", "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "critical", "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// Global logger instance (à utiliser avec précaution)
var defaultLogger *Logger

// InitDefault initialise le logger par défaut
func InitDefault(cfg Config) error {
	logger, err := New(cfg)
	if err != nil {
		return err
	}
	defaultLogger = logger
	return nil
}

// Default retourne le logger par défaut
func Default() *Logger {
	if defaultLogger == nil {
		// Créer un logger basique si aucun n'est configuré
		zapLogger, _ := zap.NewProduction()
		defaultLogger = &Logger{zap: zapLogger}
	}
	return defaultLogger
}
