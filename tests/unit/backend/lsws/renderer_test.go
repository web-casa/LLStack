package lsws_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/web-casa/llstack/internal/backend/lsws"
	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/core/render"
	"github.com/web-casa/llstack/internal/system"
)

type fakeExecutor struct {
	stdout string
}

func (f fakeExecutor) Run(ctx context.Context, cmd system.Command) (system.Result, error) {
	return system.Result{Stdout: f.stdout, ExitCode: 0}, nil
}

func TestDetectorReadsTrialSerial(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultRuntimeConfig()
	cfg.LSWS.LicenseSerialFile = filepath.Join(root, "serial.no")
	if err := os.WriteFile(cfg.LSWS.LicenseSerialFile, []byte("TRIAL-123"), 0o644); err != nil {
		t.Fatalf("write serial: %v", err)
	}

	info := lsws.NewDetector(cfg, fakeExecutor{}).Detect(context.Background())
	if info.Mode != "trial" {
		t.Fatalf("expected trial mode, got %s", info.Mode)
	}
}

func TestRendererMatchesGolden(t *testing.T) {
	cfg := config.DefaultRuntimeConfig()
	renderer := lsws.NewRenderer(cfg, lsws.LicenseInfo{Mode: "trial", Source: "test"})
	site := model.Site{
		Name:         "example.com",
		Backend:      "lsws",
		DocumentRoot: "/data/www/example.com",
		IndexFiles:   []string{"index.php", "index.html"},
		Domain: model.DomainBinding{
			ServerName: "example.com",
			Aliases:    []string{"www.example.com"},
		},
		TLS: model.TLSConfig{
			Enabled:         true,
			CertificateFile: "/etc/pki/tls/certs/example.com.crt",
			CertificateKey:  "/etc/pki/tls/private/example.com.key",
		},
		PHP: model.PHPRuntimeBinding{
			Enabled: true,
			Version: "8.4",
			Handler: "lsphp",
		},
		RewriteRules: []model.RewriteRule{
			{Pattern: "^/old$", Substitution: "/new", Flags: []string{"R=302", "L"}},
		},
		ReverseProxyRules: []model.ReverseProxyRule{
			{PathPrefix: "/api/", Upstream: "http://127.0.0.1:9000/", PreserveHost: true},
		},
		LSWS: &model.LSWSOptions{
			CustomDirectives: []string{"CacheEngine on"},
		},
		Logs: model.LogConfig{
			AccessLog: "/var/log/llstack/sites/example.com.access.log",
			ErrorLog:  "/var/log/llstack/sites/example.com.error.log",
		},
	}

	result, err := renderer.RenderSite(site, render.SiteRenderOptions{
		VHostPath:        "/usr/local/lsws/conf/llstack/includes/example.com.conf",
		ParityReportPath: "/var/lib/llstack/state/parity/example.com.lsws.json",
	})
	if err != nil {
		t.Fatalf("render site: %v", err)
	}
	if len(result.Assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(result.Assets))
	}

	checkGolden(t, filepath.Join("..", "..", "..", "..", "testdata", "golden", "lsws", "basic_vhost.conf"), string(result.Assets[0].Content))
	checkGolden(t, filepath.Join("..", "..", "..", "..", "testdata", "golden", "lsws", "basic_parity.json"), string(result.Assets[1].Content))

	if result.Capabilities == nil || result.Capabilities.LicenseMode != "trial" {
		t.Fatalf("expected trial capability snapshot, got %#v", result.Capabilities)
	}
}

func checkGolden(t *testing.T, path, got string) {
	t.Helper()
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if got != string(want) {
		t.Fatalf("golden mismatch for %s\nwant:\n%s\ngot:\n%s", path, string(want), got)
	}
}
