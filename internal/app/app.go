package app

import (
	"context"
	"fmt"
	"os"

	"github.com/web-casa/llstack/internal/buildinfo"
	"github.com/web-casa/llstack/internal/cli"
	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/logging"
	"github.com/web-casa/llstack/internal/system"
)

// Options controls the application bootstrapping.
type Options struct {
	Build buildinfo.Info
}

// App wires the CLI, TUI, logger, config, and system executor.
type App struct {
	root *cli.Root
}

// New creates a new LLStack application instance.
func New(opts Options) *App {
	runtimeConfig := config.DefaultRuntimeConfig()
	logger := logging.NewDefault(os.Stderr)
	executor := system.NewLocalExecutor()
	build := buildinfo.Normalize(opts.Build)

	root := cli.NewRoot(cli.Dependencies{
		Version: build.Version,
		Build:   build,
		Config:  runtimeConfig,
		Logger:  logger,
		Exec:    executor,
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	})

	return &App{root: root}
}

// Run executes the root command with the provided arguments.
func (a *App) Run(args []string) error {
	ctx := context.Background()
	cmd := a.root.Command(ctx)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(a.root.Stderr, cli.FormatError(err))
		return err
	}

	return nil
}
