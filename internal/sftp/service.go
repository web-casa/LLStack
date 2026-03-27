package sftp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/web-casa/llstack/internal/system"
)

// Account represents a managed SFTP account.
type Account struct {
	Username  string    `json:"username"`
	Site      string    `json:"site"`
	HomeDir   string    `json:"home_dir"`
	ChrootDir string    `json:"chroot_dir"`
	Shell     string    `json:"shell"`
	CreatedAt time.Time `json:"created_at"`
}

// Manager manages per-site SFTP accounts.
type Manager struct {
	manifestDir string
	exec        system.Executor
}

// NewManager creates an SFTP manager.
func NewManager(manifestDir string, exec system.Executor) Manager {
	return Manager{manifestDir: manifestDir, exec: exec}
}

// CreateOptions controls SFTP account creation.
type CreateOptions struct {
	Site     string
	Docroot  string // actual site docroot (may differ from /data/www/<site>)
	Username string
	Password string
	AutoPass bool
	SSHKey   string // optional public key
}

// Create creates an SFTP-only account with proper chroot isolation.
//
// OpenSSH ChrootDirectory requires the chroot target to be root-owned and not writable
// by any other user. We use a two-level structure:
//   - ChrootDirectory = /data/www (root:root 755, satisfies OpenSSH requirement)
//   - The SFTP user's home is set to /data/www/<site>
//   - After chroot, the user lands in /<site>/ and can access their files
func (m Manager) Create(ctx context.Context, opts CreateOptions) (Account, string, error) {
	if opts.Site == "" {
		return Account{}, "", fmt.Errorf("site name is required")
	}

	username := opts.Username
	if username == "" {
		username = "sftp_" + system.SiteUsername(opts.Site)
	}
	if len(username) > 32 {
		username = username[:32]
	}

	password := opts.Password
	if password == "" || opts.AutoPass {
		b := make([]byte, 12)
		if _, err := rand.Read(b); err != nil {
			b = []byte("fallback-pass-12345678")
		}
		password = hex.EncodeToString(b)
	}

	siteUser := system.SiteUsername(opts.Site)
	chrootDir := "/data/www"                                  // root-owned, satisfies OpenSSH
	homeDir := filepath.Join(chrootDir, opts.Site)             // user sees this as / after chroot
	if opts.Docroot != "" && opts.Docroot != homeDir {
		homeDir = opts.Docroot
	}

	// Create system user
	result, err := m.exec.Run(ctx, system.Command{
		Name: "useradd",
		Args: []string{
			"-r", "-s", "/sbin/nologin",
			"-d", homeDir,
			"-g", siteUser,
			"-M",
			username,
		},
	})
	if err != nil && !strings.Contains(fmt.Sprintf("%v", err), "already exists") {
		return Account{}, "", fmt.Errorf("create SFTP user: %w", err)
	}
	if result.ExitCode != 0 && !strings.Contains(result.Stderr, "already exists") {
		return Account{}, "", fmt.Errorf("create SFTP user: %s", result.Stderr)
	}

	// Add user to llstack-sftp group
	grpResult, grpErr := m.exec.Run(ctx, system.Command{
		Name: "usermod", Args: []string{"-aG", "llstack-sftp", username},
	})
	if grpErr != nil || grpResult.ExitCode != 0 {
		return Account{}, "", fmt.Errorf("add user to llstack-sftp group failed: %s", grpResult.Stderr)
	}

	// Set password
	safeUser := strings.ReplaceAll(username, "'", "'\"'\"'")
	safePass := strings.ReplaceAll(password, "'", "'\"'\"'")
	passResult, passErr := m.exec.Run(ctx, system.Command{
		Name: "sh",
		Args: []string{"-c", fmt.Sprintf("printf '%%s:%%s\\n' '%s' '%s' | chpasswd", safeUser, safePass)},
	})
	if passErr != nil || passResult.ExitCode != 0 {
		return Account{}, "", fmt.Errorf("set password failed: %s", passResult.Stderr)
	}

	// Add SSH key if provided
	if opts.SSHKey != "" {
		sshDir := filepath.Join(homeDir, ".ssh")
		os.MkdirAll(sshDir, 0o700)
		authKeys := filepath.Join(sshDir, "authorized_keys")
		f, err := os.OpenFile(authKeys, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err == nil {
			fmt.Fprintln(f, opts.SSHKey)
			f.Close()
			m.exec.Run(ctx, system.Command{Name: "chown", Args: []string{"-R", username + ":" + siteUser, sshDir}})
		}
	}

	// Ensure chroot directory is root-owned (OpenSSH requirement)
	m.exec.Run(ctx, system.Command{Name: "chown", Args: []string{"root:root", chrootDir}})
	m.exec.Run(ctx, system.Command{Name: "chmod", Args: []string{"755", chrootDir}})

	// Configure sshd (idempotent)
	m.ensureSFTPConfig(ctx)

	account := Account{
		Username:  username,
		Site:      opts.Site,
		HomeDir:   homeDir,
		ChrootDir: chrootDir,
		Shell:     "/sbin/nologin",
		CreatedAt: time.Now().UTC(),
	}

	m.saveAccount(account)
	return account, password, nil
}

// List returns all managed SFTP accounts.
func (m Manager) List() ([]Account, error) {
	entries, err := os.ReadDir(m.manifestDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var accounts []Account
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(m.manifestDir, entry.Name()))
		if err != nil {
			continue
		}
		var account Account
		if json.Unmarshal(raw, &account) == nil {
			accounts = append(accounts, account)
		}
	}
	return accounts, nil
}

// Remove deletes an SFTP account.
func (m Manager) Remove(ctx context.Context, username string) error {
	m.exec.Run(ctx, system.Command{Name: "userdel", Args: []string{"-f", username}})
	os.Remove(filepath.Join(m.manifestDir, username+".json"))
	return nil
}

func (m Manager) ensureSFTPConfig(ctx context.Context) {
	sshdConfig := "/etc/ssh/sshd_config"
	raw, err := os.ReadFile(sshdConfig)
	if err != nil {
		return
	}
	content := string(raw)

	if strings.Contains(content, "# LLStack SFTP") {
		return
	}

	// ChrootDirectory /data/www — root-owned, OpenSSH-compliant
	// After chroot, user lands in /<site-name>/ based on their home directory
	sftpBlock := `
# LLStack SFTP chroot configuration
Match Group llstack-sftp
    ForceCommand internal-sftp
    ChrootDirectory /data/www
    AllowTcpForwarding no
    X11Forwarding no
    PermitTunnel no
`
	m.exec.Run(ctx, system.Command{Name: "groupadd", Args: []string{"-f", "llstack-sftp"}})

	f, err := os.OpenFile(sshdConfig, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	fmt.Fprintln(f, sftpBlock)
	f.Close()

	// Validate sshd config before reloading
	testResult, _ := m.exec.Run(ctx, system.Command{Name: "sshd", Args: []string{"-t"}})
	if testResult.ExitCode == 0 {
		m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"reload", "sshd"}})
	} else {
		// Config invalid — revert
		os.WriteFile(sshdConfig, raw, 0o644)
	}
}

func (m Manager) saveAccount(account Account) {
	os.MkdirAll(m.manifestDir, 0o755)
	raw, _ := json.MarshalIndent(account, "", "  ")
	os.WriteFile(filepath.Join(m.manifestDir, account.Username+".json"), raw, 0o644)
}
