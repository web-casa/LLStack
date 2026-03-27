package site

import (
	"fmt"
	"os"
	"strings"
)

// LogrotateConfig controls log rotation parameters.
type LogrotateConfig struct {
	RetainDays int
	Compress   bool
	MaxSizeMB  int
}

// WriteLogrotateConfig generates a logrotate config for a site.
func WriteLogrotateConfig(siteName, logDir string, cfg LogrotateConfig) error {
	if cfg.RetainDays <= 0 {
		cfg.RetainDays = 30
	}
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = 100
	}

	compress := ""
	if cfg.Compress {
		compress = "\n    compress\n    delaycompress"
	}

	content := fmt.Sprintf(`# Managed by LLStack for site %s
%s/*.log {
    daily
    rotate %d
    size %dM
    missingok
    notifempty
    create 0640 apache apache
    sharedscripts%s
    postrotate
        /bin/systemctl reload httpd > /dev/null 2>/dev/null || true
    endscript
}
`, siteName, logDir, cfg.RetainDays, cfg.MaxSizeMB, compress)

	safeName := strings.ReplaceAll(strings.ReplaceAll(siteName, ".", "-"), "/", "-")
	path := fmt.Sprintf("/etc/logrotate.d/llstack-%s", safeName)
	return os.WriteFile(path, []byte(content), 0o644)
}

// RemoveLogrotateConfig removes the logrotate config for a site.
func RemoveLogrotateConfig(siteName string) {
	safeName := strings.ReplaceAll(strings.ReplaceAll(siteName, ".", "-"), "/", "-")
	os.Remove(fmt.Sprintf("/etc/logrotate.d/llstack-%s", safeName))
}
