// Package harness provides automated testing for AnemoneSync.
package harness

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

// Config holds the test harness configuration.
type Config struct {
	LocalBase      string `json:"local_base"`
	RemoteBase     string `json:"remote_base,omitempty"`      // Mapped drive path (e.g., Z:\TEST)
	UseMappedDrive bool   `json:"use_mapped_drive,omitempty"` // Use mapped drive instead of SMB
	RemoteHost     string `json:"remote_host"`
	RemoteShare    string `json:"remote_share"`
	RemotePath     string `json:"remote_path"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	Domain         string `json:"domain,omitempty"`
}

// Jobs returns the list of test jobs.
func (c *Config) Jobs() []JobConfig {
	return []JobConfig{
		{Name: "TEST1", Mode: "mirror", Description: "Mirror bidirectionnel"},
		{Name: "TEST2", Mode: "upload", Description: "PC → Serveur"},
		{Name: "TEST3", Mode: "download", Description: "Serveur → PC"},
		{Name: "TEST4", Mode: "mirror", Description: "Conflits"},
		{Name: "TEST5", Mode: "mirror", Description: "Stress/Volume"},
		{Name: "TEST6", Mode: "mirror", Description: "Edge cases"},
		{Name: "TEST7", Mode: "mirror", Description: "Resilience reseau (interactif)"},
	}
}

// JobConfig defines a test job.
type JobConfig struct {
	Name        string `json:"name"`
	Mode        string `json:"mode"`
	Description string `json:"description"`
}

// LocalPath returns the full local path for a job.
func (c *Config) LocalPath(job string) string {
	return filepath.Join(c.LocalBase, job)
}

// RemotePathForJob returns the full remote path for a job (relative to share or mapped drive).
func (c *Config) RemotePathForJob(job string) string {
	if c.UseMappedDrive && c.RemoteBase != "" {
		return filepath.Join(c.RemoteBase, job)
	}
	return filepath.Join(c.RemotePath, job)
}

// UNCPath returns the full UNC path for a job.
func (c *Config) UNCPath(job string) string {
	return fmt.Sprintf("\\\\%s\\%s\\%s\\%s", c.RemoteHost, c.RemoteShare, c.RemotePath, job)
}

// ConfigPath returns the path to the config file.
func ConfigPath(baseDir string) string {
	return filepath.Join(baseDir, "_harness", "config.json")
}

// DBPath returns the path to the test database.
func DBPath(baseDir string) string {
	return filepath.Join(baseDir, "_harness", "test.db")
}

// ResultsDir returns the path to the results directory.
func ResultsDir(baseDir string) string {
	return filepath.Join(baseDir, "_harness", "results")
}

// LoadConfig loads the configuration from disk.
func LoadConfig(baseDir string) (*Config, error) {
	path := ConfigPath(baseDir)
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
func SaveConfig(baseDir string, cfg *Config) error {
	path := ConfigPath(baseDir)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600) // Restricted permissions for credentials
}

// PromptConfig interactively prompts the user for configuration.
func PromptConfig(baseDir string) (*Config, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\nConfiguration non trouvée. Création...")
	fmt.Println()

	cfg := &Config{
		LocalBase: baseDir,
	}

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
	fmt.Print("Chemin dans le share [TEST]: ")
	path, _ := reader.ReadString('\n')
	path = strings.TrimSpace(path)
	if path == "" {
		path = "TEST"
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

	return cfg, nil
}

// DefaultConfig returns a default configuration for testing.
func DefaultConfig() *Config {
	return &Config{
		LocalBase:   "D:\\TEST",
		RemoteHost:  "192.168.83.221",
		RemoteShare: "data_franck",
		RemotePath:  "TEST",
	}
}
