package smb

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewSMBClient(t *testing.T) {
	tests := []struct {
		name      string
		config    *ClientConfig
		expectErr bool
	}{
		{
			name: "valid config",
			config: &ClientConfig{
				Server:   "192.168.1.100",
				Share:    "documents",
				Username: "user",
				Password: "pass",
			},
			expectErr: false,
		},
		{
			name:      "nil config",
			config:    nil,
			expectErr: true,
		},
		{
			name: "empty server",
			config: &ClientConfig{
				Server:   "",
				Share:    "documents",
				Username: "user",
				Password: "pass",
			},
			expectErr: true,
		},
		{
			name: "empty share",
			config: &ClientConfig{
				Server:   "192.168.1.100",
				Share:    "",
				Username: "user",
				Password: "pass",
			},
			expectErr: true,
		},
		{
			name: "empty username",
			config: &ClientConfig{
				Server:   "192.168.1.100",
				Share:    "documents",
				Username: "",
				Password: "pass",
			},
			expectErr: true,
		},
		{
			name: "default port",
			config: &ClientConfig{
				Server:   "192.168.1.100",
				Share:    "documents",
				Username: "user",
				Password: "pass",
				Port:     0, // Should default to 445
			},
			expectErr: false,
		},
		{
			name: "custom port",
			config: &ClientConfig{
				Server:   "192.168.1.100",
				Share:    "documents",
				Username: "user",
				Password: "pass",
				Port:     4445,
			},
			expectErr: false,
		},
		{
			name: "with domain",
			config: &ClientConfig{
				Server:   "192.168.1.100",
				Share:    "documents",
				Username: "user",
				Password: "pass",
				Domain:   "WORKGROUP",
			},
			expectErr: false,
		},
	}

	logger := zap.NewNop()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewSMBClient(tt.config, logger)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Error("expected client but got nil")
				return
			}

			// Verify fields
			if tt.config.Server != "" && client.server != tt.config.Server {
				t.Errorf("server: expected %s, got %s", tt.config.Server, client.server)
			}
			if tt.config.Share != "" && client.share != tt.config.Share {
				t.Errorf("share: expected %s, got %s", tt.config.Share, client.share)
			}
			if tt.config.Username != "" && client.username != tt.config.Username {
				t.Errorf("username: expected %s, got %s", tt.config.Username, client.username)
			}

			// Verify port default
			if tt.config.Port == 0 {
				if client.port != 445 {
					t.Errorf("port: expected default 445, got %d", client.port)
				}
			} else {
				if client.port != tt.config.Port {
					t.Errorf("port: expected %d, got %d", tt.config.Port, client.port)
				}
			}

			// Verify initial state
			if client.IsConnected() {
				t.Error("new client should not be connected")
			}
		})
	}
}

func TestSMBClient_BasicState(t *testing.T) {
	config := &ClientConfig{
		Server:   "test-server",
		Share:    "test-share",
		Username: "user",
		Password: "pass",
	}

	client, err := NewSMBClient(config, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Test initial state
	if client.IsConnected() {
		t.Error("new client should not be connected")
	}

	// Test getters
	if client.GetServer() != "test-server" {
		t.Errorf("GetServer: expected test-server, got %s", client.GetServer())
	}
	if client.GetShare() != "test-share" {
		t.Errorf("GetShare: expected test-share, got %s", client.GetShare())
	}

	// Test disconnect on non-connected client (should not error)
	if err := client.Disconnect(); err != nil {
		t.Errorf("Disconnect on non-connected client should not error: %v", err)
	}
}

func TestSMBClient_DownloadNotConnected(t *testing.T) {
	config := &ClientConfig{
		Server:   "test-server",
		Share:    "test-share",
		Username: "user",
		Password: "pass",
	}

	client, err := NewSMBClient(config, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Try to download without connecting
	err = client.Download("remote.txt", "local.txt")
	if err == nil {
		t.Error("expected error when downloading without connection")
	}
	if err != nil && err.Error() != "not connected to SMB server" {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSMBClient_UploadNotConnected(t *testing.T) {
	config := &ClientConfig{
		Server:   "test-server",
		Share:    "test-share",
		Username: "user",
		Password: "pass",
	}

	client, err := NewSMBClient(config, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Try to upload without connecting
	err = client.Upload("local.txt", "remote.txt")
	if err == nil {
		t.Error("expected error when uploading without connection")
	}
	if err != nil && err.Error() != "not connected to SMB server" {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSMBClient_ListRemoteNotConnected(t *testing.T) {
	config := &ClientConfig{
		Server:   "test-server",
		Share:    "test-share",
		Username: "user",
		Password: "pass",
	}

	client, err := NewSMBClient(config, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Try to list without connecting
	_, err = client.ListRemote(".")
	if err == nil {
		t.Error("expected error when listing without connection")
	}
	if err != nil && err.Error() != "not connected to SMB server" {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSMBClient_GetMetadataNotConnected(t *testing.T) {
	config := &ClientConfig{
		Server:   "test-server",
		Share:    "test-share",
		Username: "user",
		Password: "pass",
	}

	client, err := NewSMBClient(config, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Try to get metadata without connecting
	_, err = client.GetMetadata("file.txt")
	if err == nil {
		t.Error("expected error when getting metadata without connection")
	}
	if err != nil && err.Error() != "not connected to SMB server" {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSMBClient_DeleteNotConnected(t *testing.T) {
	config := &ClientConfig{
		Server:   "test-server",
		Share:    "test-share",
		Username: "user",
		Password: "pass",
	}

	client, err := NewSMBClient(config, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Try to delete without connecting
	err = client.Delete("file.txt")
	if err == nil {
		t.Error("expected error when deleting without connection")
	}
	if err != nil && err.Error() != "not connected to SMB server" {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}
