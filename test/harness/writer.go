package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hirochachacha/go-smb2"
)

// Action represents a test action to execute.
type Action struct {
	Type    string        `json:"type"`    // create, modify, delete, rename, wait_user
	Side    string        `json:"side"`    // local, remote, both
	Path    string        `json:"path"`    // relative path within job directory
	Content string        `json:"content"` // file content (for create/modify)
	NewPath string        `json:"new_path,omitempty"` // for rename
	Delay   time.Duration `json:"delay,omitempty"`    // delay after action
	Message string        `json:"message,omitempty"`  // message for wait_user
}

// Writer handles file operations for tests.
type Writer struct {
	config         *Config
	smbShare       *smb2.Share
	useMappedDrive bool
}

// NewWriter creates a new writer.
func NewWriter(cfg *Config, share *smb2.Share, useMappedDrive bool) *Writer {
	return &Writer{
		config:         cfg,
		smbShare:       share,
		useMappedDrive: useMappedDrive,
	}
}

// Execute performs the action.
func (w *Writer) Execute(job string, action Action) error {
	switch action.Side {
	case "local":
		return w.executeLocal(job, action)
	case "remote":
		// Use local filesystem for mapped drive mode
		if w.useMappedDrive {
			return w.executeRemoteAsLocal(job, action)
		}
		return w.executeRemote(job, action)
	case "both":
		if err := w.executeLocal(job, action); err != nil {
			return fmt.Errorf("local: %w", err)
		}
		if w.useMappedDrive {
			return w.executeRemoteAsLocal(job, action)
		}
		return w.executeRemote(job, action)
	default:
		return fmt.Errorf("unknown side: %s", action.Side)
	}
}

// executeRemoteAsLocal performs action on remote via mapped drive (local filesystem).
func (w *Writer) executeRemoteAsLocal(job string, action Action) error {
	basePath := w.config.RemotePathForJob(job)
	fullPath := filepath.Join(basePath, action.Path)

	switch action.Type {
	case "create", "modify":
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return err
		}
		return os.WriteFile(fullPath, []byte(action.Content), 0644)

	case "delete":
		return os.RemoveAll(fullPath)

	case "rename":
		newPath := filepath.Join(basePath, action.NewPath)
		if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
			return err
		}
		return os.Rename(fullPath, newPath)

	case "mkdir":
		return os.MkdirAll(fullPath, 0755)

	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// executeLocal performs action on local filesystem.
func (w *Writer) executeLocal(job string, action Action) error {
	basePath := w.config.LocalPath(job)
	fullPath := filepath.Join(basePath, action.Path)

	switch action.Type {
	case "create", "modify":
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return err
		}
		return os.WriteFile(fullPath, []byte(action.Content), 0644)

	case "delete":
		return os.RemoveAll(fullPath)

	case "rename":
		newPath := filepath.Join(basePath, action.NewPath)
		if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
			return err
		}
		return os.Rename(fullPath, newPath)

	case "mkdir":
		return os.MkdirAll(fullPath, 0755)

	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// executeRemote performs action on SMB share.
func (w *Writer) executeRemote(job string, action Action) error {
	basePath := w.config.RemotePathForJob(job)
	fullPath := filepath.ToSlash(filepath.Join(basePath, action.Path))

	switch action.Type {
	case "create", "modify":
		// Ensure parent directory exists
		parentDir := filepath.ToSlash(filepath.Dir(fullPath))
		if err := w.smbShare.MkdirAll(parentDir, 0755); err != nil {
			// Ignore if already exists
		}

		f, err := w.smbShare.Create(fullPath)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.Write([]byte(action.Content))
		return err

	case "delete":
		// Check if it's a directory
		info, err := w.smbShare.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil // Already deleted
			}
			return err
		}
		if info.IsDir() {
			return w.removeRemoteDir(fullPath)
		}
		return w.smbShare.Remove(fullPath)

	case "rename":
		newPath := filepath.ToSlash(filepath.Join(basePath, action.NewPath))
		parentDir := filepath.ToSlash(filepath.Dir(newPath))
		if err := w.smbShare.MkdirAll(parentDir, 0755); err != nil {
			// Ignore if already exists
		}
		return w.smbShare.Rename(fullPath, newPath)

	case "mkdir":
		return w.smbShare.MkdirAll(fullPath, 0755)

	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// removeRemoteDir recursively removes a remote directory.
func (w *Writer) removeRemoteDir(path string) error {
	entries, err := w.smbShare.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fullPath := filepath.ToSlash(filepath.Join(path, entry.Name()))
		if entry.IsDir() {
			if err := w.removeRemoteDir(fullPath); err != nil {
				return err
			}
		} else {
			if err := w.smbShare.Remove(fullPath); err != nil {
				return err
			}
		}
	}

	return w.smbShare.Remove(path)
}

// CreateLocalFile creates a file locally with specified content.
func (w *Writer) CreateLocalFile(job, path, content string) error {
	return w.executeLocal(job, Action{Type: "create", Path: path, Content: content})
}

// CreateRemoteFile creates a file on the SMB share with specified content.
func (w *Writer) CreateRemoteFile(job, path, content string) error {
	return w.executeRemote(job, Action{Type: "create", Path: path, Content: content})
}

// DeleteLocalFile deletes a local file.
func (w *Writer) DeleteLocalFile(job, path string) error {
	return w.executeLocal(job, Action{Type: "delete", Path: path})
}

// DeleteRemoteFile deletes a file on the SMB share.
func (w *Writer) DeleteRemoteFile(job, path string) error {
	return w.executeRemote(job, Action{Type: "delete", Path: path})
}

// GenerateContent generates test content of specified size.
func GenerateContent(size int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789\n"
	content := make([]byte, size)
	for i := range content {
		content[i] = charset[i%len(charset)]
	}
	return string(content)
}

// GenerateUniqueContent generates unique content with timestamp.
func GenerateUniqueContent(prefix string) string {
	return fmt.Sprintf("%s - %s - %d", prefix, time.Now().Format(time.RFC3339Nano), time.Now().UnixNano())
}
