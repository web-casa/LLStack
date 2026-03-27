package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/logging"
	"github.com/web-casa/llstack/internal/system"
)

// Run starts the Phase 1 TUI shell.
func Run(version string, cfg config.RuntimeConfig, logger logging.Logger, exec system.Executor) error {
	program := tea.NewProgram(NewModel(version, cfg, logger, exec), tea.WithAltScreen())
	_, err := program.Run()
	return err
}
