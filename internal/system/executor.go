package system

import (
	"bytes"
	"context"
	"os/exec"
)

// Command describes a system command executed through the central executor.
type Command struct {
	Name string
	Args []string
}

// Result captures the executor result.
type Result struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// Executor centralizes system command execution.
type Executor interface {
	Run(ctx context.Context, cmd Command) (Result, error)
}

// LocalExecutor runs commands on the local machine.
type LocalExecutor struct{}

// NewLocalExecutor returns a local command executor.
func NewLocalExecutor() LocalExecutor {
	return LocalExecutor{}
}

// Run executes the command and returns captured output.
func (e LocalExecutor) Run(ctx context.Context, cmd Command) (Result, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	c := exec.CommandContext(ctx, cmd.Name, cmd.Args...)
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()
	result := Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if c.ProcessState != nil {
		result.ExitCode = c.ProcessState.ExitCode()
	}

	return result, err
}
