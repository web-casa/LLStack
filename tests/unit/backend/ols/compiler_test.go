package ols_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/web-casa/llstack/internal/backend/ols"
	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/core/render"
)

func TestCompilerMatchesGolden(t *testing.T) {
	cfg := config.DefaultRuntimeConfig()
	compiler := ols.NewCompiler(cfg)
	site := model.Site{
		Name:         "example.com",
		Backend:      "ols",
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
			Version: "8.3",
			Handler: "lsphp",
		},
		RewriteRules: []model.RewriteRule{
			{Pattern: "^/old$", Substitution: "/new", Flags: []string{"R=302", "L"}},
		},
		ReverseProxyRules: []model.ReverseProxyRule{
			{PathPrefix: "/api/", Upstream: "http://127.0.0.1:9000/", PreserveHost: true},
		},
		HeaderRules: []model.HeaderRule{
			{Name: "X-Test", Value: "1", Action: "set"},
		},
		AccessRules: []model.AccessControlRule{
			{Path: "/admin", Action: "deny", Source: "all"},
		},
		Logs: model.LogConfig{
			AccessLog: "/var/log/llstack/sites/example.com.access.log",
			ErrorLog:  "/var/log/llstack/sites/example.com.error.log",
		},
	}

	result, err := compiler.RenderSite(site, render.SiteRenderOptions{
		VHostPath:        "/usr/local/lsws/conf/vhosts/example.com/vhconf.conf",
		ListenerMapPath:  "/usr/local/lsws/conf/llstack/listeners/example.com.map",
		ParityReportPath: "/var/lib/llstack/state/parity/example.com.ols.json",
	})
	if err != nil {
		t.Fatalf("compile site: %v", err)
	}
	if len(result.Assets) != 3 {
		t.Fatalf("expected 3 assets, got %d", len(result.Assets))
	}

	checkGolden(t, filepath.Join("..", "..", "..", "..", "testdata", "golden", "ols", "basic_vhconf.conf"), string(result.Assets[0].Content))
	checkGolden(t, filepath.Join("..", "..", "..", "..", "testdata", "golden", "ols", "basic_listener.map"), string(result.Assets[1].Content))
	checkGolden(t, filepath.Join("..", "..", "..", "..", "testdata", "golden", "ols", "basic_parity.json"), string(result.Assets[2].Content))
}

func TestCompilerProducesExpectedAssetPaths(t *testing.T) {
	cfg := config.DefaultRuntimeConfig()
	compiler := ols.NewCompiler(cfg)
	site := model.Site{
		Name:         "asset.example.com",
		Backend:      "ols",
		DocumentRoot: "/data/www/asset.example.com",
		IndexFiles:   []string{"index.php"},
		Domain: model.DomainBinding{
			ServerName: "asset.example.com",
		},
		Logs: model.LogConfig{
			AccessLog: "/var/log/llstack/sites/asset.example.com.access.log",
			ErrorLog:  "/var/log/llstack/sites/asset.example.com.error.log",
		},
	}

	result, err := compiler.RenderSite(site, render.SiteRenderOptions{
		VHostPath:        "/tmp/vhosts/asset.example.com/vhconf.conf",
		ListenerMapPath:  "/tmp/listeners/asset.example.com.map",
		ParityReportPath: "/tmp/parity/asset.example.com.ols.json",
	})
	if err != nil {
		t.Fatalf("compile site: %v", err)
	}

	got := []string{result.Assets[0].Path, result.Assets[1].Path, result.Assets[2].Path}
	want := []string{
		"/tmp/vhosts/asset.example.com/vhconf.conf",
		"/tmp/listeners/asset.example.com.map",
		"/tmp/parity/asset.example.com.ols.json",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("asset %d path mismatch: want %s, got %s", i, want[i], got[i])
		}
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
