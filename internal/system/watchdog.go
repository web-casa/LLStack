package system

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// WatchdogConfig controls service restart and alerting.
type WatchdogConfig struct {
	Services   []string // service names to monitor
	WebhookURL string   // optional webhook for alerts
}

// EnableServiceRestart configures systemd to auto-restart failed services.
func EnableServiceRestart(ctx context.Context, exec Executor, services []string) error {
	for _, service := range services {
		overrideDir := fmt.Sprintf("/etc/systemd/system/%s.d", service)
		os.MkdirAll(overrideDir, 0o755)

		override := `# LLStack service watchdog
[Service]
Restart=on-failure
RestartSec=5s

[Unit]
StartLimitIntervalSec=60
StartLimitBurst=3
`
		if err := os.WriteFile(fmt.Sprintf("%s/llstack-restart.conf", overrideDir), []byte(override), 0o644); err != nil {
			return fmt.Errorf("write systemd override for %s: %w", service, err)
		}
	}

	exec.Run(ctx, Command{Name: "systemctl", Args: []string{"daemon-reload"}})
	return nil
}

// DisableServiceRestart removes auto-restart configuration.
func DisableServiceRestart(ctx context.Context, exec Executor, services []string) error {
	for _, service := range services {
		os.Remove(fmt.Sprintf("/etc/systemd/system/%s.d/llstack-restart.conf", service))
	}
	exec.Run(ctx, Command{Name: "systemctl", Args: []string{"daemon-reload"}})
	return nil
}

// WatchdogTimerUnit returns a systemd timer for periodic health checks.
func WatchdogTimerUnit() string {
	return `[Unit]
Description=LLStack Service Watchdog Timer

[Timer]
OnBootSec=120
OnUnitActiveSec=60
AccuracySec=10s

[Install]
WantedBy=timers.target
`
}

// WatchdogServiceUnit returns the watchdog service unit.
func WatchdogServiceUnit(llstackBin string) string {
	return fmt.Sprintf(`[Unit]
Description=LLStack Service Watchdog

[Service]
Type=oneshot
ExecStart=%s doctor --json
`, llstackBin)
}

// AlertWebhook sends a simple webhook notification.
func AlertWebhook(ctx context.Context, exec Executor, webhookURL string, message string) {
	if strings.TrimSpace(webhookURL) == "" {
		return
	}
	payload := fmt.Sprintf(`{"text":"%s"}`, strings.ReplaceAll(message, `"`, `\"`))
	exec.Run(ctx, Command{
		Name: "curl",
		Args: []string{"-sS", "-X", "POST", "-H", "Content-Type: application/json", "-d", payload, webhookURL},
	})
}
