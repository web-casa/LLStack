package apply

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Change describes a filesystem mutation that can be rolled back.
type Change struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	BackupPath string `json:"backup_path,omitempty"`
}

// FileApplier applies file and directory changes while recording rollback data.
type FileApplier struct {
	BackupsDir string
}

// NewFileApplier constructs a file applier.
func NewFileApplier(backupsDir string) FileApplier {
	return FileApplier{BackupsDir: backupsDir}
}

// EnsureDir creates a directory if needed and records it when newly created.
func (a FileApplier) EnsureDir(path string, mode fs.FileMode) (*Change, error) {
	if _, err := os.Stat(path); err == nil {
		return nil, nil
	}

	if err := os.MkdirAll(path, mode); err != nil {
		return nil, err
	}

	return &Change{Path: path, Kind: "dir_created"}, nil
}

// WriteFile writes a file, backing up the prior version when it exists.
func (a FileApplier) WriteFile(path string, content []byte, mode fs.FileMode) (Change, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Change{}, err
	}

	change := Change{Path: path, Kind: "file_created"}
	if _, err := os.Stat(path); err == nil {
		backupPath, backupErr := a.backup(path)
		if backupErr != nil {
			return Change{}, backupErr
		}
		change.Kind = "file_updated"
		change.BackupPath = backupPath
	}

	if err := os.WriteFile(path, content, mode); err != nil {
		return Change{}, err
	}

	return change, nil
}

// WriteJSON writes a JSON document through the same backup-aware path.
func (a FileApplier) WriteJSON(path string, value any, mode fs.FileMode) (Change, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return Change{}, err
	}
	raw = append(raw, '\n')
	return a.WriteFile(path, raw, mode)
}

// DeleteFile removes a file and keeps a backup for rollback.
func (a FileApplier) DeleteFile(path string) (Change, error) {
	if _, err := os.Stat(path); err != nil {
		return Change{}, err
	}

	backupPath, err := a.backup(path)
	if err != nil {
		return Change{}, err
	}
	if err := os.Remove(path); err != nil {
		return Change{}, err
	}

	return Change{
		Path:       path,
		Kind:       "file_deleted",
		BackupPath: backupPath,
	}, nil
}

// Rollback reverts a recorded change.
func (a FileApplier) Rollback(change Change) error {
	switch change.Kind {
	case "file_created":
		if err := os.Remove(change.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	case "file_updated", "file_deleted":
		if change.BackupPath == "" {
			return fmt.Errorf("missing backup for %s", change.Path)
		}
		content, err := os.ReadFile(change.BackupPath)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(change.Path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(change.Path, content, 0o644)
	case "dir_created":
		if err := os.Remove(change.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	case "dir_deleted":
		if change.BackupPath == "" {
			return fmt.Errorf("missing backup for %s", change.Path)
		}
		if err := os.MkdirAll(filepath.Dir(change.Path), 0o755); err != nil {
			return err
		}
		if err := os.Rename(change.BackupPath, change.Path); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported change kind %q", change.Kind)
	}
}

func (a FileApplier) backup(path string) (string, error) {
	if err := os.MkdirAll(a.BackupsDir, 0o755); err != nil {
		return "", err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	filename := fmt.Sprintf("%d-%s.bak", time.Now().UTC().UnixNano(), filepath.Base(path))
	backupPath := filepath.Join(a.BackupsDir, filename)
	if err := os.WriteFile(backupPath, raw, 0o600); err != nil {
		return "", err
	}

	return backupPath, nil
}
