package views

import (
	"strings"

	"github.com/web-casa/llstack/internal/config"
)

// RenderLogs renders the logs page.
func RenderLogs(cfg config.RuntimeConfig) string {
	lines := []string{"Logs", ""}
	recent, err := readRecentLogLines(cfg, 8)
	if err != nil {
		return strings.Join([]string{"Logs", "", "error: " + err.Error()}, "\n")
	}
	if len(recent) == 0 {
		return strings.Join([]string{
			"Logs",
			"",
			"No managed site logs yet.",
			"Use `llstack site:logs <site>` for direct log access once a site is active.",
		}, "\n")
	}
	lines = append(lines, recent...)
	return strings.Join(lines, "\n")
}
