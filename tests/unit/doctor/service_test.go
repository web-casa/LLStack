package doctor_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/web-casa/llstack/internal/cache"
	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/db"
	"github.com/web-casa/llstack/internal/doctor"
	"github.com/web-casa/llstack/internal/logging"
	"github.com/web-casa/llstack/internal/site"
	"github.com/web-casa/llstack/internal/system"
)

type fakeExecutor struct {
	results map[string]system.Result
}

func (f fakeExecutor) Run(ctx context.Context, cmd system.Command) (system.Result, error) {
	key := commandKey(cmd)
	if result, ok := f.results[key]; ok {
		return result, nil
	}
	return system.Result{ExitCode: 0}, nil
}

type recordingExecutor struct {
	results map[string]system.Result
	calls   []system.Command
}

func (r *recordingExecutor) Run(ctx context.Context, cmd system.Command) (system.Result, error) {
	r.calls = append(r.calls, cmd)
	key := commandKey(cmd)
	if result, ok := r.results[key]; ok {
		return result, nil
	}
	return system.Result{ExitCode: 0}, nil
}

func TestDoctorRunIncludesCoreChecks(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), fakeExecutor{
		results: map[string]system.Result{
			"getenforce":                {Stdout: "Permissive\n", ExitCode: 0},
			"firewall-cmd --state":      {Stdout: "running\n", ExitCode: 0},
			"firewall-cmd --list-ports": {Stdout: "80/tcp 443/tcp\n", ExitCode: 0},
			"ss -ltn":                   {Stdout: "LISTEN 0 128 *:80 *:*\nLISTEN 0 128 *:443 *:*\n", ExitCode: 0},
		},
	})

	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}
	if len(report.Checks) < 10 {
		t.Fatalf("expected multiple checks, got %#v", report.Checks)
	}
	checks := map[string]bool{}
	for _, check := range report.Checks {
		checks[check.Name] = true
	}
	for _, name := range []string{"os_support", "managed_directories", "managed_sites", "runtime_binaries", "selinux_state", "firewalld_state", "listening_ports", "php_fpm_sockets", "php_fpm_process_health", "php_config_drift", "managed_path_ownership", "managed_path_permissions", "managed_selinux_contexts", "db_tls_state", "db_managed_artifacts", "managed_services", "managed_provider_ports", "managed_db_live_probe", "managed_db_auth_probe", "managed_cache_live_probe", "db_connection_saturation", "cache_memory_saturation", "ssl_certificate_expiry", "ols_htaccess_compat", "rollback_history"} {
		if !checks[name] {
			t.Fatalf("expected doctor check %s, got %#v", name, checks)
		}
	}
}

func TestRepairDryRunPlansDirectoryCreation(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), system.NewLocalExecutor())

	p, err := service.Repair(context.Background(), doctor.RepairOptions{
		DryRun:     true,
		SkipReload: true,
	})
	if err != nil {
		t.Fatalf("repair dry-run: %v", err)
	}
	if len(p.Operations) == 0 {
		t.Fatalf("expected repair operations, got %#v", p)
	}
}

func TestRepairDryRunPlansPermissionNormalization(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	if err := os.MkdirAll(cfg.Paths.ConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.Chmod(cfg.Paths.ConfigDir, 0o500); err != nil {
		t.Fatalf("chmod config dir: %v", err)
	}
	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), system.NewLocalExecutor())

	p, err := service.Repair(context.Background(), doctor.RepairOptions{
		DryRun:     true,
		SkipReload: true,
	})
	if err != nil {
		t.Fatalf("repair dry-run: %v", err)
	}
	for _, op := range p.Operations {
		if op.Kind == "chmod" && op.Target == cfg.Paths.ConfigDir {
			return
		}
	}
	t.Fatalf("expected chmod repair operation for %s, got %#v", cfg.Paths.ConfigDir, p.Operations)
}

func TestDoctorRunWarnsWhenRequiredPortsAreMissing(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), fakeExecutor{})
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "doctor-ports.example.com",
			Domain: model.DomainBinding{
				ServerName: "doctor-ports.example.com",
			},
			TLS: model.TLSConfig{
				Enabled:         true,
				Mode:            "custom",
				CertificateFile: "/tmp/fullchain.pem",
				CertificateKey:  "/tmp/privkey.pem",
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), fakeExecutor{
		results: map[string]system.Result{
			"getenforce":                {Stdout: "Permissive\n", ExitCode: 0},
			"firewall-cmd --state":      {Stdout: "running\n", ExitCode: 0},
			"firewall-cmd --list-ports": {Stdout: "80/tcp\n", ExitCode: 0},
			"ss -ltn":                   {Stdout: "LISTEN 0 128 *:80 *:*\n", ExitCode: 0},
		},
	})

	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}
	statusByName := map[string]string{}
	for _, check := range report.Checks {
		statusByName[check.Name] = check.Status
	}
	if statusByName["firewalld_state"] != doctor.StatusWarn {
		t.Fatalf("expected firewalld_state warning, got %#v", statusByName)
	}
	if statusByName["listening_ports"] != doctor.StatusWarn {
		t.Fatalf("expected listening_ports warning, got %#v", statusByName)
	}
}

func TestDoctorRunWarnsWhenPHPSocketIsMissing(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), fakeExecutor{})
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "socket-check.example.com",
			Domain: model.DomainBinding{
				ServerName: "socket-check.example.com",
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: true,
				Version: "8.3",
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), fakeExecutor{})
	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}
	for _, check := range report.Checks {
		if check.Name == "php_fpm_sockets" && check.Status != doctor.StatusWarn {
			t.Fatalf("expected php_fpm_sockets warning, got %#v", check)
		}
	}
}

func TestDoctorRunWarnsWhenDBTLSConfigIsMissing(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	if err := os.MkdirAll(cfg.DB.ManagedProvidersDir, 0o755); err != nil {
		t.Fatalf("mkdir db providers: %v", err)
	}
	manifest := db.ProviderManifest{
		Provider:    db.ProviderMariaDB,
		ServiceName: "mariadb",
		Status:      "initialized",
		TLS: db.DatabaseTLSProfile{
			Mode:             db.TLSEnabled,
			Enabled:          true,
			Status:           "configured",
			ServerConfigPath: filepath.Join(cfg.Paths.ConfigDir, "db", "missing-tls.cnf"),
			ServerCAFile:     filepath.Join(cfg.Paths.ConfigDir, "db", "certs", "ca.pem"),
		},
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DB.ManagedProvidersDir, "mariadb.json"), raw, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), fakeExecutor{})
	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}
	for _, check := range report.Checks {
		if check.Name == "db_tls_state" && check.Status != doctor.StatusWarn {
			t.Fatalf("expected db_tls_state warning, got %#v", check)
		}
	}
}

func TestDoctorRunWarnsWhenManagedServiceIsInactive(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), fakeExecutor{})
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "service-check.example.com",
			Domain: model.DomainBinding{
				ServerName: "service-check.example.com",
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), fakeExecutor{
		results: map[string]system.Result{
			"systemctl is-active httpd": {Stdout: "inactive\n", ExitCode: 3},
		},
	})
	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}
	for _, check := range report.Checks {
		if check.Name == "managed_services" {
			if check.Status != doctor.StatusWarn {
				t.Fatalf("expected managed_services warning, got %#v", check)
			}
			if !strings.Contains(check.Details["restartable"], "httpd") {
				t.Fatalf("expected restartable httpd, got %#v", check.Details)
			}
			return
		}
	}
	t.Fatalf("managed_services check not found")
}

func TestDoctorRunWarnsWhenManagedCacheProbeFails(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	if err := os.MkdirAll(cfg.Cache.ManagedProvidersDir, 0o755); err != nil {
		t.Fatalf("mkdir cache providers: %v", err)
	}
	manifest := cache.ProviderManifest{
		Provider:    cache.ProviderRedis,
		ServiceName: "redis",
		Bind:        "127.0.0.1",
		Port:        6379,
		Status:      "configured",
		UpdatedAt:   time.Now().UTC(),
		Capabilities: cache.ProviderCapability{
			Provider:            cache.ProviderRedis,
			SupportsPersistence: true,
			SupportsEviction:    true,
		},
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal cache manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Cache.ManagedProvidersDir, "redis.json"), raw, 0o644); err != nil {
		t.Fatalf("write cache manifest: %v", err)
	}

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), fakeExecutor{
		results: map[string]system.Result{
			"redis-cli -h 127.0.0.1 -p 6379 PING": {Stdout: "NOAUTH\n", ExitCode: 1},
		},
	})
	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}
	for _, check := range report.Checks {
		if check.Name == "managed_cache_live_probe" {
			if check.Status != doctor.StatusWarn {
				t.Fatalf("expected managed_cache_live_probe warning, got %#v", check)
			}
			return
		}
	}
	t.Fatalf("expected managed_cache_live_probe check")
}

func TestDoctorRunWarnsWhenManagedProviderPortsAreMissing(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	if err := os.MkdirAll(cfg.DB.ManagedProvidersDir, 0o755); err != nil {
		t.Fatalf("mkdir db providers: %v", err)
	}
	if err := os.MkdirAll(cfg.Cache.ManagedProvidersDir, 0o755); err != nil {
		t.Fatalf("mkdir cache providers: %v", err)
	}

	dbManifest := db.ProviderManifest{
		Provider:    db.ProviderMariaDB,
		Version:     "10.x",
		ServiceName: "mariadb",
		Status:      "initialized",
	}
	rawDB, err := json.Marshal(dbManifest)
	if err != nil {
		t.Fatalf("marshal db manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DB.ManagedProvidersDir, "mariadb.json"), rawDB, 0o644); err != nil {
		t.Fatalf("write db manifest: %v", err)
	}

	cacheManifest := `{"provider":"redis","service_name":"redis","status":"configured","port":6379}`
	if err := os.WriteFile(filepath.Join(cfg.Cache.ManagedProvidersDir, "redis.json"), []byte(cacheManifest), 0o644); err != nil {
		t.Fatalf("write cache manifest: %v", err)
	}

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), fakeExecutor{
		results: map[string]system.Result{
			"ss -ltn": {Stdout: "LISTEN 0 128 *:80 *:*\nLISTEN 0 128 *:6379 *:*\n", ExitCode: 0},
		},
	})
	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}
	for _, check := range report.Checks {
		if check.Name == "managed_provider_ports" {
			if check.Status != doctor.StatusWarn {
				t.Fatalf("expected managed_provider_ports warning, got %#v", check)
			}
			if !strings.Contains(check.Details["missing"], "db:mariadb:3306") {
				t.Fatalf("expected missing mariadb port, got %#v", check.Details)
			}
			return
		}
	}
	t.Fatalf("managed_provider_ports check not found")
}

func TestDoctorRunWarnsWhenManagedSELinuxContextLooksSuspicious(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	if err := os.MkdirAll(cfg.Paths.ConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.MkdirAll(cfg.Apache.ManagedVhostsDir, 0o755); err != nil {
		t.Fatalf("mkdir apache dir: %v", err)
	}

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), fakeExecutor{
		results: map[string]system.Result{
			"getenforce":                            {Stdout: "Enforcing\n", ExitCode: 0},
			"ls -Zd " + cfg.Paths.ConfigDir:         {Stdout: "unconfined_u:object_r:default_t:s0 " + cfg.Paths.ConfigDir + "\n", ExitCode: 0},
			"ls -Zd " + cfg.Apache.ManagedVhostsDir: {Stdout: "system_u:object_r:httpd_config_t:s0 " + cfg.Apache.ManagedVhostsDir + "\n", ExitCode: 0},
		},
	})

	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}
	for _, check := range report.Checks {
		if check.Name == "managed_selinux_contexts" {
			if check.Status != doctor.StatusWarn {
				t.Fatalf("expected managed_selinux_contexts warning, got %#v", check)
			}
			if !strings.Contains(check.Details["problems"], cfg.Paths.ConfigDir) {
				t.Fatalf("expected suspicious config dir context, got %#v", check.Details)
			}
			return
		}
	}
	t.Fatalf("managed_selinux_contexts check not found")
}

func TestDoctorRunHandlesManagedDatabaseLiveProbe(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	if err := os.MkdirAll(cfg.DB.ManagedProvidersDir, 0o755); err != nil {
		t.Fatalf("mkdir db providers: %v", err)
	}

	mariaManifest := db.ProviderManifest{
		Provider:    db.ProviderMariaDB,
		Version:     "10.x",
		ServiceName: "mariadb",
		Status:      "initialized",
		AdminConnection: &db.ConnectionInfo{
			Host: "127.0.0.1",
			Port: 3306,
		},
	}
	pgManifest := db.ProviderManifest{
		Provider:    db.ProviderPostgres,
		Version:     "16",
		ServiceName: "postgresql-16",
		Status:      "initialized",
		AdminConnection: &db.ConnectionInfo{
			Host: "127.0.0.1",
			Port: 5432,
		},
	}
	rawMaria, err := json.Marshal(mariaManifest)
	if err != nil {
		t.Fatalf("marshal mariadb manifest: %v", err)
	}
	rawPG, err := json.Marshal(pgManifest)
	if err != nil {
		t.Fatalf("marshal postgres manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DB.ManagedProvidersDir, "mariadb.json"), rawMaria, 0o644); err != nil {
		t.Fatalf("write mariadb manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DB.ManagedProvidersDir, "postgresql.json"), rawPG, 0o644); err != nil {
		t.Fatalf("write postgres manifest: %v", err)
	}

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), fakeExecutor{
		results: map[string]system.Result{
			"mysqladmin --protocol=tcp --host 127.0.0.1 --port 3306 ping": {Stdout: "mysqld is alive\n", ExitCode: 0},
			"pg_isready -h 127.0.0.1 -p 5432":                             {Stderr: "no response\n", ExitCode: 2},
		},
	})
	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}
	for _, check := range report.Checks {
		if check.Name == "managed_db_live_probe" {
			if check.Status != doctor.StatusWarn {
				t.Fatalf("expected managed_db_live_probe warning, got %#v", check)
			}
			if !strings.Contains(check.Details["passed"], "mariadb@127.0.0.1:3306") {
				t.Fatalf("expected passed mariadb probe, got %#v", check.Details)
			}
			if !strings.Contains(check.Details["failed"], "postgresql@127.0.0.1:5432") {
				t.Fatalf("expected failed postgres probe, got %#v", check.Details)
			}
			return
		}
	}
	t.Fatalf("managed_db_live_probe check not found")
}

func TestDoctorRunHandlesManagedDatabaseAuthProbe(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	if err := os.MkdirAll(cfg.DB.ManagedProvidersDir, 0o755); err != nil {
		t.Fatalf("mkdir db providers: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.Paths.ConfigDir, "db", "credentials"), 0o755); err != nil {
		t.Fatalf("mkdir db credentials: %v", err)
	}
	passwordFile := filepath.Join(cfg.Paths.ConfigDir, "db", "credentials", "mariadb-admin.secret")
	if err := os.WriteFile(passwordFile, []byte("secret\n"), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	manifest := db.ProviderManifest{
		Provider:    db.ProviderMariaDB,
		Version:     "10.x",
		ServiceName: "mariadb",
		Status:      "initialized",
		AdminConnection: &db.ConnectionInfo{
			Host:         "127.0.0.1",
			Port:         3306,
			User:         "llstack_admin",
			PasswordFile: passwordFile,
		},
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DB.ManagedProvidersDir, "mariadb.json"), raw, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), fakeExecutor{
		results: map[string]system.Result{
			"mysql --protocol=tcp --host 127.0.0.1 --port 3306 --user llstack_admin --password=secret -e SELECT 1;": {Stdout: "1\n", ExitCode: 0},
		},
	})
	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}
	for _, check := range report.Checks {
		if check.Name == "managed_db_auth_probe" {
			if check.Status != doctor.StatusPass {
				t.Fatalf("expected managed_db_auth_probe pass, got %#v", check)
			}
			if !strings.Contains(check.Details["passed"], "mariadb@127.0.0.1:3306/llstack_admin") {
				t.Fatalf("expected passed auth probe label, got %#v", check.Details)
			}
			return
		}
	}
	t.Fatalf("managed_db_auth_probe check not found")
}

func TestRepairPlansDatabaseReconcile(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	cfg.DB.MariaDBTLSConfigPath = filepath.Join(filepath.Dir(cfg.DB.ManagedProvidersDir), "tls", "llstack-mariadb-tls.cnf")
	exec := fakeExecutor{}
	manager := db.NewManager(cfg, logging.NewDefault(io.Discard), exec)
	if _, err := manager.Install(context.Background(), db.InstallOptions{
		Provider: db.ProviderMariaDB,
		TLSMode:  db.TLSEnabled,
	}); err != nil {
		t.Fatalf("install provider: %v", err)
	}
	if _, err := manager.Init(context.Background(), db.InitOptions{
		Provider:      db.ProviderMariaDB,
		AdminUser:     "llstack_admin",
		AdminPassword: "secret",
		TLSMode:       db.TLSEnabled,
	}); err != nil {
		t.Fatalf("init provider: %v", err)
	}
	if err := os.Remove(cfg.DB.MariaDBTLSConfigPath); err != nil {
		t.Fatalf("remove tls config: %v", err)
	}

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), exec)
	p, err := service.Repair(context.Background(), doctor.RepairOptions{
		DryRun:     true,
		SkipReload: true,
	})
	if err != nil {
		t.Fatalf("repair dry-run: %v", err)
	}
	for _, op := range p.Operations {
		if op.Kind == "db.reconcile" && op.Target == "mariadb" {
			return
		}
	}
	t.Fatalf("expected db.reconcile operation, got %#v", p.Operations)
}

func TestRepairPlansFirewalldPortUpdate(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	exec := fakeExecutor{
		results: map[string]system.Result{
			"firewall-cmd --state":      {Stdout: "running\n", ExitCode: 0},
			"firewall-cmd --list-ports": {Stdout: "80/tcp\n", ExitCode: 0},
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
			Name: "firewalld.example.com",
			Domain: model.DomainBinding{
				ServerName: "firewalld.example.com",
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
	p, err := service.Repair(context.Background(), doctor.RepairOptions{
		DryRun:     true,
		SkipReload: true,
	})
	if err != nil {
		t.Fatalf("repair dry-run: %v", err)
	}
	foundPort := false
	foundReload := false
	for _, op := range p.Operations {
		if op.Kind == "firewalld.add-port" && op.Target == "443/tcp" {
			foundPort = true
		}
		if op.Kind == "firewalld.reload" {
			foundReload = true
		}
	}
	if !foundPort || !foundReload {
		t.Fatalf("expected firewalld repair operations, got %#v", p.Operations)
	}
}

func TestRepairStartsInactiveManagedService(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	exec := &recordingExecutor{
		results: map[string]system.Result{
			"systemctl is-active httpd": {Stdout: "inactive\n", ExitCode: 3},
			"systemctl start httpd":     {ExitCode: 0},
		},
	}
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "repair-service.example.com",
			Domain: model.DomainBinding{
				ServerName: "repair-service.example.com",
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
	foundExec := false
	for _, op := range p.Operations {
		if op.Kind == "service.start" && op.Target == "httpd" {
			foundPlan = true
		}
	}
	for _, call := range exec.calls {
		if commandKey(call) == "systemctl start httpd" {
			foundExec = true
		}
	}
	if !foundPlan {
		t.Fatalf("expected repair plan to include service.start httpd, got %#v", p.Operations)
	}
	if !foundExec {
		t.Fatalf("expected repair to execute systemctl start httpd, got %#v", exec.calls)
	}
}

func TestBundleWritesDiagnosticsArchive(t *testing.T) {
	cfg := testDoctorConfig(t.TempDir())
	cfg.DB.MariaDBTLSConfigPath = filepath.Join(filepath.Dir(cfg.DB.ManagedProvidersDir), "tls", "llstack-mariadb-tls.cnf")
	cfg.Cache.MemcachedConfigPath = filepath.Join(filepath.Dir(cfg.Cache.ManagedProvidersDir), "sysconfig", "memcached")
	manager := site.NewManager(cfg, logging.NewDefault(io.Discard), fakeExecutor{})
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "bundle.example.com",
			Domain: model.DomainBinding{
				ServerName: "bundle.example.com",
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}
	manifest, err := manager.Show("bundle.example.com")
	if err != nil {
		t.Fatalf("show site: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(manifest.Site.Logs.AccessLog), 0o755); err != nil {
		t.Fatalf("mkdir site log dir: %v", err)
	}
	if err := os.WriteFile(manifest.Site.Logs.AccessLog, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatalf("write access log: %v", err)
	}
	if err := os.WriteFile(manifest.Site.Logs.ErrorLog, []byte("err1\nerr2\n"), 0o644); err != nil {
		t.Fatalf("write error log: %v", err)
	}
	if err := os.MkdirAll(cfg.Cache.ManagedProvidersDir, 0o755); err != nil {
		t.Fatalf("mkdir cache providers: %v", err)
	}
	cacheManifest := `{"provider":"memcached","service_name":"memcached","status":"configured"}`
	if err := os.WriteFile(filepath.Join(cfg.Cache.ManagedProvidersDir, "memcached.json"), []byte(cacheManifest), 0o644); err != nil {
		t.Fatalf("write cache manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DB.MariaDBTLSConfigPath), 0o755); err != nil {
		t.Fatalf("mkdir db tls dir: %v", err)
	}
	if err := os.WriteFile(cfg.DB.MariaDBTLSConfigPath, []byte("[mariadb]\nssl=on\n"), 0o644); err != nil {
		t.Fatalf("write db tls config: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Cache.MemcachedConfigPath), 0o755); err != nil {
		t.Fatalf("mkdir memcached config dir: %v", err)
	}
	if err := os.WriteFile(cfg.Cache.MemcachedConfigPath, []byte("PORT=11211\n"), 0o644); err != nil {
		t.Fatalf("write memcached config: %v", err)
	}

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), fakeExecutor{
		results: map[string]system.Result{
			"getenforce":                               {Stdout: "Permissive\n", ExitCode: 0},
			"firewall-cmd --state":                     {Stdout: "running\n", ExitCode: 0},
			"firewall-cmd --list-ports":                {Stdout: "80/tcp\n", ExitCode: 0},
			"ss -ltn":                                  {Stdout: "LISTEN 0 128 *:80 *:*\n", ExitCode: 0},
			"systemctl is-active httpd":                {Stdout: "active\n", ExitCode: 0},
			"systemctl is-active memcached":            {Stdout: "active\n", ExitCode: 0},
			"systemctl --failed --no-pager --plain":    {Stdout: "0 loaded units listed.\n", ExitCode: 0},
			"uname -a":                                 {Stdout: "Linux testhost 5.14.0 el9 x86_64\n", ExitCode: 0},
			"hostnamectl":                              {Stdout: "Static hostname: testhost\n", ExitCode: 0},
			"df -h":                                    {Stdout: "Filesystem Size Used Avail Use% Mounted on\n/dev/sda1 40G 10G 30G 25% /\n", ExitCode: 0},
			"free -m":                                  {Stdout: "Mem: 1024 256 512 0 256 768\n", ExitCode: 0},
			"journalctl -u httpd -n 50 --no-pager":     {Stdout: "Mar 25 10:00:00 testhost httpd[100]: ready\n", ExitCode: 0},
			"journalctl -u memcached -n 50 --no-pager": {Stdout: "Mar 25 10:00:01 testhost memcached[101]: accepting connections\n", ExitCode: 0},
		},
	})
	bundlePath := filepath.Join(t.TempDir(), "doctor-bundle.tar.gz")

	result, err := service.Bundle(context.Background(), bundlePath)
	if err != nil {
		t.Fatalf("bundle diagnostics: %v", err)
	}
	if result.Path != bundlePath {
		t.Fatalf("unexpected bundle path: %#v", result)
	}

	names := readTarGZNames(t, bundlePath)
	required := []string{
		"report.json",
		"summary.json",
		"host/runtime.json",
		"status/summary.json",
		"services/managed.json",
		"probes/checks.json",
		"commands/getenforce.json",
		"commands/firewall-state.json",
		"commands/firewall-list-ports.json",
		"commands/ss-ltn.json",
		"commands/systemctl-failed.json",
		"commands/uname-a.json",
		"commands/hostnamectl.json",
		"commands/df-h.json",
		"commands/free-m.json",
		"commands/ps-aux.json",
		"journal/services/httpd.json",
		"journal/services/memcached.json",
		"sites/bundle.example.com.json",
		"cache/providers/memcached.json",
		"config/db/mariadb-tls.conf",
		"config/cache/memcached.conf",
		"logs/sites/bundle.example.com/access.log",
		"logs/sites/bundle.example.com/error.log",
	}
	for _, want := range required {
		found := false
		for _, name := range names {
			if name == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected bundle entry %s, got %#v", want, names)
		}
	}

	logs := readTarGZText(t, bundlePath)
	if got := logs["logs/sites/bundle.example.com/access.log"]; !strings.Contains(got, "line3") {
		t.Fatalf("expected access log tail in bundle, got %q", got)
	}
	if got := logs["probes/checks.json"]; !strings.Contains(got, "\"managed_services\"") {
		t.Fatalf("expected probe snapshot content, got %q", got)
	}
	if got := logs["status/summary.json"]; !strings.Contains(got, "\"managed_sites\": 1") || !strings.Contains(got, "\"backends\": [") {
		t.Fatalf("expected status summary snapshot content, got %q", got)
	}
	if got := logs["services/managed.json"]; !strings.Contains(got, "\"active\": [") || !strings.Contains(got, "\"httpd\"") || !strings.Contains(got, "\"memcached\"") {
		t.Fatalf("expected managed service snapshot content, got %q", got)
	}
	if got := logs["host/runtime.json"]; !strings.Contains(got, "\"goos\"") || !strings.Contains(got, "\"sites_root\"") {
		t.Fatalf("expected host runtime snapshot content, got %q", got)
	}
	if got := logs["commands/getenforce.json"]; !strings.Contains(got, "\"command\": [") || !strings.Contains(got, "Permissive") {
		t.Fatalf("expected getenforce command snapshot content, got %q", got)
	}
	if got := logs["commands/systemctl-failed.json"]; !strings.Contains(got, "\"status\": \"ok\"") || !strings.Contains(got, "0 loaded units listed.") {
		t.Fatalf("expected systemctl failed snapshot content, got %q", got)
	}
	if got := logs["commands/uname-a.json"]; !strings.Contains(got, "Linux testhost") {
		t.Fatalf("expected uname snapshot content, got %q", got)
	}
	if got := logs["journal/services/httpd.json"]; !strings.Contains(got, "httpd[100]: ready") {
		t.Fatalf("expected httpd journal snapshot content, got %q", got)
	}
	if got := logs["journal/services/memcached.json"]; !strings.Contains(got, "accepting connections") {
		t.Fatalf("expected memcached journal snapshot content, got %q", got)
	}
	if got := logs["config/db/mariadb-tls.conf"]; !strings.Contains(got, "ssl=on") {
		t.Fatalf("expected mariadb config snapshot content, got %q", got)
	}
	if got := logs["config/cache/memcached.conf"]; !strings.Contains(got, "PORT=11211") {
		t.Fatalf("expected memcached config snapshot content, got %q", got)
	}
}

func commandKey(cmd system.Command) string {
	if len(cmd.Args) == 0 {
		return cmd.Name
	}
	return cmd.Name + " " + strings.Join(cmd.Args, " ")
}

func testDoctorConfig(root string) config.RuntimeConfig {
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

func readTarGZNames(t *testing.T, path string) []string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open bundle: %v", err)
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	var names []string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		names = append(names, header.Name)
	}
	return names
}

func TestDoctorRunWarnsOnDatabaseConnectionSaturation(t *testing.T) {
	root := t.TempDir()
	cfg := testDoctorConfig(root)
	providersDir := cfg.DB.ManagedProvidersDir
	connectionsDir := cfg.DB.ManagedConnectionsDir
	credentialsDir := filepath.Join(root, "etc", "llstack", "db", "credentials")
	for _, dir := range []string{providersDir, connectionsDir, credentialsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	manifest := db.ProviderManifest{
		Provider:    db.ProviderMariaDB,
		Version:     "",
		Family:      "mysql",
		ServiceName: "mariadb",
		Status:      "initialized",
		AdminConnection: &db.ConnectionInfo{
			Host:         "127.0.0.1",
			Port:         3306,
			User:         "root",
			PasswordFile: filepath.Join(credentialsDir, "mariadb-admin.secret"),
		},
	}
	raw, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(providersDir, "mariadb.json"), raw, 0o644)
	os.WriteFile(manifest.AdminConnection.PasswordFile, []byte("test-pass"), 0o600)

	// Simulate 90% saturation: 90 threads / 100 max
	saturationOutput := "Variable_name\tValue\nThreads_connected\t90\nVariable_name\tValue\nmax_connections\t100\n"

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), fakeExecutor{
		results: map[string]system.Result{
			"getenforce":                {Stdout: "Permissive\n"},
			"firewall-cmd --state":      {Stdout: "running\n"},
			"firewall-cmd --list-ports": {Stdout: "80/tcp\n"},
			"ss -ltn":                   {Stdout: "LISTEN 0 128 *:80 *:*\nLISTEN 0 128 *:3306 *:*\n"},
		},
	})

	// Override executor with one that returns saturation data for the mysql query
	service = doctor.NewService(cfg, logging.NewDefault(io.Discard), fakeExecutor{
		results: map[string]system.Result{
			"getenforce":                {Stdout: "Permissive\n"},
			"firewall-cmd --state":      {Stdout: "running\n"},
			"firewall-cmd --list-ports": {Stdout: "80/tcp 3306/tcp\n"},
			"ss -ltn":                   {Stdout: "LISTEN 0 128 *:80 *:*\nLISTEN 0 128 *:3306 *:*\n"},
			"mysql --protocol=tcp --host 127.0.0.1 --port 3306 --user root --password=test-pass --batch -e SHOW STATUS LIKE 'Threads_connected'; SHOW VARIABLES LIKE 'max_connections';": {Stdout: saturationOutput, ExitCode: 0},
			"mysqladmin --protocol=tcp --host 127.0.0.1 --port 3306 ping":                                                                                                               {Stdout: "mysqld is alive\n", ExitCode: 0},
			"mysql --protocol=tcp --host 127.0.0.1 --port 3306 --user root --password=test-pass -e SELECT 1;":                                                                           {Stdout: "1\n", ExitCode: 0},
			"systemctl is-active mariadb": {Stdout: "active\n", ExitCode: 0},
		},
	})

	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}
	for _, check := range report.Checks {
		if check.Name == "db_connection_saturation" {
			if check.Status != "warn" {
				t.Fatalf("expected db_connection_saturation warn, got %s: %s", check.Status, check.Summary)
			}
			if check.Details["warned_count"] != "1" {
				t.Fatalf("expected 1 warned provider, got %v", check.Details)
			}
			return
		}
	}
	t.Fatalf("expected db_connection_saturation check in report")
}

func TestDoctorRunWarnsOnCacheMemorySaturation(t *testing.T) {
	root := t.TempDir()
	cfg := testDoctorConfig(root)
	cacheDir := cfg.Cache.ManagedProvidersDir
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", cacheDir, err)
	}

	manifest := cache.ProviderManifest{
		Provider:    cache.ProviderRedis,
		ServiceName: "redis",
		ConfigPath:  "/etc/redis.conf",
		Bind:        "127.0.0.1",
		Port:        6379,
		Status:      "configured",
	}
	raw, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(cacheDir, "redis.json"), raw, 0o644)

	// Simulate 90% memory: 90MB used / 100MB max
	redisInfoOutput := "# Memory\r\nused_memory:94371840\r\nmaxmemory:104857600\r\nused_memory_human:90.00M\r\n"

	service := doctor.NewService(cfg, logging.NewDefault(io.Discard), fakeExecutor{
		results: map[string]system.Result{
			"getenforce":                                  {Stdout: "Permissive\n"},
			"firewall-cmd --state":                        {Stdout: "running\n"},
			"firewall-cmd --list-ports":                   {Stdout: "80/tcp\n"},
			"ss -ltn":                                     {Stdout: "LISTEN 0 128 *:80 *:*\nLISTEN 0 128 *:6379 *:*\n"},
			"redis-cli -h 127.0.0.1 -p 6379 PING":        {Stdout: "PONG\n", ExitCode: 0},
			"redis-cli -h 127.0.0.1 -p 6379 INFO memory": {Stdout: redisInfoOutput, ExitCode: 0},
			"systemctl is-active redis":                   {Stdout: "active\n", ExitCode: 0},
		},
	})

	report, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}
	for _, check := range report.Checks {
		if check.Name == "cache_memory_saturation" {
			if check.Status != "warn" {
				t.Fatalf("expected cache_memory_saturation warn, got %s: %s", check.Status, check.Summary)
			}
			if check.Details["warned_count"] != "1" {
				t.Fatalf("expected 1 warned provider, got %v", check.Details)
			}
			return
		}
	}
	t.Fatalf("expected cache_memory_saturation check in report")
}

func readTarGZText(t *testing.T, path string) map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open bundle: %v", err)
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	out := map[string]string{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		raw, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read tar entry: %v", err)
		}
		out[header.Name] = string(raw)
	}
	return out
}
