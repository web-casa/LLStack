package ols_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	coremodel "github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/backend/ols"
)

func TestRegisterSiteAddsVhostAndMap(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "httpd_config.conf")
	baseConfig := `serverRoot /usr/local/lsws

listener Default {
  address *:80
  secure 0
}
`
	os.WriteFile(configPath, []byte(baseConfig), 0o644)

	site := coremodel.Site{
		Name:         "test.example.com",
		DocumentRoot: "/data/www/test.example.com",
		Domain:       coremodel.DomainBinding{ServerName: "test.example.com"},
	}
	err := ols.RegisterSiteInMainConfig(configPath, site, "/usr/local/lsws/conf/vhosts/test.example.com/vhconf.conf")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	s := string(content)

	if !strings.Contains(s, "LLSTACK_VHOST_BEGIN test.example.com") {
		t.Fatal("expected LLSTACK_VHOST_BEGIN marker")
	}
	if !strings.Contains(s, "LLSTACK_MAP_BEGIN test.example.com") {
		t.Fatal("expected LLSTACK_MAP_BEGIN marker")
	}
	if !strings.Contains(s, "virtualhost test.example.com") {
		t.Fatal("expected virtualhost block")
	}
}

func TestUnregisterSiteRemovesBlocks(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "httpd_config.conf")
	baseConfig := `serverRoot /usr/local/lsws

listener Default {
  address *:80
# LLSTACK_MAP_BEGIN test.example.com
  map                     test.example.com test.example.com
# LLSTACK_MAP_END test.example.com
}

# LLSTACK_VHOST_BEGIN test.example.com
virtualhost test.example.com {
  vhRoot                  /data/www/test.example.com
  configFile              /some/path/vhconf.conf
}
# LLSTACK_VHOST_END test.example.com
`
	os.WriteFile(configPath, []byte(baseConfig), 0o644)

	err := ols.UnregisterSiteFromMainConfig(configPath, "test.example.com")
	if err != nil {
		t.Fatalf("unregister: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	s := string(content)

	if strings.Contains(s, "LLSTACK_VHOST_BEGIN") {
		t.Fatal("expected vhost block removed")
	}
	if strings.Contains(s, "LLSTACK_MAP_BEGIN") {
		t.Fatal("expected map block removed")
	}
	if strings.Contains(s, "virtualhost test.example.com") {
		t.Fatal("expected virtualhost removed")
	}
}

func TestRegisterIsIdempotent(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "httpd_config.conf")
	os.WriteFile(configPath, []byte("listener Default {\n  address *:80\n}\n"), 0o644)

	site := coremodel.Site{
		Name:         "idem.example.com",
		DocumentRoot: "/data/www/idem.example.com",
		Domain:       coremodel.DomainBinding{ServerName: "idem.example.com"},
	}
	ols.RegisterSiteInMainConfig(configPath, site, "/path/vhconf.conf")
	ols.RegisterSiteInMainConfig(configPath, site, "/path/vhconf.conf")

	content, _ := os.ReadFile(configPath)
	count := strings.Count(string(content), "LLSTACK_VHOST_BEGIN idem.example.com")
	if count != 1 {
		t.Fatalf("expected exactly 1 vhost block after double register, got %d", count)
	}
}
