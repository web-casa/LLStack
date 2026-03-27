package lsws

import (
	"context"
	"os"
	"strings"

	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/system"
)

// LicenseInfo describes the detected LSWS license mode.
type LicenseInfo struct {
	Mode   string `json:"mode"`
	Source string `json:"source,omitempty"`
	Raw    string `json:"raw,omitempty"`
}

// Detector detects LSWS license mode.
type Detector struct {
	cfg  config.RuntimeConfig
	exec system.Executor
}

// NewDetector constructs a license detector.
func NewDetector(cfg config.RuntimeConfig, exec system.Executor) Detector {
	return Detector{cfg: cfg, exec: exec}
}

// Detect returns the best-effort license mode for LSWS.
func (d Detector) Detect(ctx context.Context) LicenseInfo {
	if info, ok := d.detectFromSerial(); ok {
		return info
	}
	if info, ok := d.detectFromCommand(ctx); ok {
		return info
	}
	return LicenseInfo{Mode: "unknown"}
}

func (d Detector) detectFromSerial() (LicenseInfo, bool) {
	raw, err := os.ReadFile(d.cfg.LSWS.LicenseSerialFile)
	if err != nil {
		return LicenseInfo{}, false
	}
	serial := strings.TrimSpace(string(raw))
	if serial == "" {
		return LicenseInfo{}, false
	}
	if strings.Contains(strings.ToLower(serial), "trial") {
		return LicenseInfo{Mode: "trial", Source: "serial_file", Raw: serial}, true
	}
	return LicenseInfo{Mode: "licensed", Source: "serial_file", Raw: serial}, true
}

func (d Detector) detectFromCommand(ctx context.Context) (LicenseInfo, bool) {
	if len(d.cfg.LSWS.DetectCmd) == 0 {
		return LicenseInfo{}, false
	}
	result, err := d.exec.Run(ctx, system.Command{
		Name: d.cfg.LSWS.DetectCmd[0],
		Args: d.cfg.LSWS.DetectCmd[1:],
	})
	if err != nil {
		return LicenseInfo{}, false
	}
	combined := strings.ToLower(result.Stdout + "\n" + result.Stderr)
	switch {
	case strings.Contains(combined, "trial"):
		return LicenseInfo{Mode: "trial", Source: "detect_cmd", Raw: strings.TrimSpace(combined)}, true
	case strings.Contains(combined, "serial"), strings.Contains(combined, "licensed"):
		return LicenseInfo{Mode: "licensed", Source: "detect_cmd", Raw: strings.TrimSpace(combined)}, true
	default:
		return LicenseInfo{}, false
	}
}
