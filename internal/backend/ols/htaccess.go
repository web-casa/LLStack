package ols

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HtaccessCheckResult captures the compatibility analysis of a .htaccess file.
type HtaccessCheckResult struct {
	Site        string               `json:"site"`
	Path        string               `json:"htaccess_path"`
	Translated  []HtaccessDirective  `json:"translated,omitempty"`
	Warnings    []HtaccessDirective  `json:"warnings,omitempty"`
	Compatible  []HtaccessDirective  `json:"compatible,omitempty"`
}

// HtaccessDirective represents a single .htaccess directive analysis.
type HtaccessDirective struct {
	Line       int    `json:"line"`
	Directive  string `json:"directive"`
	Status     string `json:"status"` // compatible, convertible, unsupported
	Target     string `json:"target,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// CheckHtaccess scans a site's .htaccess for OLS compatibility issues.
func CheckHtaccess(siteName, docroot string) (HtaccessCheckResult, error) {
	htPath := filepath.Join(docroot, ".htaccess")
	result := HtaccessCheckResult{
		Site: siteName,
		Path: htPath,
	}

	raw, err := os.ReadFile(htPath)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil // no .htaccess, nothing to check
		}
		return result, err
	}

	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lineNum := i + 1
		directive := classifyDirective(trimmed)
		directive.Line = lineNum

		switch directive.Status {
		case "convertible":
			result.Translated = append(result.Translated, directive)
		case "unsupported":
			result.Warnings = append(result.Warnings, directive)
		default:
			result.Compatible = append(result.Compatible, directive)
		}
	}

	return result, nil
}

func classifyDirective(line string) HtaccessDirective {
	lower := strings.ToLower(line)

	// PHP directives: OLS doesn't support in .htaccess
	if strings.HasPrefix(lower, "php_value") || strings.HasPrefix(lower, "php_flag") ||
		strings.HasPrefix(lower, "php_admin_value") || strings.HasPrefix(lower, "php_admin_flag") {
		return HtaccessDirective{
			Directive:  line,
			Status:     "convertible",
			Target:     ".user.ini",
			Suggestion: "OLS does not support php_value/php_flag in .htaccess; convert to .user.ini",
		}
	}

	// RewriteBase: not supported in OLS
	if strings.HasPrefix(lower, "rewritebase") {
		return HtaccessDirective{
			Directive:  line,
			Status:     "unsupported",
			Target:     "OLS Context",
			Suggestion: "OLS does not support RewriteBase; use OLS Context configuration instead",
		}
	}

	// IfModule (non-LiteSpeed): may not work
	if strings.HasPrefix(lower, "<ifmodule") && !strings.Contains(lower, "litespeed") {
		return HtaccessDirective{
			Directive:  line,
			Status:     "unsupported",
			Suggestion: "OLS only supports <IfModule LiteSpeed>; other IfModule blocks may be ignored",
		}
	}

	// FilesMatch with rewrite: OLS doesn't trigger rewrite inside FilesMatch
	if strings.HasPrefix(lower, "<filesmatch") {
		return HtaccessDirective{
			Directive:  line,
			Status:     "unsupported",
			Suggestion: "OLS does not trigger rewrite rules inside <FilesMatch> blocks",
		}
	}

	// Rewrite rules: compatible (OLS supports mod_rewrite syntax)
	if strings.HasPrefix(lower, "rewriterule") || strings.HasPrefix(lower, "rewritecond") ||
		strings.HasPrefix(lower, "rewriteengine") {
		return HtaccessDirective{
			Directive: line,
			Status:    "compatible",
		}
	}

	// Common safe directives
	for _, safe := range []string{"errordocument", "options", "deny", "allow", "order",
		"header", "requestheader", "expiresactive", "expiresbytype", "setenvif",
		"addtype", "addhandler", "directoryindex", "setenv"} {
		if strings.HasPrefix(lower, safe) {
			return HtaccessDirective{
				Directive: line,
				Status:    "compatible",
			}
		}
	}

	// Default: mark as compatible (OLS reads most .htaccess directives)
	return HtaccessDirective{
		Directive: line,
		Status:    "compatible",
	}
}

// CompileHtaccess converts incompatible .htaccess directives to OLS-compatible format.
// When apply=true, modifies .htaccess and writes .user.ini.
func CompileHtaccess(siteName, docroot string, apply bool) (HtaccessCheckResult, error) {
	result, err := CheckHtaccess(siteName, docroot)
	if err != nil {
		return result, err
	}

	if !apply || len(result.Translated) == 0 {
		return result, nil
	}

	htPath := filepath.Join(docroot, ".htaccess")
	raw, err := os.ReadFile(htPath)
	if err != nil {
		return result, err
	}

	// Collect PHP directives for .user.ini
	var userIniLines []string
	htLines := strings.Split(string(raw), "\n")

	for _, directive := range result.Translated {
		if directive.Target == ".user.ini" {
			// Parse php_value/php_flag to .user.ini format
			converted := convertPHPDirective(directive.Directive)
			if converted != "" {
				userIniLines = append(userIniLines, converted)
			}
			// Comment out in .htaccess
			if directive.Line > 0 && directive.Line <= len(htLines) {
				htLines[directive.Line-1] = "# Converted by LLStack to .user.ini: " + htLines[directive.Line-1]
			}
		}
	}

	// Write .user.ini
	if len(userIniLines) > 0 {
		userIniPath := filepath.Join(docroot, ".user.ini")
		existing, _ := os.ReadFile(userIniPath)
		newContent := string(existing)
		if !strings.Contains(newContent, "; Converted by LLStack") {
			newContent += "\n; Converted by LLStack from .htaccess\n"
		}
		for _, line := range userIniLines {
			if !strings.Contains(newContent, strings.Split(line, " = ")[0]+" =") {
				newContent += line + "\n"
			}
		}
		if err := os.WriteFile(userIniPath, []byte(newContent), 0o644); err != nil {
			return result, fmt.Errorf("write .user.ini: %w", err)
		}
	}

	// Write modified .htaccess
	if err := os.WriteFile(htPath, []byte(strings.Join(htLines, "\n")), 0o644); err != nil {
		return result, fmt.Errorf("write .htaccess: %w", err)
	}

	return result, nil
}

func convertPHPDirective(line string) string {
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return ""
	}
	// php_value memory_limit 512M → memory_limit = 512M
	// php_flag display_errors Off → display_errors = Off
	return fmt.Sprintf("%s = %s", parts[1], strings.Join(parts[2:], " "))
}
