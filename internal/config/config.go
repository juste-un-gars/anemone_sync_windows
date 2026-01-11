package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

// Config représente la configuration de l'application
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
	Paths    PathsConfig    `mapstructure:"paths"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Sync     SyncConfig     `mapstructure:"sync"`
	UI       UIConfig       `mapstructure:"ui"`
	Security SecurityConfig `mapstructure:"security"`
	Advanced AdvancedConfig `mapstructure:"advanced"`
}

type AppConfig struct {
	Name     string `mapstructure:"name"`
	Version  string `mapstructure:"version"`
	LogLevel string `mapstructure:"log_level"`
	Language string `mapstructure:"language"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type PathsConfig struct {
	ConfigDir string `mapstructure:"config_dir"`
	LogDir    string `mapstructure:"log_dir"`
}

type LoggingConfig struct {
	Rotation LogRotationConfig `mapstructure:"rotation"`
	Levels   LogLevelsConfig   `mapstructure:"levels"`
}

type LogRotationConfig struct {
	MaxSizeMB int  `mapstructure:"max_size_mb"`
	MaxFiles  int  `mapstructure:"max_files"`
	Compress  bool `mapstructure:"compress"`
}

type LogLevelsConfig struct {
	Console string `mapstructure:"console"`
	File    string `mapstructure:"file"`
}

type SyncConfig struct {
	DefaultMode               string              `mapstructure:"default_mode"`
	DefaultTrigger            string              `mapstructure:"default_trigger"`
	DefaultConflictResolution string              `mapstructure:"default_conflict_resolution"`
	Realtime                  RealtimeConfig      `mapstructure:"realtime"`
	Performance               PerformanceConfig   `mapstructure:"performance"`
	Network                   NetworkConfig       `mapstructure:"network"`
}

type RealtimeConfig struct {
	DebounceSeconds      int `mapstructure:"debounce_seconds"`
	BatchIntervalMinutes int `mapstructure:"batch_interval_minutes"`
}

type PerformanceConfig struct {
	ParallelTransfers int    `mapstructure:"parallel_transfers"`
	BufferSizeMB      int    `mapstructure:"buffer_size_mb"`
	HashAlgorithm     string `mapstructure:"hash_algorithm"`
}

type NetworkConfig struct {
	RequireWifi        bool `mapstructure:"require_wifi"`
	RequireData        bool `mapstructure:"require_data"`
	EnableOfflineQueue bool `mapstructure:"enable_offline_queue"`
}

type UIConfig struct {
	StartMinimized    bool                      `mapstructure:"start_minimized"`
	ShowNotifications bool                      `mapstructure:"show_notifications"`
	NotificationTypes NotificationTypesConfig   `mapstructure:"notification_types"`
	TrayIcon          TrayIconConfig            `mapstructure:"tray_icon"`
}

type NotificationTypesConfig struct {
	SyncCompleted bool `mapstructure:"sync_completed"`
	SyncErrors    bool `mapstructure:"sync_errors"`
	Conflicts     bool `mapstructure:"conflicts"`
	OfflineQueue  bool `mapstructure:"offline_queue"`
	DiskSpaceLow  bool `mapstructure:"disk_space_low"`
}

type TrayIconConfig struct {
	ShowSyncProgress bool `mapstructure:"show_sync_progress"`
	ShowErrorCount   bool `mapstructure:"show_error_count"`
}

type SecurityConfig struct {
	KeystoreServiceName  string `mapstructure:"keystore_service_name"`
	ZeroMemoryAfterUse   bool   `mapstructure:"zero_memory_after_use"`
	EnableSMB3Encryption bool   `mapstructure:"enable_smb3_encryption"`
}

type AdvancedConfig struct {
	Throttling  ThrottlingConfig  `mapstructure:"throttling"`
	Compression CompressionConfig `mapstructure:"compression"`
	DeltaSync   DeltaSyncConfig   `mapstructure:"delta_sync"`
}

type ThrottlingConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	MaxBandwidthMbps int      `mapstructure:"max_bandwidth_mbps"`
	Schedule         []string `mapstructure:"schedule"`
}

type CompressionConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Algorithm string `mapstructure:"algorithm"`
}

type DeltaSyncConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// Load charge la configuration depuis le fichier par défaut ou spécifié
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Définir les chemins de recherche de configuration
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Chercher dans les emplacements standards
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
		v.AddConfigPath(getDefaultConfigDir())
	}

	// Lire la configuration
	if err := v.ReadInConfig(); err != nil {
		// Si le fichier n'existe pas, utiliser les valeurs par défaut
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("erreur lecture config: %w", err)
		}
	}

	// Charger les valeurs par défaut depuis le fichier embarqué
	setDefaults(v)

	// Permettre les variables d'environnement
	v.SetEnvPrefix("ANEMONE")
	v.AutomaticEnv()

	// Decoder dans la structure
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("erreur décodage config: %w", err)
	}

	// Remplacer les variables d'environnement dans les chemins
	config.Paths.ConfigDir = expandPath(config.Paths.ConfigDir)
	config.Paths.LogDir = expandPath(config.Paths.LogDir)
	config.Database.Path = expandPath(config.Database.Path)

	return &config, nil
}

// getDefaultConfigDir retourne le répertoire de configuration par défaut selon l'OS
func getDefaultConfigDir() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "AnemoneSync")
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "AnemoneSync")
	default: // Linux et autres
		return filepath.Join(os.Getenv("HOME"), ".config", "anemone_sync")
	}
}

// expandPath remplace ${HOME} et autres variables dans les chemins
func expandPath(path string) string {
	if path == "" {
		return path
	}

	// Remplacer ${HOME} ou $HOME
	home, _ := os.UserHomeDir()
	path = os.Expand(path, func(key string) string {
		switch key {
		case "HOME":
			return home
		default:
			return os.Getenv(key)
		}
	})

	return path
}

// setDefaults définit les valeurs par défaut
func setDefaults(v *viper.Viper) {
	// App
	v.SetDefault("app.name", "AnemoneSync")
	v.SetDefault("app.version", "0.1.0-dev")
	v.SetDefault("app.log_level", "info")
	v.SetDefault("app.language", "fr")

	// Database
	v.SetDefault("database.path", filepath.Join(getDefaultConfigDir(), "anemone_sync.db"))

	// Paths
	v.SetDefault("paths.config_dir", getDefaultConfigDir())
	v.SetDefault("paths.log_dir", filepath.Join(getDefaultConfigDir(), "logs"))

	// Logging
	v.SetDefault("logging.rotation.max_size_mb", 10)
	v.SetDefault("logging.rotation.max_files", 5)
	v.SetDefault("logging.rotation.compress", true)
	v.SetDefault("logging.levels.console", "info")
	v.SetDefault("logging.levels.file", "debug")

	// Sync
	v.SetDefault("sync.default_mode", "mirror")
	v.SetDefault("sync.default_trigger", "realtime")
	v.SetDefault("sync.default_conflict_resolution", "recent")
	v.SetDefault("sync.realtime.debounce_seconds", 30)
	v.SetDefault("sync.realtime.batch_interval_minutes", 5)
	v.SetDefault("sync.performance.parallel_transfers", 4)
	v.SetDefault("sync.performance.buffer_size_mb", 4)
	v.SetDefault("sync.performance.hash_algorithm", "sha256")
	v.SetDefault("sync.network.require_wifi", false)
	v.SetDefault("sync.network.require_data", false)
	v.SetDefault("sync.network.enable_offline_queue", true)

	// UI
	v.SetDefault("ui.start_minimized", false)
	v.SetDefault("ui.show_notifications", true)

	// Security
	v.SetDefault("security.keystore_service_name", "AnemoneSync")
	v.SetDefault("security.zero_memory_after_use", true)
	v.SetDefault("security.enable_smb3_encryption", true)
}
