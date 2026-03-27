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

type phase7Executor struct {
	commands []system.Command
}

func (f *phase7Executor) Run(ctx context.Context, cmd system.Command) (system.Result, error) {
	f.commands = append(f.commands, cmd)
	return system.Result{ExitCode: 0}, nil
}

func TestUpdateTLSLetsEncryptAndReadLogs(t *testing.T) {
	cfg := testConfigPhase7(t.TempDir())
	cfg.SSL.CertbotCandidates = []string{writeMockCertbot(t)}
	exec := &phase7Executor{}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "phase7.example.com",
			Domain: model.DomainBinding{
				ServerName: "phase7.example.com",
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: true,
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	if _, err := manager.UpdateTLS(context.Background(), site.UpdateTLSOptions{
		Name:       "phase7.example.com",
		Mode:       "letsencrypt",
		Email:      "admin@example.com",
		SkipReload: true,
	}); err != nil {
		t.Fatalf("update tls: %v", err)
	}

	manifest, err := manager.Show("phase7.example.com")
	if err != nil {
		t.Fatalf("show site: %v", err)
	}
	if !manifest.Site.TLS.Enabled || manifest.Site.TLS.Mode != "letsencrypt" {
		t.Fatalf("unexpected tls state: %#v", manifest.Site.TLS)
	}

	accessPath := filepath.Join(cfg.Paths.SiteLogsDir(), "phase7.example.com.access.log")
	if err := os.MkdirAll(filepath.Dir(accessPath), 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(accessPath, []byte("1\n2\n3\n"), 0o644); err != nil {
		t.Fatalf("write access log: %v", err)
	}
	lines, err := manager.ReadLogs(site.LogReadOptions{Name: "phase7.example.com", Lines: 2})
	if err != nil {
		t.Fatalf("read logs: %v", err)
	}
	if len(lines) != 2 || lines[0] != "2" || lines[1] != "3" {
		t.Fatalf("unexpected log lines: %#v", lines)
	}
}

func TestUpdateTLSLetsEncryptUsesDetectedCertbotBinary(t *testing.T) {
	cfg := testConfigPhase7(t.TempDir())
	mockCertbot := writeMockCertbot(t)
	cfg.SSL.CertbotCandidates = []string{mockCertbot}

	exec := &phase7Executor{}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "acme.example.com",
			Domain: model.DomainBinding{
				ServerName: "acme.example.com",
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: true,
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	if _, err := manager.UpdateTLS(context.Background(), site.UpdateTLSOptions{
		Name:       "acme.example.com",
		Mode:       "letsencrypt",
		Email:      "admin@example.com",
		SkipReload: true,
	}); err != nil {
		t.Fatalf("update tls with detected certbot: %v", err)
	}

	var command system.Command
	found := false
	for _, cmd := range exec.commands {
		if cmd.Name == mockCertbot {
			command = cmd
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected certbot command, got commands: %v", exec.commands)
	}
	if len(command.Args) < 10 || command.Args[0] != "certonly" || command.Args[1] != "--webroot" {
		t.Fatalf("unexpected certbot args: %#v", command.Args)
	}
}

func TestCreateStaticProfileWritesScaffoldAndDiffDetectsDrift(t *testing.T) {
	cfg := testConfigPhase7(t.TempDir())
	exec := &phase7Executor{}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name:    "static.example.com",
			Profile: site.ProfileStatic,
			Domain: model.DomainBinding{
				ServerName: "static.example.com",
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create static site: %v", err)
	}

	indexPath := filepath.Join(cfg.Paths.SitesRootDir, "static.example.com", "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("expected scaffold file: %v", err)
	}
	if err := os.WriteFile(indexPath, []byte("drifted"), 0o644); err != nil {
		t.Fatalf("mutate scaffold: %v", err)
	}

	report, err := manager.Diff(context.Background(), "static.example.com")
	if err != nil {
		t.Fatalf("diff site: %v", err)
	}
	if len(report.Entries) == 0 {
		t.Fatal("expected drift entries")
	}
}

func TestCreateWordPressAndLaravelProfilesWriteExtendedScaffolds(t *testing.T) {
	cfg := testConfigPhase7(t.TempDir())
	exec := &phase7Executor{}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name:    "wp.example.com",
			Profile: site.ProfileWordPress,
			Domain: model.DomainBinding{
				ServerName: "wp.example.com",
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create wordpress site: %v", err)
	}
	for _, path := range []string{
		filepath.Join(cfg.Paths.SitesRootDir, "wp.example.com", "wp-config-sample.php"),
		filepath.Join(cfg.Paths.SitesRootDir, "wp.example.com", "README-LLSTACK.md"),
		filepath.Join(cfg.Paths.SitesRootDir, "wp.example.com", "wp-content", "uploads", ".gitkeep"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected wordpress scaffold asset %s: %v", path, err)
		}
	}

	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name:    "laravel.example.com",
			Profile: site.ProfileLaravel,
			Domain: model.DomainBinding{
				ServerName: "laravel.example.com",
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create laravel site: %v", err)
	}
	projectRoot := filepath.Join(cfg.Paths.SitesRootDir, "laravel.example.com")
	for _, path := range []string{
		filepath.Join(projectRoot, ".env.example"),
		filepath.Join(projectRoot, "artisan"),
		filepath.Join(projectRoot, "bootstrap", "app.php"),
		filepath.Join(projectRoot, "routes", "web.php"),
		filepath.Join(projectRoot, "storage", "logs", ".gitkeep"),
		filepath.Join(projectRoot, "README-LLSTACK.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected laravel scaffold asset %s: %v", path, err)
		}
	}
}

func TestStopAndStartSiteToggleManagedConfigPath(t *testing.T) {
	cfg := testConfigPhase7(t.TempDir())
	exec := &phase7Executor{}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "toggle.example.com",
			Domain: model.DomainBinding{
				ServerName: "toggle.example.com",
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: true,
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	if _, err := manager.SetState(context.Background(), site.StateChangeOptions{
		Name:       "toggle.example.com",
		State:      "disabled",
		SkipReload: true,
	}); err != nil {
		t.Fatalf("disable site: %v", err)
	}
	disabledPath := filepath.Join(cfg.Apache.ManagedVhostsDir, "toggle.example.com.conf.disabled")
	if _, err := os.Stat(disabledPath); err != nil {
		t.Fatalf("expected disabled config path: %v", err)
	}
	manifest, err := manager.Show("toggle.example.com")
	if err != nil {
		t.Fatalf("show disabled site: %v", err)
	}
	if manifest.Site.State != "disabled" {
		t.Fatalf("expected disabled state, got %s", manifest.Site.State)
	}

	if _, err := manager.SetState(context.Background(), site.StateChangeOptions{
		Name:       "toggle.example.com",
		State:      "enabled",
		SkipReload: true,
	}); err != nil {
		t.Fatalf("enable site: %v", err)
	}
	enabledPath := filepath.Join(cfg.Apache.ManagedVhostsDir, "toggle.example.com.conf")
	if _, err := os.Stat(enabledPath); err != nil {
		t.Fatalf("expected enabled config path: %v", err)
	}
}

func TestRestartSiteRunsBackendRestartCommand(t *testing.T) {
	cfg := testConfigPhase7(t.TempDir())
	exec := &phase7Executor{}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "restart.example.com",
			Domain: model.DomainBinding{
				ServerName: "restart.example.com",
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: true,
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	if _, err := manager.Restart(context.Background(), "restart.example.com"); err != nil {
		t.Fatalf("restart site: %v", err)
	}
	if len(exec.commands) < 2 {
		t.Fatalf("expected restart to run verifier commands, got %d", len(exec.commands))
	}
	last := exec.commands[len(exec.commands)-1]
	if last.Name != "systemctl" || len(last.Args) < 2 || last.Args[0] != "restart" || last.Args[1] != "httpd" {
		t.Fatalf("expected restart command, got %#v", last)
	}
}

func TestUpdateSiteSettingsRewritesManifest(t *testing.T) {
	cfg := testConfigPhase7(t.TempDir())
	exec := &phase7Executor{}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "edit.example.com",
			Domain: model.DomainBinding{
				ServerName: "edit.example.com",
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: true,
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	newRoot := filepath.Join(cfg.Paths.SitesRootDir, "custom", "edit.example.com")
	if _, err := manager.UpdateSettings(context.Background(), site.UpdateSettingsOptions{
		Name:         "edit.example.com",
		DocumentRoot: newRoot,
		Aliases:      []string{"www.edit.example.com", "cdn.edit.example.com"},
		IndexFiles:   []string{"index.php", "index.html"},
		SkipReload:   true,
	}); err != nil {
		t.Fatalf("update site settings: %v", err)
	}

	manifest, err := manager.Show("edit.example.com")
	if err != nil {
		t.Fatalf("show updated site: %v", err)
	}
	if manifest.Site.DocumentRoot != newRoot {
		t.Fatalf("expected updated docroot %s, got %s", newRoot, manifest.Site.DocumentRoot)
	}
	if len(manifest.Site.Domain.Aliases) != 2 {
		t.Fatalf("expected aliases to be updated, got %#v", manifest.Site.Domain.Aliases)
	}
	if len(manifest.Site.IndexFiles) != 2 || manifest.Site.IndexFiles[0] != "index.php" || manifest.Site.IndexFiles[1] != "index.html" {
		t.Fatalf("expected index files to be updated, got %#v", manifest.Site.IndexFiles)
	}
}

func TestUpdateReverseProxyUpstreamRewritesManifest(t *testing.T) {
	cfg := testConfigPhase7(t.TempDir())
	exec := &phase7Executor{}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	reverseProxySite := model.Site{
		Name: "proxy-edit.example.com",
		Domain: model.DomainBinding{
			ServerName: "proxy-edit.example.com",
		},
	}
	if err := site.ApplyProfile(&reverseProxySite, site.ProfileReverseProxy, "http://127.0.0.1:8080"); err != nil {
		t.Fatalf("apply reverse proxy profile: %v", err)
	}
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site:       reverseProxySite,
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create reverse proxy site: %v", err)
	}

	if _, err := manager.UpdateSettings(context.Background(), site.UpdateSettingsOptions{
		Name:       "proxy-edit.example.com",
		Upstream:   "http://127.0.0.1:9000",
		SkipReload: true,
	}); err != nil {
		t.Fatalf("update reverse proxy settings: %v", err)
	}

	manifest, err := manager.Show("proxy-edit.example.com")
	if err != nil {
		t.Fatalf("show updated reverse proxy site: %v", err)
	}
	if len(manifest.Site.ReverseProxyRules) != 1 || manifest.Site.ReverseProxyRules[0].Upstream != "http://127.0.0.1:9000" {
		t.Fatalf("expected updated reverse proxy upstream, got %#v", manifest.Site.ReverseProxyRules)
	}
}

func testConfigPhase7(root string) config.RuntimeConfig {
	cfg := config.DefaultRuntimeConfig()
	cfg.Paths.ConfigDir = filepath.Join(root, "etc", "llstack")
	cfg.Paths.StateDir = filepath.Join(root, "var", "lib", "llstack", "state")
	cfg.Paths.HistoryDir = filepath.Join(root, "var", "lib", "llstack", "history")
	cfg.Paths.BackupsDir = filepath.Join(root, "var", "lib", "llstack", "backups")
	cfg.Paths.LogDir = filepath.Join(root, "var", "log", "llstack")
	cfg.Paths.SitesRootDir = filepath.Join(root, "data", "www")
	cfg.Apache.ManagedVhostsDir = filepath.Join(root, "etc", "httpd", "conf.d", "llstack", "sites")
	cfg.OLS.ManagedVhostsRoot = filepath.Join(root, "usr", "local", "lsws", "conf", "vhosts")
	cfg.OLS.ManagedListenersDir = filepath.Join(root, "usr", "local", "lsws", "conf", "llstack", "listeners")
	cfg.LSWS.ManagedIncludesDir = filepath.Join(root, "usr", "local", "lsws", "conf", "llstack", "includes")
	return cfg
}

func writeMockCertbot(t *testing.T) string {
	t.Helper()
	mockCertbot := filepath.Join(t.TempDir(), "certbot")
	if err := os.WriteFile(mockCertbot, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write mock certbot: %v", err)
	}
	return mockCertbot
}
