package ssl_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/web-casa/llstack/internal/config"
	sslprovider "github.com/web-casa/llstack/internal/ssl"
)

func TestDetectCertbotFindsConfiguredAbsolutePath(t *testing.T) {
	cfg := config.DefaultRuntimeConfig()
	mockCertbot := filepath.Join(t.TempDir(), "certbot")
	if err := os.WriteFile(mockCertbot, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write mock certbot: %v", err)
	}
	cfg.SSL.CertbotCandidates = []string{mockCertbot}

	info := sslprovider.DetectCertbot(cfg)
	if !info.Found || info.Binary != mockCertbot {
		t.Fatalf("expected configured certbot path to be detected, got %#v", info)
	}
}

func TestBuildCertbotCommand(t *testing.T) {
	command, err := sslprovider.BuildCertbotCommand(sslprovider.CertbotInfo{
		Binary: "/usr/bin/certbot",
		Found:  true,
	}, sslprovider.CertbotRequest{
		Domain:  "example.com",
		Docroot: "/data/www/example.com",
		Email:   "admin@example.com",
	})
	if err != nil {
		t.Fatalf("build certbot command: %v", err)
	}
	if command.Name != "/usr/bin/certbot" {
		t.Fatalf("unexpected binary: %#v", command)
	}
	if len(command.Args) == 0 || command.Args[0] != "certonly" {
		t.Fatalf("unexpected args: %#v", command.Args)
	}
	hasEmail := false
	for _, arg := range command.Args {
		if arg == "admin@example.com" {
			hasEmail = true
		}
	}
	if !hasEmail {
		t.Fatalf("expected email in certbot args, got %v", command.Args)
	}
}

func TestBuildCertbotCommandWithoutEmail(t *testing.T) {
	command, err := sslprovider.BuildCertbotCommand(sslprovider.CertbotInfo{
		Binary: "/usr/bin/certbot",
		Found:  true,
	}, sslprovider.CertbotRequest{
		Domain:  "example.com",
		Docroot: "/data/www/example.com",
	})
	if err != nil {
		t.Fatalf("build certbot command: %v", err)
	}
	hasUnsafe := false
	for _, arg := range command.Args {
		if arg == "--register-unsafely-without-email" {
			hasUnsafe = true
		}
	}
	if !hasUnsafe {
		t.Fatalf("expected --register-unsafely-without-email when no email, got %v", command.Args)
	}
}

func TestBuildCertbotCommandFailsWithoutBinary(t *testing.T) {
	_, err := sslprovider.BuildCertbotCommand(sslprovider.CertbotInfo{
		Found: false,
	}, sslprovider.CertbotRequest{
		Domain:  "example.com",
		Docroot: "/data/www/example.com",
	})
	if err == nil {
		t.Fatalf("expected error when certbot not found")
	}
	if !strings.Contains(err.Error(), "certbot binary not detected") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildCertbotCommandFailsWithoutDomain(t *testing.T) {
	_, err := sslprovider.BuildCertbotCommand(sslprovider.CertbotInfo{
		Binary: "/usr/bin/certbot",
		Found:  true,
	}, sslprovider.CertbotRequest{
		Docroot: "/data/www/example.com",
	})
	if err == nil {
		t.Fatalf("expected error when domain is missing")
	}
	if !strings.Contains(err.Error(), "domain is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildCertbotCommandFailsWithoutDocroot(t *testing.T) {
	_, err := sslprovider.BuildCertbotCommand(sslprovider.CertbotInfo{
		Binary: "/usr/bin/certbot",
		Found:  true,
	}, sslprovider.CertbotRequest{
		Domain: "example.com",
	})
	if err == nil {
		t.Fatalf("expected error when docroot is missing")
	}
	if !strings.Contains(err.Error(), "docroot is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDetectCertbotReturnsNotFoundWhenNoCandidates(t *testing.T) {
	cfg := config.DefaultRuntimeConfig()
	cfg.SSL.CertbotCandidates = []string{"/nonexistent/certbot"}
	info := sslprovider.DetectCertbot(cfg)
	if info.Found {
		t.Fatalf("expected certbot not found, got %#v", info)
	}
}

func TestBuildCertbotCommandIncludesWebroot(t *testing.T) {
	command, err := sslprovider.BuildCertbotCommand(sslprovider.CertbotInfo{
		Binary: "/usr/bin/certbot",
		Found:  true,
	}, sslprovider.CertbotRequest{
		Domain:  "test.example.com",
		Docroot: "/data/www/test.example.com",
		Email:   "ops@example.com",
	})
	if err != nil {
		t.Fatalf("build certbot command: %v", err)
	}
	hasWebroot := false
	hasDomain := false
	hasDocroot := false
	for i, arg := range command.Args {
		if arg == "--webroot" {
			hasWebroot = true
		}
		if arg == "-d" && i+1 < len(command.Args) && command.Args[i+1] == "test.example.com" {
			hasDomain = true
		}
		if arg == "-w" && i+1 < len(command.Args) && command.Args[i+1] == "/data/www/test.example.com" {
			hasDocroot = true
		}
	}
	if !hasWebroot || !hasDomain || !hasDocroot {
		t.Fatalf("expected certbot command with webroot/domain/docroot, got %v", command.Args)
	}
}
