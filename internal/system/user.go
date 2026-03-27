package system

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os/exec"
	"os/user"
	"regexp"
	"strings"
)

const (
	maxUsernameLen = 12
	hashSuffixLen  = 4
	prefixMaxLen   = maxUsernameLen - hashSuffixLen - 1 // 7
)

var unsafeChars = regexp.MustCompile(`[^a-z0-9]`)

// SiteUsername generates a Linux username from a domain name.
// Format: {prefix}_{hash4}, max 12 characters.
// Example: wp.example.com → wp_a3f2
func SiteUsername(domain string) string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return ""
	}

	// Extract prefix: first label before '.'
	prefix := domain
	if idx := strings.Index(domain, "."); idx > 0 {
		prefix = domain[:idx]
	}

	// Sanitize: only lowercase alphanumeric
	prefix = unsafeChars.ReplaceAllString(prefix, "")
	if prefix == "" {
		prefix = "site"
	}

	// Truncate prefix
	if len(prefix) > prefixMaxLen {
		prefix = prefix[:prefixMaxLen]
	}

	// Hash suffix from full domain
	sum := sha256.Sum256([]byte(domain))
	hash := fmt.Sprintf("%x", sum[:2]) // 4 hex chars

	return prefix + "_" + hash[:hashSuffixLen]
}

// SiteUserExists checks if a Linux user exists.
func SiteUserExists(username string) bool {
	_, err := user.Lookup(username)
	return err == nil
}

// CreateSiteUser creates a system user for a site with nologin shell.
func CreateSiteUser(ctx context.Context, exec Executor, username string, homeDir string) error {
	if SiteUserExists(username) {
		return nil // idempotent
	}

	// Create group first
	result, err := exec.Run(ctx, Command{
		Name: "groupadd",
		Args: []string{"-r", username},
	})
	if err != nil {
		return fmt.Errorf("groupadd %s: %w", username, err)
	}
	if result.ExitCode != 0 && !strings.Contains(result.Stderr, "already exists") {
		return fmt.Errorf("groupadd %s: exit %d: %s", username, result.ExitCode, result.Stderr)
	}

	// Create user
	result, err = exec.Run(ctx, Command{
		Name: "useradd",
		Args: []string{
			"-r",
			"-s", "/sbin/nologin",
			"-d", homeDir,
			"-g", username,
			"-M", // don't create home dir (site manager handles this)
			username,
		},
	})
	if err != nil {
		return fmt.Errorf("useradd %s: %w", username, err)
	}
	if result.ExitCode != 0 && !strings.Contains(result.Stderr, "already exists") {
		return fmt.Errorf("useradd %s: exit %d: %s", username, result.ExitCode, result.Stderr)
	}

	return nil
}

// DeleteSiteUser removes a site user and its primary group.
func DeleteSiteUser(ctx context.Context, exec Executor, username string) error {
	if !SiteUserExists(username) {
		return nil // idempotent
	}

	result, err := exec.Run(ctx, Command{
		Name: "userdel",
		Args: []string{"-f", username},
	})
	if err != nil {
		return fmt.Errorf("userdel %s: %w", username, err)
	}
	if result.ExitCode != 0 && !strings.Contains(result.Stderr, "does not exist") {
		return fmt.Errorf("userdel %s: exit %d: %s", username, result.ExitCode, result.Stderr)
	}

	return nil
}

// AddUserToGroup adds a user to a supplementary group (for per-site group model).
func AddUserToGroup(ctx context.Context, exec Executor, user string, group string) error {
	result, err := exec.Run(ctx, Command{
		Name: "usermod",
		Args: []string{"-aG", group, user},
	})
	if err != nil {
		return fmt.Errorf("usermod -aG %s %s: %w", group, user, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("usermod -aG %s %s: exit %d: %s", group, user, result.ExitCode, result.Stderr)
	}
	return nil
}

// RemoveUserFromGroup removes a user from a group.
func RemoveUserFromGroup(ctx context.Context, exec Executor, user string, group string) error {
	result, err := exec.Run(ctx, Command{
		Name: "gpasswd",
		Args: []string{"-d", user, group},
	})
	if err != nil {
		return fmt.Errorf("gpasswd -d %s %s: %w", user, group, err)
	}
	if result.ExitCode != 0 && !strings.Contains(result.Stderr, "not a member") {
		return fmt.Errorf("gpasswd -d %s %s: exit %d: %s", user, group, result.ExitCode, result.Stderr)
	}
	return nil
}

// WebServerUser returns the default web server process user for a backend.
func WebServerUser(backend string) string {
	switch backend {
	case "apache":
		return "apache"
	case "ols", "lsws":
		return "nobody"
	default:
		return "apache"
	}
}

// HasCommand checks if a command exists in PATH.
func HasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
