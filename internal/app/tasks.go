package app

import (
	"context"

	"github.com/web-casa/llstack/internal/core/plan"
	"github.com/web-casa/llstack/internal/logging"
)

// Task represents a unit of work that can be executed by the shared task runner.
type Task interface {
	Name() string
	Run(ctx context.Context) error
}

// TaskResult captures the outcome of a task execution.
type TaskResult struct {
	Name   string     `json:"name"`
	Status string     `json:"status"`
	Plan   *plan.Plan `json:"plan,omitempty"`
	Error  string     `json:"error,omitempty"`
}

// TaskRunner executes tasks and reports outcomes through the shared logger.
type TaskRunner struct {
	logger logging.Logger
}

// NewTaskRunner builds a task runner for CLI/TUI operations.
func NewTaskRunner(logger logging.Logger) TaskRunner {
	return TaskRunner{logger: logger}
}

// Run executes a task synchronously.
func (r TaskRunner) Run(ctx context.Context, task Task) TaskResult {
	r.logger.Info("task started", "task", task.Name())
	if err := task.Run(ctx); err != nil {
		r.logger.Error("task failed", "task", task.Name(), "error", err)
		return TaskResult{
			Name:   task.Name(),
			Status: "failed",
			Error:  err.Error(),
		}
	}

	r.logger.Info("task completed", "task", task.Name())
	return TaskResult{
		Name:   task.Name(),
		Status: "completed",
	}
}
