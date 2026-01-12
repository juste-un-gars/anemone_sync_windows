package smb

import (
	"encoding/json"
	"fmt"

	"github.com/zalando/go-keyring"
	"go.uber.org/zap"
)

const (
	// ServiceName is the name used to identify credentials in the system keyring
	ServiceName = "anemone-sync-smb"
)

// Credentials represents SMB connection credentials
type Credentials struct {
	Server   string `json:"server"`
	Share    string `json:"share"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Domain   string `json:"domain,omitempty"`
}

// CredentialManager handles secure storage and retrieval of SMB credentials
type CredentialManager struct {
	logger *zap.Logger
}

// NewCredentialManager creates a new credential manager
func NewCredentialManager(logger *zap.Logger) *CredentialManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CredentialManager{
		logger: logger.With(zap.String("component", "credential-manager")),
	}
}

// makeKey creates a unique key for the credentials based on server and share
func makeKey(server, share string) string {
	return fmt.Sprintf("%s:%s", server, share)
}

// Save stores credentials securely in the system keyring
// The credentials are stored as JSON under a key derived from server:share
func (cm *CredentialManager) Save(creds *Credentials) error {
	if creds == nil {
		return fmt.Errorf("credentials cannot be nil")
	}
	if creds.Server == "" {
		return fmt.Errorf("server cannot be empty")
	}
	if creds.Share == "" {
		return fmt.Errorf("share cannot be empty")
	}
	if creds.Username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	key := makeKey(creds.Server, creds.Share)

	// Marshal credentials to JSON
	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Store in keyring
	if err := keyring.Set(ServiceName, key, string(data)); err != nil {
		return fmt.Errorf("failed to store credentials in keyring: %w", err)
	}

	cm.logger.Info("credentials saved to keyring",
		zap.String("server", creds.Server),
		zap.String("share", creds.Share))

	return nil
}

// Load retrieves credentials from the system keyring
func (cm *CredentialManager) Load(server, share string) (*Credentials, error) {
	if server == "" {
		return nil, fmt.Errorf("server cannot be empty")
	}
	if share == "" {
		return nil, fmt.Errorf("share cannot be empty")
	}

	key := makeKey(server, share)

	// Get from keyring
	data, err := keyring.Get(ServiceName, key)
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials from keyring: %w", err)
	}

	// Unmarshal JSON
	var creds Credentials
	if err := json.Unmarshal([]byte(data), &creds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	cm.logger.Info("credentials loaded from keyring",
		zap.String("server", server),
		zap.String("share", share))

	return &creds, nil
}

// Delete removes credentials from the system keyring
func (cm *CredentialManager) Delete(server, share string) error {
	if server == "" {
		return fmt.Errorf("server cannot be empty")
	}
	if share == "" {
		return fmt.Errorf("share cannot be empty")
	}

	key := makeKey(server, share)

	// Delete from keyring
	if err := keyring.Delete(ServiceName, key); err != nil {
		return fmt.Errorf("failed to delete credentials from keyring: %w", err)
	}

	cm.logger.Info("credentials deleted from keyring",
		zap.String("server", server),
		zap.String("share", share))

	return nil
}

// Exists checks if credentials exist in the keyring for the given server and share
func (cm *CredentialManager) Exists(server, share string) bool {
	if server == "" || share == "" {
		return false
	}

	key := makeKey(server, share)

	// Try to get from keyring
	_, err := keyring.Get(ServiceName, key)
	return err == nil
}
