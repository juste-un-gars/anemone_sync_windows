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

// Save stores credentials securely in the system keyring
// The credentials are stored as JSON under the server hostname as key
func (cm *CredentialManager) Save(creds *Credentials) error {
	if creds == nil {
		return fmt.Errorf("credentials cannot be nil")
	}
	if creds.Server == "" {
		return fmt.Errorf("server cannot be empty")
	}
	if creds.Username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	// Marshal credentials to JSON
	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Store in keyring using server as key
	if err := keyring.Set(ServiceName, creds.Server, string(data)); err != nil {
		return fmt.Errorf("failed to store credentials in keyring: %w", err)
	}

	cm.logger.Info("credentials saved to keyring",
		zap.String("server", creds.Server))

	return nil
}

// Load retrieves credentials from the system keyring
func (cm *CredentialManager) Load(server string) (*Credentials, error) {
	if server == "" {
		return nil, fmt.Errorf("server cannot be empty")
	}

	// Get from keyring using server as key
	data, err := keyring.Get(ServiceName, server)
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials from keyring: %w", err)
	}

	// Unmarshal JSON
	var creds Credentials
	if err := json.Unmarshal([]byte(data), &creds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	cm.logger.Info("credentials loaded from keyring",
		zap.String("server", server))

	return &creds, nil
}

// Delete removes credentials from the system keyring
func (cm *CredentialManager) Delete(server string) error {
	if server == "" {
		return fmt.Errorf("server cannot be empty")
	}

	// Delete from keyring
	if err := keyring.Delete(ServiceName, server); err != nil {
		return fmt.Errorf("failed to delete credentials from keyring: %w", err)
	}

	cm.logger.Info("credentials deleted from keyring",
		zap.String("server", server))

	return nil
}

// Exists checks if credentials exist in the keyring for the given server
func (cm *CredentialManager) Exists(server string) bool {
	if server == "" {
		return false
	}

	// Try to get from keyring
	_, err := keyring.Get(ServiceName, server)
	return err == nil
}
