package cron

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Job represents a managed cron job for a site.
type Job struct {
	ID        string    `json:"id"`
	Site      string    `json:"site"`
	User      string    `json:"user"`
	Schedule  string    `json:"schedule"`
	Command   string    `json:"command"`
	Preset    string    `json:"preset,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Manifest stores all cron jobs for a site.
type Manifest struct {
	Site string `json:"site"`
	Jobs []Job  `json:"jobs"`
}

// Manager manages per-site cron jobs.
type Manager struct {
	cronDir     string // /etc/cron.d
	manifestDir string // /etc/llstack/cron
}

// NewManager creates a cron manager.
func NewManager(cronDir, manifestDir string) Manager {
	return Manager{cronDir: cronDir, manifestDir: manifestDir}
}

// Add creates a cron job for a site.
func (m Manager) Add(site, user, schedule, command, preset string) (Job, error) {
	if strings.TrimSpace(schedule) == "" || strings.TrimSpace(command) == "" {
		return Job{}, fmt.Errorf("schedule and command are required")
	}

	job := Job{
		ID:        generateJobID(site, command),
		Site:      site,
		User:      user,
		Schedule:  schedule,
		Command:   command,
		Preset:    preset,
		CreatedAt: time.Now().UTC(),
	}

	manifest := m.loadManifest(site)
	manifest.Jobs = append(manifest.Jobs, job)

	if err := m.saveManifest(manifest); err != nil {
		return Job{}, err
	}
	if err := m.writeCronFile(manifest); err != nil {
		return Job{}, err
	}

	return job, nil
}

// List returns all cron jobs for a site (or all sites if site is empty).
func (m Manager) List(site string) ([]Job, error) {
	if site != "" {
		manifest := m.loadManifest(site)
		return manifest.Jobs, nil
	}
	entries, err := os.ReadDir(m.manifestDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var all []Job
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(m.manifestDir, entry.Name()))
		if err != nil {
			continue
		}
		var manifest Manifest
		if json.Unmarshal(raw, &manifest) == nil {
			all = append(all, manifest.Jobs...)
		}
	}
	return all, nil
}

// Remove deletes a cron job by ID.
func (m Manager) Remove(site, jobID string) error {
	manifest := m.loadManifest(site)
	var remaining []Job
	found := false
	for _, job := range manifest.Jobs {
		if job.ID == jobID {
			found = true
			continue
		}
		remaining = append(remaining, job)
	}
	if !found {
		return fmt.Errorf("cron job %q not found for site %q", jobID, site)
	}
	manifest.Jobs = remaining

	if err := m.saveManifest(manifest); err != nil {
		return err
	}
	if len(remaining) == 0 {
		os.Remove(m.cronFilePath(site))
	} else {
		return m.writeCronFile(manifest)
	}
	return nil
}

// Preset definitions.
const (
	PresetWPCron          = "wp-cron"
	PresetLaravelSchedule = "laravel-scheduler"
)

// AddPreset adds a pre-configured cron job.
func (m Manager) AddPreset(site, user, docroot, preset string) (Job, error) {
	switch preset {
	case PresetWPCron:
		// Auto-disable WordPress internal wp-cron when using system cron
		// Best-effort: don't fail cron creation if wp-config.php is missing/unmodifiable
		_ = DisableWPInternalCron(docroot)
		safeDocroot := "'" + strings.ReplaceAll(docroot, "'", "'\"'\"'") + "'"
		return m.Add(site, user, "*/5 * * * *",
			fmt.Sprintf("cd %s && php wp-cron.php > /dev/null 2>&1", safeDocroot), preset)
	case PresetLaravelSchedule:
		safeDocroot := "'" + strings.ReplaceAll(docroot, "'", "'\"'\"'") + "'"
		return m.Add(site, user, "* * * * *",
			fmt.Sprintf("cd %s && php artisan schedule:run >> /dev/null 2>&1", safeDocroot), preset)
	default:
		return Job{}, fmt.Errorf("unknown preset %q", preset)
	}
}

func (m Manager) loadManifest(site string) Manifest {
	path := filepath.Join(m.manifestDir, site+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{Site: site}
	}
	var manifest Manifest
	if json.Unmarshal(raw, &manifest) != nil {
		return Manifest{Site: site}
	}
	return manifest
}

func (m Manager) saveManifest(manifest Manifest) error {
	if err := os.MkdirAll(m.manifestDir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.manifestDir, manifest.Site+".json"), raw, 0o644)
}

func (m Manager) writeCronFile(manifest Manifest) error {
	if err := os.MkdirAll(m.cronDir, 0o755); err != nil {
		return err
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("# Managed by LLStack for site %s", manifest.Site))
	lines = append(lines, "SHELL=/bin/bash")
	for _, job := range manifest.Jobs {
		user := job.User
		if user == "" {
			user = "root"
		}
		lines = append(lines, fmt.Sprintf("%s %s %s", job.Schedule, user, job.Command))
	}
	lines = append(lines, "")
	return os.WriteFile(m.cronFilePath(manifest.Site), []byte(strings.Join(lines, "\n")), 0o644)
}

func (m Manager) cronFilePath(site string) string {
	safe := strings.NewReplacer(".", "-", "/", "-").Replace(site)
	return filepath.Join(m.cronDir, "llstack-"+safe)
}

// DisableWPInternalCron writes DISABLE_WP_CRON to wp-config.php so system cron is used instead.
func DisableWPInternalCron(docroot string) error {
	wpConfig := filepath.Join(docroot, "wp-config.php")
	raw, err := os.ReadFile(wpConfig)
	if err != nil {
		return fmt.Errorf("read wp-config.php: %w", err)
	}
	content := string(raw)
	if strings.Contains(content, "DISABLE_WP_CRON") {
		return nil // already set
	}
	// Insert before "That's all" comment or at end of opening PHP block
	marker := "/* That's all"
	if idx := strings.Index(content, marker); idx > 0 {
		content = content[:idx] + "define('DISABLE_WP_CRON', true);\n" + content[idx:]
	} else {
		// Fallback: insert after <?php
		content = strings.Replace(content, "<?php", "<?php\ndefine('DISABLE_WP_CRON', true);", 1)
	}
	return os.WriteFile(wpConfig, []byte(content), 0o644)
}

// HtaccessWatchTimerUnit generates a systemd timer for .htaccess monitoring.
func HtaccessWatchTimerUnit(intervalSec int) string {
	return fmt.Sprintf(`[Unit]
Description=LLStack OLS .htaccess Watch Timer

[Timer]
OnBootSec=60
OnUnitActiveSec=%ds
AccuracySec=5s

[Install]
WantedBy=timers.target
`, intervalSec)
}

// HtaccessWatchServiceUnit generates the systemd service unit.
func HtaccessWatchServiceUnit(llstackBin string) string {
	return fmt.Sprintf(`[Unit]
Description=LLStack OLS .htaccess Watch

[Service]
Type=oneshot
ExecStart=/bin/bash -c 'for site in $(%s site:list --json 2>/dev/null | grep -o "\"name\":\"[^\"]*\"" | cut -d\" -f4); do %s site:reload "$site" 2>/dev/null; done'
`, llstackBin, llstackBin)
}

func generateJobID(site, command string) string {
	sum := sha256.Sum256([]byte(site + ":" + command + ":" + time.Now().UTC().String()))
	return fmt.Sprintf("%x", sum[:6]) // 12 hex chars, collision-safe for practical use
}
