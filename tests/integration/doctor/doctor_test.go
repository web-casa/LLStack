package doctor_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/doctor"
	"github.com/web-casa/llstack/internal/logging"
	"github.com/web-casa/llstack/internal/site"
	"github.com/web-casa/llstack/internal/system"
)

type fakeExecutor struct{}

func (fakeExecutor) Run(ctx context.Context, cmd system.Command) (system.Result, error) {
	return system.Result{ExitCode: 0}, nil
}

type recordingExecutor struct {
	results map[string]system.Result
	calls   []system.Command
}

func (r *recordingExecutor) Run(ctx context.Context, cmd system.Command) (system.Result, error) {
	r.calls = append(r.calls, cmd)
	key := cmd.Name
	if len(cmd.Args) > 0 {
		for _, arg := range cmd.Args {
			key += " " + arg
		}
	}
	if result, ok := r.results[key]; ok {
		return result, nil
	}
	return system.Result{ExitCode: 0}, nil
}

func TestRepairRestoresMissingManagedSiteAssets(t *testing.T) {
	cfg := testConfig(t.TempDir())
	exec := fakeExecutor{}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "repair.example.com",
			Domain: model.DomainBinding{
				ServerName: "repair.example.com",
			},
			PHP: model.PHPRuntimeBinding{Enabled: true},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	manifest, err := manager.Show("repair.example.com")
	if err != nil {
		t.Fatalf("show site: %v", err)
	}
	if err := os.Remove(manifest.VHostPath); err != nil {
		t.Fatalf("remove managed asset: %v", err)
	}

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), exec)
	p, err := service.Repair(context.Background(), doctor.RepairOptions{SkipReload: true})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if len(p.Operations) == 0 {
		t.Fatalf("expected repair operations, got %#v", p)
	}
	if _, err := os.Stat(manifest.VHostPath); err != nil {
		t.Fatalf("expected repaired asset to be restored: %v", err)
	}
}

func TestRepairStartsInactiveBackendService(t *testing.T) {
	cfg := testConfig(t.TempDir())
	exec := &recordingExecutor{
		results: map[string]system.Result{
			"systemctl is-active httpd": {Stdout: "inactive\n", ExitCode: 3},
			"systemctl start httpd":     {ExitCode: 0},
		},
	}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "inactive-httpd.example.com",
			Domain: model.DomainBinding{
				ServerName: "inactive-httpd.example.com",
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), exec)
	p, err := service.Repair(context.Background(), doctor.RepairOptions{SkipReload: true})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}

	foundPlan := false
	foundStart := false
	for _, op := range p.Operations {
		if op.Kind == "service.start" && op.Target == "httpd" {
			foundPlan = true
		}
	}
	for _, call := range exec.calls {
		if call.Name == "systemctl" && len(call.Args) == 2 && call.Args[0] == "start" && call.Args[1] == "httpd" {
			foundStart = true
		}
	}
	if !foundPlan {
		t.Fatalf("expected service.start httpd in repair plan, got %#v", p.Operations)
	}
	if !foundStart {
		t.Fatalf("expected systemctl start httpd to be executed, got %#v", exec.calls)
	}
}

func TestRepairAddsMissingFirewalldPorts(t *testing.T) {
	cfg := testConfig(t.TempDir())
	exec := &recordingExecutor{
		results: map[string]system.Result{
			"firewall-cmd --state":                        {Stdout: "running\n", ExitCode: 0},
			"firewall-cmd --list-ports":                   {Stdout: "80/tcp\n", ExitCode: 0},
			"firewall-cmd --permanent --add-port=443/tcp": {ExitCode: 0},
			"firewall-cmd --reload":                       {ExitCode: 0},
		},
	}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)
	certFile := filepath.Join(t.TempDir(), "fullchain.pem")
	keyFile := filepath.Join(t.TempDir(), "privkey.pem")
	if err := os.WriteFile(certFile, []byte("cert"), 0o644); err != nil {
		t.Fatalf("write cert file: %v", err)
	}
	if err := os.WriteFile(keyFile, []byte("key"), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "https-only.example.com",
			Domain: model.DomainBinding{
				ServerName: "https-only.example.com",
			},
			TLS: model.TLSConfig{
				Enabled:         true,
				Mode:            "custom",
				CertificateFile: certFile,
				CertificateKey:  keyFile,
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), exec)
	p, err := service.Repair(context.Background(), doctor.RepairOptions{SkipReload: true})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}

	foundPlan := false
	foundAdd := false
	foundReload := false
	for _, op := range p.Operations {
		if op.Kind == "firewalld.add-port" && op.Target == "443/tcp" {
			foundPlan = true
		}
	}
	for _, call := range exec.calls {
		if call.Name == "firewall-cmd" && len(call.Args) == 2 && call.Args[0] == "--permanent" && call.Args[1] == "--add-port=443/tcp" {
			foundAdd = true
		}
		if call.Name == "firewall-cmd" && len(call.Args) == 1 && call.Args[0] == "--reload" {
			foundReload = true
		}
	}
	if !foundPlan {
		t.Fatalf("expected firewalld.add-port in repair plan, got %#v", p.Operations)
	}
	if !foundAdd || !foundReload {
		t.Fatalf("expected firewalld add/reload execution, got %#v", exec.calls)
	}
}

func TestRepairNormalizesManagedDirectoryPermissions(t *testing.T) {
	cfg := testConfig(t.TempDir())
	if err := os.MkdirAll(cfg.Paths.ConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.Chmod(cfg.Paths.ConfigDir, 0o500); err != nil {
		t.Fatalf("chmod config dir: %v", err)
	}

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), fakeExecutor{})
	p, err := service.Repair(context.Background(), doctor.RepairOptions{SkipReload: true})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	found := false
	for _, op := range p.Operations {
		if op.Kind == "chmod" && op.Target == cfg.Paths.ConfigDir {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected chmod operation in repair plan, got %#v", p.Operations)
	}

	info, err := os.Stat(cfg.Paths.ConfigDir)
	if err != nil {
		t.Fatalf("stat config dir: %v", err)
	}
	if info.Mode().Perm()&0o700 != 0o700 {
		t.Fatalf("expected repaired mode to include owner rwx, got %#o", info.Mode().Perm())
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
	cfg.OLS.ManagedVhostsRoot = filepath.Join(root, "usr", "local", "lsws", "conf", "vhosts")
	cfg.OLS.ManagedListenersDir = filepath.Join(root, "usr", "local", "lsws", "conf", "llstack", "listeners")
	cfg.LSWS.ManagedIncludesDir = filepath.Join(root, "usr", "local", "lsws", "conf", "llstack", "includes")
	cfg.PHP.ManagedRuntimesDir = filepath.Join(root, "etc", "llstack", "php", "runtimes")
	cfg.PHP.ManagedProfilesDir = filepath.Join(root, "etc", "llstack", "php", "profiles")
	cfg.DB.ManagedProvidersDir = filepath.Join(root, "etc", "llstack", "db", "providers")
	cfg.DB.ManagedConnectionsDir = filepath.Join(root, "etc", "llstack", "db", "connections")
	cfg.DB.CertificatesDir = filepath.Join(root, "etc", "llstack", "db", "certs")
	cfg.Cache.ManagedProvidersDir = filepath.Join(root, "etc", "llstack", "cache", "providers")
	return cfg
}
