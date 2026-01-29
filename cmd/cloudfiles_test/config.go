//go:build windows
// +build windows

// Package main provides automated testing for Cloud Files API.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// Config holds the test configuration.
type Config struct {
	// LocalSyncRoot is the sync root path (e.g., D:\test_anemone)
	LocalSyncRoot string `json:"local_sync_root"`

	// TestSubdir is the subdirectory for tests (e.g., _cftest)
	TestSubdir string `json:"test_subdir"`

	// Remote SMB settings
	RemoteHost  string `json:"remote_host"`
	RemoteShare string `json:"remote_share"`
	RemotePath  string `json:"remote_path"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Domain      string `json:"domain,omitempty"`

	// SourceDir contains files to use for tests (e.g., D:\temp)
	SourceDir string `json:"source_dir"`
}

// LocalTestPath returns the full local path for tests.
func (c *Config) LocalTestPath() string {
	return filepath.Join(c.LocalSyncRoot, c.TestSubdir)
}

// RemoteTestPath returns the full remote path for tests.
func (c *Config) RemoteTestPath() string {
	return filepath.Join(c.RemotePath, c.TestSubdir)
}

// UNCPath returns the full UNC path for the test directory.
func (c *Config) UNCPath() string {
	return fmt.Sprintf("\\\\%s\\%s\\%s\\%s", c.RemoteHost, c.RemoteShare, c.RemotePath, c.TestSubdir)
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	return filepath.Join(os.TempDir(), "cloudfiles_test_config.json")
}

// LoadConfig loads the configuration from disk.
func LoadConfig() (*Config, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// SaveConfig saves the configuration to disk.
func SaveConfig(cfg *Config) error {
	path := ConfigPath()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600) // Restricted permissions
}

// PromptConfig interactively prompts the user for configuration.
func PromptConfig() (*Config, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\nConfiguration non trouvée. Création...")
	fmt.Println()

	cfg := &Config{}

	// Local sync root
	fmt.Print("Sync root local [D:\\test_anemone]: ")
	syncRoot, _ := reader.ReadString('\n')
	syncRoot = strings.TrimSpace(syncRoot)
	if syncRoot == "" {
		syncRoot = "D:\\test_anemone"
	}
	cfg.LocalSyncRoot = syncRoot

	// Test subdirectory
	fmt.Print("Sous-dossier de test [_cftest]: ")
	subdir, _ := reader.ReadString('\n')
	subdir = strings.TrimSpace(subdir)
	if subdir == "" {
		subdir = "_cftest"
	}
	cfg.TestSubdir = subdir

	// Remote host
	fmt.Print("Serveur SMB [192.168.83.221]: ")
	host, _ := reader.ReadString('\n')
	host = strings.TrimSpace(host)
	if host == "" {
		host = "192.168.83.221"
	}
	cfg.RemoteHost = host

	// Remote share
	fmt.Print("Share [data_franck]: ")
	share, _ := reader.ReadString('\n')
	share = strings.TrimSpace(share)
	if share == "" {
		share = "data_franck"
	}
	cfg.RemoteShare = share

	// Remote path
	fmt.Print("Chemin dans le share [test_anemone]: ")
	path, _ := reader.ReadString('\n')
	path = strings.TrimSpace(path)
	if path == "" {
		path = "test_anemone"
	}
	cfg.RemotePath = path

	// Username
	fmt.Print("Utilisateur: ")
	username, _ := reader.ReadString('\n')
	cfg.Username = strings.TrimSpace(username)

	// Password (hidden input)
	fmt.Print("Mot de passe: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return nil, fmt.Errorf("erreur lecture mot de passe: %w", err)
	}
	fmt.Println()
	cfg.Password = string(passwordBytes)

	// Domain (optional)
	fmt.Print("Domaine (vide si aucun): ")
	domain, _ := reader.ReadString('\n')
	cfg.Domain = strings.TrimSpace(domain)

	// Source directory
	fmt.Print("Dossier source fichiers [D:\\temp]: ")
	sourceDir, _ := reader.ReadString('\n')
	sourceDir = strings.TrimSpace(sourceDir)
	if sourceDir == "" {
		sourceDir = "D:\\temp"
	}
	cfg.SourceDir = sourceDir

	return cfg, nil
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		LocalSyncRoot: "D:\\test_anemone",
		TestSubdir:    "_cftest",
		RemoteHost:    "192.168.83.221",
		RemoteShare:   "data_franck",
		RemotePath:    "test_anemone",
		SourceDir:     "D:\\temp",
	}
}

// Validate checks the configuration is valid.
func (c *Config) Validate() error {
	if c.LocalSyncRoot == "" {
		return fmt.Errorf("local_sync_root is required")
	}
	if c.RemoteHost == "" {
		return fmt.Errorf("remote_host is required")
	}
	if c.RemoteShare == "" {
		return fmt.Errorf("remote_share is required")
	}
	if c.Username == "" {
		return fmt.Errorf("username is required")
	}
	if c.Password == "" {
		return fmt.Errorf("password is required")
	}
	return nil
}
