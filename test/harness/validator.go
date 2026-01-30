package harness

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hirochachacha/go-smb2"
)

// Expectation defines what we expect after a sync.
type Expectation struct {
	Type     string `json:"type"`     // file_exists, file_not_exists, content_equals, files_match
	Side     string `json:"side"`     // local, remote, both
	Path     string `json:"path"`     // file path
	Content  string `json:"content"`  // expected content (for content_equals)
	Expected bool   `json:"expected"` // expected result (for exists checks)
}

// Validation represents a validation result.
type Validation struct {
	Check    string `json:"check"`
	Path     string `json:"path,omitempty"`
	Side     string `json:"side,omitempty"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Passed   bool   `json:"passed"`
}

// Validator checks test expectations.
type Validator struct {
	config         *Config
	smbShare       *smb2.Share
	useMappedDrive bool
}

// NewValidator creates a new validator.
func NewValidator(cfg *Config, share *smb2.Share, useMappedDrive bool) *Validator {
	return &Validator{
		config:         cfg,
		smbShare:       share,
		useMappedDrive: useMappedDrive,
	}
}

// ValidateAll validates all expectations and returns results.
func (v *Validator) ValidateAll(job string, expectations []Expectation) ([]Validation, error) {
	var validations []Validation
	var firstError error

	for _, exp := range expectations {
		results, err := v.validate(job, exp)
		validations = append(validations, results...)
		if err != nil && firstError == nil {
			firstError = err
		}
	}

	return validations, firstError
}

// validate checks a single expectation.
func (v *Validator) validate(job string, exp Expectation) ([]Validation, error) {
	switch exp.Side {
	case "local":
		return v.validateSide(job, exp, "local")
	case "remote":
		return v.validateSide(job, exp, "remote")
	case "both":
		localResults, localErr := v.validateSide(job, exp, "local")
		remoteResults, remoteErr := v.validateSide(job, exp, "remote")
		results := append(localResults, remoteResults...)
		if localErr != nil {
			return results, localErr
		}
		return results, remoteErr
	default:
		return nil, fmt.Errorf("unknown side: %s", exp.Side)
	}
}

// validateSide checks an expectation on one side.
func (v *Validator) validateSide(job string, exp Expectation, side string) ([]Validation, error) {
	switch exp.Type {
	case "file_exists":
		return v.validateFileExists(job, exp.Path, side, exp.Expected)
	case "file_not_exists":
		return v.validateFileExists(job, exp.Path, side, false)
	case "content_equals":
		return v.validateContentEquals(job, exp.Path, side, exp.Content)
	case "files_match":
		return v.validateFilesMatch(job, exp.Path)
	default:
		return nil, fmt.Errorf("unknown expectation type: %s", exp.Type)
	}
}

// validateFileExists checks if a file exists.
func (v *Validator) validateFileExists(job, path, side string, expected bool) ([]Validation, error) {
	var exists bool
	var err error

	if side == "local" {
		fullPath := filepath.Join(v.config.LocalPath(job), path)
		_, err = os.Stat(fullPath)
		exists = err == nil
	} else {
		fullPath := filepath.Join(v.config.RemotePathForJob(job), path)
		if v.useMappedDrive {
			_, err = os.Stat(fullPath)
		} else {
			fullPath = filepath.ToSlash(fullPath)
			_, err = v.smbShare.Stat(fullPath)
		}
		exists = err == nil
	}

	validation := Validation{
		Check:    "file_exists",
		Path:     path,
		Side:     side,
		Expected: fmt.Sprintf("%v", expected),
		Actual:   fmt.Sprintf("%v", exists),
		Passed:   exists == expected,
	}

	var resultErr error
	if !validation.Passed {
		if expected {
			resultErr = fmt.Errorf("fichier %s devrait exister sur %s mais n'existe pas", path, side)
		} else {
			resultErr = fmt.Errorf("fichier %s ne devrait pas exister sur %s mais existe", path, side)
		}
	}

	return []Validation{validation}, resultErr
}

// validateContentEquals checks if file content matches expected.
func (v *Validator) validateContentEquals(job, path, side, expectedContent string) ([]Validation, error) {
	var actualContent string
	var err error

	if side == "local" {
		fullPath := filepath.Join(v.config.LocalPath(job), path)
		data, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			return []Validation{{
				Check:    "content_equals",
				Path:     path,
				Side:     side,
				Expected: truncate(expectedContent, 50),
				Actual:   fmt.Sprintf("error: %v", readErr),
				Passed:   false,
			}}, readErr
		}
		actualContent = string(data)
	} else {
		fullPath := filepath.Join(v.config.RemotePathForJob(job), path)

		// Use local filesystem for mapped drive
		if v.useMappedDrive {
			data, readErr := os.ReadFile(fullPath)
			if readErr != nil {
				return []Validation{{
					Check:    "content_equals",
					Path:     path,
					Side:     side,
					Expected: truncate(expectedContent, 50),
					Actual:   fmt.Sprintf("error: %v", readErr),
					Passed:   false,
				}}, readErr
			}
			actualContent = string(data)
		} else {
			fullPath = filepath.ToSlash(fullPath)
			f, openErr := v.smbShare.Open(fullPath)
			if openErr != nil {
				return []Validation{{
					Check:    "content_equals",
					Path:     path,
					Side:     side,
					Expected: truncate(expectedContent, 50),
					Actual:   fmt.Sprintf("error: %v", openErr),
					Passed:   false,
				}}, openErr
			}
			defer f.Close()

			info, _ := f.Stat()
			data := make([]byte, info.Size())
			_, err = f.Read(data)
			if err != nil {
				return []Validation{{
					Check:    "content_equals",
					Path:     path,
					Side:     side,
					Expected: truncate(expectedContent, 50),
					Actual:   fmt.Sprintf("error: %v", err),
					Passed:   false,
				}}, err
			}
			actualContent = string(data)
		}
	}

	passed := actualContent == expectedContent
	validation := Validation{
		Check:    "content_equals",
		Path:     path,
		Side:     side,
		Expected: truncate(expectedContent, 50),
		Actual:   truncate(actualContent, 50),
		Passed:   passed,
	}

	var resultErr error
	if !passed {
		resultErr = fmt.Errorf("contenu de %s sur %s ne correspond pas", path, side)
	}

	return []Validation{validation}, resultErr
}

// validateFilesMatch checks if local and remote files have same content.
func (v *Validator) validateFilesMatch(job, path string) ([]Validation, error) {
	// Read local
	localPath := filepath.Join(v.config.LocalPath(job), path)
	localData, err := os.ReadFile(localPath)
	if err != nil {
		return []Validation{{
			Check:    "files_match",
			Path:     path,
			Expected: "local == remote",
			Actual:   fmt.Sprintf("local read error: %v", err),
			Passed:   false,
		}}, err
	}

	// Read remote
	remotePath := filepath.Join(v.config.RemotePathForJob(job), path)
	var remoteData []byte

	if v.useMappedDrive {
		// Use local filesystem for mapped drive
		remoteData, err = os.ReadFile(remotePath)
		if err != nil {
			return []Validation{{
				Check:    "files_match",
				Path:     path,
				Expected: "local == remote",
				Actual:   fmt.Sprintf("remote read error: %v", err),
				Passed:   false,
			}}, err
		}
	} else {
		remotePath = filepath.ToSlash(remotePath)
		f, err := v.smbShare.Open(remotePath)
		if err != nil {
			return []Validation{{
				Check:    "files_match",
				Path:     path,
				Expected: "local == remote",
				Actual:   fmt.Sprintf("remote read error: %v", err),
				Passed:   false,
			}}, err
		}
		defer f.Close()

		info, _ := f.Stat()
		remoteData = make([]byte, info.Size())
		_, err = f.Read(remoteData)
		if err != nil {
			return []Validation{{
				Check:    "files_match",
				Path:     path,
				Expected: "local == remote",
				Actual:   fmt.Sprintf("remote read error: %v", err),
				Passed:   false,
			}}, err
		}
	}

	passed := string(localData) == string(remoteData)
	validation := Validation{
		Check:    "files_match",
		Path:     path,
		Expected: "local == remote",
		Actual:   fmt.Sprintf("match=%v (local=%d bytes, remote=%d bytes)", passed, len(localData), len(remoteData)),
		Passed:   passed,
	}

	var resultErr error
	if !passed {
		resultErr = fmt.Errorf("fichiers %s local et remote ne correspondent pas", path)
	}

	return []Validation{validation}, resultErr
}

// ListLocalFiles returns all files in a local directory.
func (v *Validator) ListLocalFiles(job string) ([]string, error) {
	var files []string
	basePath := v.config.LocalPath(job)

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, _ := filepath.Rel(basePath, path)
			files = append(files, relPath)
		}
		return nil
	})

	return files, err
}

// ListRemoteFiles returns all files in a remote directory.
func (v *Validator) ListRemoteFiles(job string) ([]string, error) {
	var files []string
	basePath := v.config.RemotePathForJob(job)

	// Use local filesystem for mapped drive
	if v.useMappedDrive {
		return v.ListLocalFiles(job)
	}

	var walkDir func(string) error
	walkDir = func(dir string) error {
		entries, err := v.smbShare.ReadDir(dir)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			fullPath := filepath.ToSlash(filepath.Join(dir, entry.Name()))
			if entry.IsDir() {
				if err := walkDir(fullPath); err != nil {
					return err
				}
			} else {
				relPath, _ := filepath.Rel(basePath, fullPath)
				files = append(files, filepath.ToSlash(relPath))
			}
		}
		return nil
	}

	err := walkDir(basePath)
	return files, err
}

// truncate shortens a string for display.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
