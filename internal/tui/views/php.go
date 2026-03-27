package views

import (
	"fmt"
	"strings"

	"github.com/web-casa/llstack/internal/config"
)

// RenderPHP renders the PHP page.
func RenderPHP(cfg config.RuntimeConfig) string {
	lines := []string{"PHP Versions", ""}
	runtimes, err := readPHPRuntimes(cfg)
	if err != nil {
		return strings.Join([]string{"PHP Versions", "", "error: " + err.Error()}, "\n")
	}
	if len(runtimes) == 0 {
		return strings.Join([]string{
			"PHP Versions",
			"",
			"No PHP runtimes installed yet.",
			"Use `llstack php:install 8.4` to add the first runtime.",
		}, "\n")
	}
	for _, runtime := range runtimes {
		lines = append(lines, fmt.Sprintf("%s  profile=%s  extensions=%s", runtime.Version, runtime.Profile, strings.Join(runtime.Extensions, ",")))
	}
	return strings.Join(lines, "\n")
}
