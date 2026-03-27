package site_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/logging"
	"github.com/web-casa/llstack/internal/site"
	"github.com/web-casa/llstack/internal/system"
)

type fakeExecutor struct {
	commands []system.Command
}

func (f *fakeExecutor) Run(ctx context.Context, cmd system.Command) (system.Result, error) {
	f.commands = append(f.commands, cmd)
	return system.Result{ExitCode: 0}, nil
}

func TestCreateSiteApplyWritesFiles(t *testing.T) {
	cfg := testConfig(t.TempDir())
	exec := &fakeExecutor{}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	siteSpec := model.Site{
		Name: "example.com",
		Domain: model.DomainBinding{
			ServerName: "example.com",
		},
		PHP: model.PHPRuntimeBinding{
			Enabled: true,
		},
	}

	if _, err := manager.Create(context.Background(), site.CreateOptions{Site: siteSpec}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	manifestPath := filepath.Join(cfg.Paths.ManagedSitesDir(), "example.com.json")
	vhostPath := filepath.Join(cfg.Apache.ManagedVhostsDir, "example.com.conf")

	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	if _, err := os.Stat(vhostPath); err != nil {
		t.Fatalf("vhost missing: %v", err)
	}
	// Commands include: groupadd, useradd, usermod, chown, chmod, configtest, reload
	if len(exec.commands) < 2 {
		t.Fatalf("expected at least 2 commands (configtest+reload), got %d", len(exec.commands))
	}
	// Verify configtest and reload are present
	hasConfigtest := false
	hasReload := false
	for _, cmd := range exec.commands {
		if cmd.Name == "apachectl" && len(cmd.Args) > 0 && cmd.Args[0] == "configtest" {
			hasConfigtest = true
		}
		if cmd.Name == "systemctl" && len(cmd.Args) > 0 && cmd.Args[0] == "reload" {
			hasReload = true
		}
	}
	if !hasConfigtest || !hasReload {
		t.Fatalf("expected configtest and reload commands, got %v", exec.commands)
	}
}

func TestDeleteAndRollbackRestoreSite(t *testing.T) {
	cfg := testConfig(t.TempDir())
	exec := &fakeExecutor{}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	siteSpec := model.Site{
		Name: "restore.example.com",
		Domain: model.DomainBinding{
			ServerName: "restore.example.com",
		},
		PHP: model.PHPRuntimeBinding{
			Enabled: true,
		},
	}

	if _, err := manager.Create(context.Background(), site.CreateOptions{Site: siteSpec, SkipReload: true}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	docroot := filepath.Join(cfg.Paths.SitesRootDir, "restore.example.com")
	contentPath := filepath.Join(docroot, "index.php")
	if err := os.WriteFile(contentPath, []byte("<?php echo 'ok';"), 0o644); err != nil {
		t.Fatalf("seed docroot: %v", err)
	}

	if _, err := manager.Delete(context.Background(), site.DeleteOptions{
		Name:       "restore.example.com",
		PurgeRoot:  true,
		SkipReload: true,
	}); err != nil {
		t.Fatalf("delete site: %v", err)
	}

	if _, err := os.Stat(docroot); !os.IsNotExist(err) {
		t.Fatalf("expected docroot to be removed, stat err=%v", err)
	}

	if _, err := manager.RollbackLast(context.Background(), site.RollbackOptions{SkipReload: true}); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if _, err := os.Stat(contentPath); err != nil {
		t.Fatalf("expected docroot restored, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.Paths.ManagedSitesDir(), "restore.example.com.json")); err != nil {
		t.Fatalf("expected manifest restored, stat err=%v", err)
	}
}

func TestCreateOLSSiteApplyWritesAssetsAndParity(t *testing.T) {
	cfg := testConfig(t.TempDir())
	exec := &fakeExecutor{}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	siteSpec := model.Site{
		Name:    "ols.example.com",
		Backend: "ols",
		Domain: model.DomainBinding{
			ServerName: "ols.example.com",
		},
		PHP: model.PHPRuntimeBinding{
			Enabled: true,
			Version: "8.3",
		},
		HeaderRules: []model.HeaderRule{
			{Name: "X-Test", Value: "1", Action: "set"},
		},
	}

	if _, err := manager.Create(context.Background(), site.CreateOptions{Site: siteSpec, SkipReload: true}); err != nil {
		t.Fatalf("create ols site: %v", err)
	}

	paths := []string{
		filepath.Join(cfg.OLS.ManagedVhostsRoot, "ols.example.com", "vhconf.conf"),
		filepath.Join(cfg.OLS.ManagedListenersDir, "ols.example.com.map"),
		filepath.Join(cfg.Paths.ParityReportsDir(), "ols.example.com.ols.json"),
		filepath.Join(cfg.Paths.ManagedSitesDir(), "ols.example.com.json"),
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected asset %s, stat err=%v", path, err)
		}
	}
}

func TestCreateLSWSSiteStoresCapabilities(t *testing.T) {
	cfg := testConfig(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(cfg.LSWS.LicenseSerialFile), 0o755); err != nil {
		t.Fatalf("mkdir serial dir: %v", err)
	}
	if err := os.WriteFile(cfg.LSWS.LicenseSerialFile, []byte("TRIAL-XYZ"), 0o644); err != nil {
		t.Fatalf("write lsws serial: %v", err)
	}
	exec := &fakeExecutor{}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	siteSpec := model.Site{
		Name:    "lsws.example.com",
		Backend: "lsws",
		Domain: model.DomainBinding{
			ServerName: "lsws.example.com",
		},
		PHP: model.PHPRuntimeBinding{
			Enabled: true,
			Version: "8.4",
		},
		LSWS: &model.LSWSOptions{
			CustomDirectives:      []string{"CacheEngine on"},
			RequestedFeatureFlags: []string{"quic", "cache"},
		},
	}

	if _, err := manager.Create(context.Background(), site.CreateOptions{Site: siteSpec, SkipReload: true}); err != nil {
		t.Fatalf("create lsws site: %v", err)
	}

	listed, err := manager.List()
	if err != nil {
		t.Fatalf("list sites: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected one lsws site, got %d", len(listed))
	}
	if listed[0].Capabilities == nil || listed[0].Capabilities.LicenseMode != "trial" {
		t.Fatalf("expected trial capability snapshot, got %#v", listed[0].Capabilities)
	}
}

func testConfig(root string) config.RuntimeConfig {
	cfg := config.DefaultRuntimeConfig()
	cfg.Paths.ConfigDir = filepath.Join(root, "etc", "llstack")
	cfg.Paths.StateDir = filepath.Join(root, "var", "lib", "llstack", "state")
	cfg.Paths.HistoryDir = filepath.Join(root, "var", "lib", "llstack", "history")
	cfg.Paths.BackupsDir = filepath.Join(root, "var", "lib", "llstack", "backups")
	cfg.Paths.LogDir = filepath.Join(root, "var", "log", "llstack")
	cfg.Paths.SitesRootDir = filepath.Join(root, "data", "www")
	cfg.Apache.ManagedVhostsDir = filepath.Join(root, "etc", "httpd", "conf.d", "llstack", "sites")
	cfg.Apache.ConfigTestCmd = []string{"apachectl", "configtest"}
	cfg.Apache.ReloadCmd = []string{"systemctl", "reload", "httpd"}
	cfg.OLS.ManagedVhostsRoot = filepath.Join(root, "usr", "local", "lsws", "conf", "vhosts")
	cfg.OLS.ManagedListenersDir = filepath.Join(root, "usr", "local", "lsws", "conf", "llstack", "listeners")
	cfg.OLS.DefaultPHPBinary = "/usr/bin/lsphp"
	cfg.OLS.ConfigTestCmd = []string{"lswsctrl", "configtest"}
	cfg.OLS.ReloadCmd = []string{"lswsctrl", "reload"}
	cfg.LSWS.ManagedIncludesDir = filepath.Join(root, "usr", "local", "lsws", "conf", "llstack", "includes")
	cfg.LSWS.DefaultPHPBinary = "/usr/bin/lsphp"
	cfg.LSWS.LicenseSerialFile = filepath.Join(root, "usr", "local", "lsws", "conf", "serial.no")
	cfg.LSWS.DetectCmd = []string{"lshttpd", "-v"}
	cfg.LSWS.ConfigTestCmd = []string{"lswsctrl", "configtest"}
	cfg.LSWS.ReloadCmd = []string{"lswsctrl", "reload"}
	return cfg
}
