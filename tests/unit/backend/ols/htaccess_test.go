package ols_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/web-casa/llstack/internal/backend/ols"
)

func TestCheckHtaccessDetectsPHPValue(t *testing.T) {
	docroot := t.TempDir()
	htaccess := `RewriteEngine On
RewriteRule ^index\.php$ - [L]
php_value memory_limit 512M
php_flag display_errors Off
`
	os.WriteFile(filepath.Join(docroot, ".htaccess"), []byte(htaccess), 0o644)

	result, err := ols.CheckHtaccess("test.com", docroot)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(result.Translated) != 2 {
		t.Fatalf("expected 2 convertible directives, got %d", len(result.Translated))
	}
	if result.Translated[0].Target != ".user.ini" {
		t.Fatalf("expected .user.ini target, got %s", result.Translated[0].Target)
	}
}

func TestCheckHtaccessDetectsRewriteBase(t *testing.T) {
	docroot := t.TempDir()
	htaccess := `RewriteEngine On
RewriteBase /blog
RewriteRule ^index\.php$ - [L]
`
	os.WriteFile(filepath.Join(docroot, ".htaccess"), []byte(htaccess), 0o644)

	result, err := ols.CheckHtaccess("test.com", docroot)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 unsupported directive (RewriteBase), got %d", len(result.Warnings))
	}
}

func TestCheckHtaccessNoFile(t *testing.T) {
	result, err := ols.CheckHtaccess("test.com", t.TempDir())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(result.Translated) != 0 || len(result.Warnings) != 0 {
		t.Fatalf("expected no issues for missing .htaccess")
	}
}

func TestCompileHtaccessConvertsToUserIni(t *testing.T) {
	docroot := t.TempDir()
	htaccess := `php_value memory_limit 512M
php_flag display_errors Off
RewriteRule ^index\.php$ - [L]
`
	os.WriteFile(filepath.Join(docroot, ".htaccess"), []byte(htaccess), 0o644)

	result, err := ols.CompileHtaccess("test.com", docroot, true)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(result.Translated) != 2 {
		t.Fatalf("expected 2 translated, got %d", len(result.Translated))
	}

	// Verify .user.ini was created
	userIni, err := os.ReadFile(filepath.Join(docroot, ".user.ini"))
	if err != nil {
		t.Fatalf("read .user.ini: %v", err)
	}
	if len(userIni) == 0 {
		t.Fatal("expected non-empty .user.ini")
	}

	// Verify .htaccess was modified (commented out)
	modified, _ := os.ReadFile(filepath.Join(docroot, ".htaccess"))
	if !contains(string(modified), "# Converted by LLStack") {
		t.Fatal("expected .htaccess to have conversion comments")
	}
}

func TestCompileHtaccessDryRunDoesNotModify(t *testing.T) {
	docroot := t.TempDir()
	htaccess := `php_value memory_limit 256M
`
	os.WriteFile(filepath.Join(docroot, ".htaccess"), []byte(htaccess), 0o644)

	_, err := ols.CompileHtaccess("test.com", docroot, false)
	if err != nil {
		t.Fatalf("compile dry-run: %v", err)
	}

	// .user.ini should NOT exist
	if _, err := os.Stat(filepath.Join(docroot, ".user.ini")); err == nil {
		t.Fatal("expected no .user.ini in dry-run mode")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}
func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
