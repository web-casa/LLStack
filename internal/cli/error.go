package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// FormatError adds concise next-step guidance for common operator-facing errors.
func FormatError(err error) string {
	if err == nil {
		return ""
	}

	message := err.Error()
	hints := hintsForError(err)
	if len(hints) == 0 {
		return message
	}

	var builder strings.Builder
	builder.WriteString(message)
	builder.WriteString("\nnext steps:\n")
	for _, hint := range hints {
		builder.WriteString(" - ")
		builder.WriteString(hint)
		builder.WriteString("\n")
	}
	return strings.TrimRight(builder.String(), "\n")
}

func hintsForError(err error) []string {
	message := err.Error()
	switch {
	case strings.Contains(message, "managed site") && strings.Contains(message, "not found"):
		return []string{
			"run `llstack site:list` to see currently managed sites",
			"create a site first with `llstack site:create <server-name> --dry-run` if this host is new",
		}
	case strings.Contains(message, "runtime") && strings.Contains(message, "not found"):
		return []string{
			"inspect installed runtimes with `llstack php:list`",
			"install a runtime first with `llstack php:install <version> --dry-run`",
		}
	case strings.Contains(message, "rollback history entry not found"):
		return []string{
			"inspect rollback history with `llstack rollback:list`",
			"run `llstack doctor --bundle` if history files were removed unexpectedly",
		}
	case strings.Contains(message, "docker daemon is unavailable"):
		return []string{
			"check access to `/var/run/docker.sock` for the current user",
			"run `docker info` directly to confirm daemon access before retrying",
		}
	}

	var pathError *os.PathError
	if errors.As(err, &pathError) {
		return []string{
			fmt.Sprintf("verify the path exists and is readable: %s", pathError.Path),
		}
	}

	return nil
}
