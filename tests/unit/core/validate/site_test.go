package validate_test

import (
	"testing"

	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/core/validate"
)

func TestSiteValidationRejectsRelativeDocroot(t *testing.T) {
	site := model.Site{
		Name:         "example.com",
		Backend:      "apache",
		DocumentRoot: "relative/path",
		IndexFiles:   []string{"index.php"},
		Domain: model.DomainBinding{
			ServerName: "example.com",
		},
		PHP: model.PHPRuntimeBinding{
			Enabled: true,
			Handler: "php-fpm",
			Socket:  "/run/php-fpm/www.sock",
		},
	}

	if err := validate.Site(site); err == nil {
		t.Fatal("expected validation error for relative document root")
	}
}
