package system_test

import (
	"testing"

	"github.com/web-casa/llstack/internal/system"
)

func TestSiteUsernameBasicDomains(t *testing.T) {
	cases := []struct {
		domain string
		prefix string // expected prefix before _
	}{
		{"wp.example.com", "wp"},
		{"example.com", "example"},
		{"app.test.io", "app"},
		{"a.com", "a"},
		{"my-site.example.com", "mysite"},
	}
	for _, tc := range cases {
		name := system.SiteUsername(tc.domain)
		if name == "" {
			t.Fatalf("SiteUsername(%q) returned empty", tc.domain)
		}
		if len(name) > 12 {
			t.Fatalf("SiteUsername(%q) = %q (len %d), exceeds 12 chars", tc.domain, name, len(name))
		}
		if name[:len(tc.prefix)] != tc.prefix {
			t.Fatalf("SiteUsername(%q) = %q, expected prefix %q", tc.domain, name, tc.prefix)
		}
		// Must contain underscore separator
		if name[len(name)-5] != '_' {
			t.Fatalf("SiteUsername(%q) = %q, expected _hash4 suffix", tc.domain, name)
		}
	}
}

func TestSiteUsernameLongDomain(t *testing.T) {
	name := system.SiteUsername("my-very-long-subdomain.example.com")
	if len(name) > 12 {
		t.Fatalf("SiteUsername for long domain = %q (len %d), exceeds 12", name, len(name))
	}
}

func TestSiteUsernameUniqueness(t *testing.T) {
	a := system.SiteUsername("site1.example.com")
	b := system.SiteUsername("site2.example.com")
	if a == b {
		t.Fatalf("different domains produced same username: %q", a)
	}
}

func TestSiteUsernameEmpty(t *testing.T) {
	name := system.SiteUsername("")
	if name != "" {
		t.Fatalf("SiteUsername(\"\") should return empty, got %q", name)
	}
}

func TestSiteUsernameIdempotent(t *testing.T) {
	a := system.SiteUsername("test.example.com")
	b := system.SiteUsername("test.example.com")
	if a != b {
		t.Fatalf("SiteUsername not idempotent: %q vs %q", a, b)
	}
}
