package smb

import (
	"testing"

	"go.uber.org/zap"
)

func TestCredentialManager_SaveValidation(t *testing.T) {
	cm := NewCredentialManager(zap.NewNop())

	tests := []struct {
		name      string
		creds     *Credentials
		expectErr bool
	}{
		{
			name:      "nil credentials",
			creds:     nil,
			expectErr: true,
		},
		{
			name: "empty server",
			creds: &Credentials{
				Server:   "",
				Username: "user",
				Password: "pass",
			},
			expectErr: true,
		},
		{
			name: "empty username",
			creds: &Credentials{
				Server:   "test-server",
				Username: "",
				Password: "pass",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cm.Save(tt.creds)
			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCredentialManager_LoadValidation(t *testing.T) {
	cm := NewCredentialManager(zap.NewNop())

	tests := []struct {
		name      string
		server    string
		expectErr bool
	}{
		{
			name:      "empty server",
			server:    "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cm.Load(tt.server)
			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
			}
		})
	}
}

func TestCredentialManager_DeleteValidation(t *testing.T) {
	cm := NewCredentialManager(zap.NewNop())

	tests := []struct {
		name      string
		server    string
		expectErr bool
	}{
		{
			name:      "empty server",
			server:    "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cm.Delete(tt.server)
			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
			}
		})
	}
}

func TestCredentialManager_Exists(t *testing.T) {
	cm := NewCredentialManager(zap.NewNop())

	// Test with empty parameters
	if cm.Exists("") {
		t.Error("Exists should return false for empty server")
	}

	// Test with non-existent credentials
	if cm.Exists("non-existent-server-xyz123") {
		t.Error("Exists should return false for non-existent credentials")
	}
}

func TestCredentialManager_SaveLoadDeleteCycle(t *testing.T) {
	// Skip this test in CI or if keyring is not available
	// This test requires a working system keyring
	if testing.Short() {
		t.Skip("skipping keyring integration test in short mode")
	}

	cm := NewCredentialManager(zap.NewNop())

	// Use unique test credentials
	testCreds := &Credentials{
		Server:   "test-keyring-server",
		Port:     445,
		Username: "testuser",
		Password: "testpass",
		Domain:   "TESTDOMAIN",
	}

	// Clean up any existing test credentials
	_ = cm.Delete(testCreds.Server)

	// Test Save
	if err := cm.Save(testCreds); err != nil {
		t.Fatalf("failed to save credentials: %v", err)
	}

	// Test Exists
	if !cm.Exists(testCreds.Server) {
		t.Error("Exists should return true after Save")
	}

	// Test Load
	loadedCreds, err := cm.Load(testCreds.Server)
	if err != nil {
		t.Fatalf("failed to load credentials: %v", err)
	}

	// Verify loaded credentials match
	if loadedCreds.Server != testCreds.Server {
		t.Errorf("server: expected %s, got %s", testCreds.Server, loadedCreds.Server)
	}
	if loadedCreds.Port != testCreds.Port {
		t.Errorf("port: expected %d, got %d", testCreds.Port, loadedCreds.Port)
	}
	if loadedCreds.Username != testCreds.Username {
		t.Errorf("username: expected %s, got %s", testCreds.Username, loadedCreds.Username)
	}
	if loadedCreds.Password != testCreds.Password {
		t.Errorf("password: expected %s, got %s", testCreds.Password, loadedCreds.Password)
	}
	if loadedCreds.Domain != testCreds.Domain {
		t.Errorf("domain: expected %s, got %s", testCreds.Domain, loadedCreds.Domain)
	}

	// Test Delete
	if err := cm.Delete(testCreds.Server); err != nil {
		t.Fatalf("failed to delete credentials: %v", err)
	}

	// Test Exists after delete
	if cm.Exists(testCreds.Server) {
		t.Error("Exists should return false after Delete")
	}

	// Test Load after delete (should fail)
	_, err = cm.Load(testCreds.Server)
	if err == nil {
		t.Error("Load should fail after Delete")
	}
}
