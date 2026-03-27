package ssl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/system"
)

// CertbotInfo captures the resolved certbot binary path.
type CertbotInfo struct {
	Binary string
	Found  bool
	Source string
}

// CertbotRequest captures the minimum inputs for a webroot ACME issuance.
type CertbotRequest struct {
	Domain  string
	Docroot string
	Email   string
}

// DetectCertbot resolves the preferred certbot binary using distro path candidates first.
func DetectCertbot(cfg config.RuntimeConfig) CertbotInfo {
	candidates := cfg.SSL.CertbotCandidates
	if len(candidates) == 0 {
		candidates = []string{"/usr/bin/certbot", "/bin/certbot", "/usr/local/bin/certbot", "certbot"}
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if filepath.IsAbs(candidate) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return CertbotInfo{Binary: candidate, Found: true, Source: "filesystem"}
			}
			continue
		}
		if resolved, err := exec.LookPath(candidate); err == nil {
			return CertbotInfo{Binary: resolved, Found: true, Source: "PATH"}
		}
	}
	return CertbotInfo{}
}

// BuildCertbotCommand builds the LLStack-managed webroot certbot command.
func BuildCertbotCommand(info CertbotInfo, req CertbotRequest) (system.Command, error) {
	if !info.Found || info.Binary == "" {
		return system.Command{}, fmt.Errorf("certbot binary not detected")
	}
	if req.Domain == "" {
		return system.Command{}, fmt.Errorf("domain is required")
	}
	if req.Docroot == "" {
		return system.Command{}, fmt.Errorf("docroot is required")
	}

	args := []string{
		"certonly",
		"--webroot",
		"-w", req.Docroot,
		"-d", req.Domain,
		"--agree-tos",
		"--non-interactive",
		"--keep-until-expiring",
		"--preferred-challenges", "http",
	}
	if req.Email != "" {
		args = append(args, "-m", req.Email)
	} else {
		args = append(args, "--register-unsafely-without-email")
	}
	return system.Command{Name: info.Binary, Args: args}, nil
}
