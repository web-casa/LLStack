package install_test

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/web-casa/llstack/internal/config"
	installsvc "github.com/web-casa/llstack/internal/install"
	"github.com/web-casa/llstack/internal/logging"
	"github.com/web-casa/llstack/internal/system"
)

type fakeExecutor struct {
	commands []system.Command
}

func (f *fakeExecutor) Run(ctx context.Context, cmd system.Command) (system.Result, error) {
	f.commands = append(f.commands, cmd)
	return system.Result{ExitCode: 0}, nil
}

func TestInstallServiceAggregatesSubsystemPlans(t *testing.T) {
	cfg := testConfig(t.TempDir())
	exec := &fakeExecutor{}
	service := installsvc.NewService(cfg, logging.NewDefault(io.Discard), exec)

	p, err := service.Execute(context.Background(), installsvc.Options{
		Backend:       "apache",
		PHPVersion:    "8.3",
		DBProvider:    "mariadb",
		DBTLS:         "enabled",
		WithMemcached: true,
		Site:          "example.com",
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("execute install: %v", err)
	}
	if len(p.Operations) == 0 {
		t.Fatal("expected aggregated install operations")
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
	cfg.PHP.ProfileRoot = filepath.Join(root, "etc", "opt", "remi")
	cfg.PHP.RuntimeRoot = filepath.Join(root, "opt", "remi")
	cfg.PHP.StateRoot = filepath.Join(root, "var", "opt", "remi")
	cfg.DB.ManagedProvidersDir = filepath.Join(root, "etc", "llstack", "db", "providers")
	cfg.DB.ManagedConnectionsDir = filepath.Join(root, "etc", "llstack", "db", "connections")
	cfg.DB.CertificatesDir = filepath.Join(root, "etc", "llstack", "db", "certs")
	cfg.DB.MariaDBTLSConfigPath = filepath.Join(root, "etc", "my.cnf.d", "llstack-mariadb-tls.cnf")
	cfg.Cache.ManagedProvidersDir = filepath.Join(root, "etc", "llstack", "cache", "providers")
	return cfg
}
