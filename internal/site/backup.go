package site

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/web-casa/llstack/internal/core/plan"
	"github.com/web-casa/llstack/internal/system"
)

// BackupOptions controls site backup.
type BackupOptions struct {
	Name      string
	OutputDir string
	IncludeDB bool
	DBDumpCmd string // mysqldump or pg_dumpall path
	DryRun    bool
}

// BackupResult captures the backup outcome.
type BackupResult struct {
	Site       string `json:"site"`
	BackupPath string `json:"backup_path"`
	Size       int64  `json:"size_bytes"`
	Timestamp  string `json:"timestamp"`
}

// Backup creates a complete site backup (files + config + optional DB dump).
func (m Manager) Backup(ctx context.Context, opts BackupOptions) (plan.Plan, BackupResult, error) {
	p := plan.New("site.backup", fmt.Sprintf("Backup site %s", opts.Name))
	p.DryRun = opts.DryRun
	result := BackupResult{Site: opts.Name}

	manifest, err := m.loadManifest(opts.Name)
	if err != nil {
		return p, result, fmt.Errorf("site %q not found", opts.Name)
	}

	timestamp := time.Now().UTC().Format("20060102T150405Z")
	result.Timestamp = timestamp

	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(m.cfg.Paths.BackupsDir, "sites", opts.Name)
	}
	backupName := fmt.Sprintf("%s-%s.tar.gz", opts.Name, timestamp)
	backupPath := filepath.Join(outputDir, backupName)
	result.BackupPath = backupPath

	p.AddOperation(plan.Operation{
		ID: "backup-files", Kind: "site.backup.files", Target: backupPath,
		Details: map[string]string{"docroot": manifest.Site.DocumentRoot},
	})
	if opts.IncludeDB {
		p.AddOperation(plan.Operation{
			ID: "backup-db", Kind: "site.backup.db", Target: backupPath,
		})
	}

	if opts.DryRun {
		return p, result, nil
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return p, result, err
	}

	// Create temp dir for staging
	stageDir := filepath.Join(os.TempDir(), "llstack-backup-"+opts.Name)
	os.MkdirAll(stageDir, 0o755)
	defer os.RemoveAll(stageDir)

	// Copy site files
	cpRes, cpErr := m.exec.Run(ctx, system.Command{
		Name: "cp", Args: []string{"-a", manifest.Site.DocumentRoot, filepath.Join(stageDir, "files")},
	})
	if cpErr != nil || cpRes.ExitCode != 0 {
		return p, result, fmt.Errorf("copy site files failed: %s", cpRes.Stderr)
	}

	// Copy manifest (best-effort, non-fatal for backup)
	manifestSrc := m.manifestPath(opts.Name)
	if raw, readErr := os.ReadFile(manifestSrc); readErr == nil {
		if writeErr := os.WriteFile(filepath.Join(stageDir, "manifest.json"), raw, 0o644); writeErr != nil {
			return p, result, fmt.Errorf("write backup manifest: %w", writeErr)
		}
	}

	// Copy vhost config
	if manifest.VHostPath != "" {
		if raw, readErr := os.ReadFile(manifest.VHostPath); readErr == nil {
			os.WriteFile(filepath.Join(stageDir, "vhost.conf"), raw, 0o644)
		}
	}

	// DB dump (command is operator-provided, validated by CLI flag)
	if opts.IncludeDB && opts.DBDumpCmd != "" {
		dbDumpPath := filepath.Join(stageDir, "database.sql")
		// Use shell but quote output path to prevent path injection
		safeDumpPath := "'" + strings.ReplaceAll(dbDumpPath, "'", "'\"'\"'") + "'"
		dumpRes, dumpErr := m.exec.Run(ctx, system.Command{
			Name: "sh", Args: []string{"-c", opts.DBDumpCmd + " > " + safeDumpPath},
		})
		if dumpErr != nil || dumpRes.ExitCode != 0 {
			return p, result, fmt.Errorf("database dump failed: %s", dumpRes.Stderr)
		}
	}

	// Create archive
	res, _ := m.exec.Run(ctx, system.Command{
		Name: "tar", Args: []string{"-czf", backupPath, "-C", stageDir, "."},
	})
	if res.ExitCode != 0 {
		return p, result, fmt.Errorf("tar failed: %s", res.Stderr)
	}

	if info, err := os.Stat(backupPath); err == nil {
		result.Size = info.Size()
	}

	return p, result, nil
}

// RestoreOptions controls site restoration.
type RestoreOptions struct {
	Name       string
	BackupPath string
	DryRun     bool
}

// Restore restores a site from a backup archive.
func (m Manager) Restore(ctx context.Context, opts RestoreOptions) (plan.Plan, error) {
	p := plan.New("site.restore", fmt.Sprintf("Restore site %s from backup", opts.Name))
	p.DryRun = opts.DryRun

	manifest, err := m.loadManifest(opts.Name)
	if err != nil {
		return p, fmt.Errorf("site %q not found (create site first, then restore)", opts.Name)
	}

	p.AddOperation(plan.Operation{
		ID: "restore-files", Kind: "site.restore", Target: manifest.Site.DocumentRoot,
		Details: map[string]string{"backup": opts.BackupPath},
	})

	if opts.DryRun {
		return p, nil
	}

	// Extract backup to temp
	stageDir := filepath.Join(os.TempDir(), "llstack-restore-"+opts.Name)
	os.MkdirAll(stageDir, 0o755)
	defer os.RemoveAll(stageDir)

	res, _ := m.exec.Run(ctx, system.Command{
		Name: "tar", Args: []string{"xzf", opts.BackupPath, "-C", stageDir},
	})
	if res.ExitCode != 0 {
		return p, fmt.Errorf("extract backup failed: %s", res.Stderr)
	}

	// Restore files (quote paths to handle spaces)
	filesDir := filepath.Join(stageDir, "files")
	if _, err := os.Stat(filesDir); err == nil {
		cpRes, cpErr := m.exec.Run(ctx, system.Command{
			Name: "cp", Args: []string{"-a", filesDir + "/.", manifest.Site.DocumentRoot + "/"},
		})
		if cpErr != nil || cpRes.ExitCode != 0 {
			return p, fmt.Errorf("restore files failed: %s", cpRes.Stderr)
		}
	}

	// Fix permissions
	siteUser := system.SiteUsername(opts.Name)
	if siteUser != "" && system.SiteUserExists(siteUser) {
		m.exec.Run(ctx, system.Command{Name: "chown", Args: []string{"-R", siteUser + ":" + siteUser, manifest.Site.DocumentRoot}})
	}
	m.exec.Run(ctx, system.Command{Name: "restorecon", Args: []string{"-Rv", manifest.Site.DocumentRoot}})

	return p, nil
}

// CleanupSiteBackups removes old backups keeping only the most recent N.
func CleanupSiteBackups(dir string, retain int) (int, error) {
	if retain <= 0 {
		return 0, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	var archives []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tar.gz") {
			archives = append(archives, filepath.Join(dir, e.Name()))
		}
	}
	if len(archives) <= retain {
		return 0, nil
	}
	sort.Strings(archives)
	toDelete := archives[:len(archives)-retain]
	for _, f := range toDelete {
		os.Remove(f)
	}
	return len(toDelete), nil
}
