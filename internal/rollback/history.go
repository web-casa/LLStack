package rollback

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/web-casa/llstack/internal/core/apply"
)

// Entry stores rollback metadata for a completed operation.
type Entry struct {
	ID         string         `json:"id"`
	Action     string         `json:"action"`
	Resource   string         `json:"resource"`
	Backend    string         `json:"backend,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
	Changes    []apply.Change `json:"changes"`
	RolledBack bool           `json:"rolled_back"`
}

// StoredEntry includes the on-disk path of the history record.
type StoredEntry struct {
	Entry
	Path string
}

// Save persists a history entry.
func Save(historyDir string, entry Entry) (string, error) {
	if err := os.MkdirAll(historyDir, 0o755); err != nil {
		return "", err
	}

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	filename := entry.Timestamp.Format("20060102T150405.000000000Z07") + "-" + entry.ID + ".json"
	path := filepath.Join(historyDir, filename)
	raw, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return "", err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// LoadLatestPending returns the latest non-rolled-back history entry.
func LoadLatestPending(historyDir string) (StoredEntry, error) {
	entries, err := List(historyDir, 0)
	if err != nil {
		return StoredEntry{}, err
	}

	for _, entry := range entries {
		if entry.RolledBack {
			continue
		}
		return entry, nil
	}

	return StoredEntry{}, errors.New("no rollback entry available")
}

// List returns recent rollback history entries in descending timestamp/file order.
// If limit <= 0, all entries are returned.
func List(historyDir string, limit int) ([]StoredEntry, error) {
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, filepath.Join(historyDir, entry.Name()))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}

	out := make([]StoredEntry, 0, len(files))
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var entry Entry
		if err := json.Unmarshal(raw, &entry); err != nil {
			return nil, err
		}
		out = append(out, StoredEntry{Entry: entry, Path: path})
	}
	return out, nil
}

// MarkRolledBack updates the source history entry after a successful rollback.
func MarkRolledBack(stored StoredEntry) error {
	stored.RolledBack = true
	raw, err := json.MarshalIndent(stored.Entry, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(stored.Path, raw, 0o644)
}

// Get resolves a rollback history entry by ID or history filename.
func Get(historyDir string, key string) (StoredEntry, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return StoredEntry{}, errors.New("history key is required")
	}

	entries, err := List(historyDir, 0)
	if err != nil {
		return StoredEntry{}, err
	}
	for _, entry := range entries {
		if entry.ID == key || filepath.Base(entry.Path) == key {
			return entry, nil
		}
	}
	return StoredEntry{}, errors.New("rollback history entry not found")
}
