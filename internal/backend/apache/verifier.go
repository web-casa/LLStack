package apache

import (
	"context"
	"fmt"

	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/system"
)

// Verifier performs Apache config tests and reloads.
type Verifier struct {
	cfg  config.RuntimeConfig
	exec system.Executor
}

// NewVerifier creates an Apache verifier.
func NewVerifier(cfg config.RuntimeConfig, exec system.Executor) Verifier {
	return Verifier{cfg: cfg, exec: exec}
}

// ConfigTest runs Apache config validation.
func (v Verifier) ConfigTest(ctx context.Context) error {
	return v.run(ctx, v.cfg.Apache.ConfigTestCmd)
}

// Reload reloads Apache.
func (v Verifier) Reload(ctx context.Context) error {
	return v.run(ctx, v.cfg.Apache.ReloadCmd)
}

// Restart restarts Apache.
func (v Verifier) Restart(ctx context.Context) error {
	return v.run(ctx, v.cfg.Apache.RestartCmd)
}

func (v Verifier) run(ctx context.Context, parts []string) error {
	if len(parts) == 0 {
		return nil
	}

	result, err := v.exec.Run(ctx, system.Command{
		Name: parts[0],
		Args: parts[1:],
	})
	if err != nil {
		return fmt.Errorf("%s: %w (%s)", parts[0], err, result.Stderr)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("%s exited with %d: %s", parts[0], result.ExitCode, result.Stderr)
	}
	return nil
}
