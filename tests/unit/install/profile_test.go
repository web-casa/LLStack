package install_test

import (
	"os"
	"path/filepath"
	"testing"

	installsvc "github.com/web-casa/llstack/internal/install"
)

func TestLoadProfileYAML(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "install.yaml")
	if err := os.WriteFile(path, []byte(`
backend: ols
php_version: "8.4"
php_versions:
  - "8.3"
  - "8.4"
  - "8.4"
db: postgresql
db_tls: required
with_memcached: true
site: example.com
site_profile: static
dry_run: true
`), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	profile, err := installsvc.LoadProfile(path)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if profile.Backend != "ols" || profile.PHPVersion != "8.4" || profile.DB != "postgresql" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
	if len(profile.PHPVersions) != 2 {
		t.Fatalf("expected deduped php versions, got %#v", profile.PHPVersions)
	}
	opts := profile.ToOptions()
	if opts.SiteProfile != "static" || !opts.WithMemcached || !opts.DryRun {
		t.Fatalf("unexpected normalized options: %#v", opts)
	}
}

func TestLoadProfileNestedYAML(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "install.yaml")
	if err := os.WriteFile(path, []byte(`
backend: apache
php:
  primary_version: "8.3"
  versions:
    - "8.2"
    - "8.3"
database:
  provider: mariadb
  tls: enabled
cache:
  memcached: true
  redis: false
first_site:
  domain: nested.example.com
  profile: generic
operator:
  email: admin@example.com
execution:
  non_interactive: true
  dry_run: true
`), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	profile, err := installsvc.LoadProfile(path)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	opts := profile.ToOptions()
	if opts.Backend != "apache" || opts.PHPVersion != "8.3" || opts.DBProvider != "mariadb" {
		t.Fatalf("unexpected nested options: %#v", opts)
	}
	if opts.Site != "nested.example.com" || opts.SiteProfile != "generic" {
		t.Fatalf("unexpected first site options: %#v", opts)
	}
	if !opts.WithMemcached || opts.WithRedis || !opts.NonInteractive || !opts.DryRun {
		t.Fatalf("unexpected execution/cache options: %#v", opts)
	}
}

func TestLoadProfileScenarioYAML(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "install.yaml")
	if err := os.WriteFile(path, []byte(`
backend: ols
scenario_profile:
  name: reverse-proxy
  upstream: http://127.0.0.1:9000
  install_redis: true
execution:
  dry_run: true
`), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	profile, err := installsvc.LoadProfile(path)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	opts := profile.ToOptions()
	if opts.SiteProfile != "reverse-proxy" {
		t.Fatalf("expected reverse-proxy site profile, got %#v", opts)
	}
	if opts.SiteUpstream != "http://127.0.0.1:9000" {
		t.Fatalf("expected upstream to be preserved, got %#v", opts)
	}
	if !opts.WithRedis || !opts.DryRun {
		t.Fatalf("unexpected scenario options: %#v", opts)
	}
}

func TestLoadProfileScenarioWordPress(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "install.yaml")
	if err := os.WriteFile(path, []byte(`
backend: apache
scenario_profile:
  name: wordpress
execution:
  dry_run: true
`), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	profile, err := installsvc.LoadProfile(path)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	opts := profile.ToOptions()
	if opts.DBProvider != "mariadb" {
		t.Fatalf("expected wordpress scenario to default DB to mariadb, got %s", opts.DBProvider)
	}
	if opts.PHPVersion != "8.3" {
		t.Fatalf("expected wordpress scenario to default PHP to 8.3, got %s", opts.PHPVersion)
	}
	if !opts.WithMemcached {
		t.Fatalf("expected wordpress scenario to enable memcached")
	}
	if opts.DBTLS != "enabled" {
		t.Fatalf("expected wordpress scenario to default TLS to enabled, got %s", opts.DBTLS)
	}
	if opts.SiteProfile != "wordpress" {
		t.Fatalf("expected wordpress site profile, got %s", opts.SiteProfile)
	}
}

func TestLoadProfileScenarioLaravel(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "install.yaml")
	if err := os.WriteFile(path, []byte(`
backend: apache
scenario_profile:
  name: laravel
execution:
  dry_run: true
`), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	profile, err := installsvc.LoadProfile(path)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	opts := profile.ToOptions()
	if opts.DBProvider != "postgresql" {
		t.Fatalf("expected laravel scenario to default DB to postgresql, got %s", opts.DBProvider)
	}
	if !opts.WithRedis {
		t.Fatalf("expected laravel scenario to enable redis")
	}
	if opts.SiteProfile != "laravel" {
		t.Fatalf("expected laravel site profile, got %s", opts.SiteProfile)
	}
}

func TestLoadProfileScenarioAPI(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "install.yaml")
	if err := os.WriteFile(path, []byte(`
backend: ols
scenario_profile:
  name: api
execution:
  dry_run: true
`), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	profile, err := installsvc.LoadProfile(path)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	opts := profile.ToOptions()
	if opts.DBProvider != "postgresql" {
		t.Fatalf("expected api scenario to default DB to postgresql, got %s", opts.DBProvider)
	}
	if opts.PHPVersion != "8.3" {
		t.Fatalf("expected api scenario to default PHP to 8.3, got %s", opts.PHPVersion)
	}
	if !opts.WithRedis {
		t.Fatalf("expected api scenario to enable redis")
	}
}

func TestLoadProfileScenarioStatic(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "install.yaml")
	if err := os.WriteFile(path, []byte(`
backend: apache
scenario_profile:
  name: static
execution:
  dry_run: true
`), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	profile, err := installsvc.LoadProfile(path)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	opts := profile.ToOptions()
	if opts.SiteProfile != "static" {
		t.Fatalf("expected static site profile, got %s", opts.SiteProfile)
	}
	if opts.DBProvider != "" {
		t.Fatalf("expected static scenario to skip DB, got %s", opts.DBProvider)
	}
	if opts.WithMemcached || opts.WithRedis {
		t.Fatalf("expected static scenario to skip cache")
	}
}
