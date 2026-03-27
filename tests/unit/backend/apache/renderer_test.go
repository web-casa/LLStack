package apache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/web-casa/llstack/internal/backend/apache"
	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/core/render"
)

func TestRendererMatchesGolden(t *testing.T) {
	renderer := apache.NewRenderer()
	site := model.Site{
		Name:         "example.com",
		Backend:      "apache",
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
			Handler: "php-fpm",
			Socket:  "/run/php-fpm/www.sock",
		},
		RewriteRules: []model.RewriteRule{
			{Pattern: "^/old$", Substitution: "/new", Flags: []string{"R=302", "L"}},
		},
		ReverseProxyRules: []model.ReverseProxyRule{
			{PathPrefix: "/api/", Upstream: "http://127.0.0.1:9000/", PreserveHost: true},
		},
		Logs: model.LogConfig{
			AccessLog: "/var/log/llstack/sites/example.com.access.log",
			ErrorLog:  "/var/log/llstack/sites/example.com.error.log",
		},
	}

	result, err := renderer.RenderSite(site, render.SiteRenderOptions{
		VHostPath: "/etc/httpd/conf.d/llstack/sites/example.com.conf",
	})
	if err != nil {
		t.Fatalf("render site: %v", err)
	}
	if len(result.Assets) != 1 {
		t.Fatalf("expected one rendered asset, got %d", len(result.Assets))
	}

	goldenPath := filepath.Join("..", "..", "..", "..", "testdata", "golden", "apache", "basic_vhost.conf")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}

	got := string(result.Assets[0].Content)
	if got != string(want) {
		t.Fatalf("rendered vhost mismatch\nwant:\n%s\ngot:\n%s", string(want), got)
	}
}
