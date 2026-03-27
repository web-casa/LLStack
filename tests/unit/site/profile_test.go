package site_test

import (
	"testing"

	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/site"
)

func TestApplyStaticProfileDisablesPHP(t *testing.T) {
	spec := model.Site{
		Name: "static.example.com",
		Domain: model.DomainBinding{
			ServerName: "static.example.com",
		},
		PHP: model.PHPRuntimeBinding{
			Enabled: true,
			Version: "8.3",
		},
	}

	if err := site.ApplyProfile(&spec, site.ProfileStatic, ""); err != nil {
		t.Fatalf("apply profile: %v", err)
	}
	if spec.PHP.Enabled {
		t.Fatalf("expected static profile to disable php, got %#v", spec.PHP)
	}
	if spec.Profile != site.ProfileStatic {
		t.Fatalf("unexpected profile: %s", spec.Profile)
	}
}

func TestApplyReverseProxyProfileRequiresUpstream(t *testing.T) {
	spec := model.Site{
		Name: "proxy.example.com",
		Domain: model.DomainBinding{
			ServerName: "proxy.example.com",
		},
	}

	if err := site.ApplyProfile(&spec, site.ProfileReverseProxy, ""); err == nil {
		t.Fatal("expected reverse-proxy profile to require upstream")
	}
	if err := site.ApplyProfile(&spec, site.ProfileReverseProxy, "http://127.0.0.1:8080"); err != nil {
		t.Fatalf("apply reverse-proxy profile: %v", err)
	}
	if len(spec.ReverseProxyRules) != 1 || spec.ReverseProxyRules[0].Upstream != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected reverse proxy rules: %#v", spec.ReverseProxyRules)
	}
}
