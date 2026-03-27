package plan

import (
	"encoding/json"
	"time"
)

// Operation represents a single planned action.
type Operation struct {
	ID      string            `json:"id"`
	Kind    string            `json:"kind"`
	Target  string            `json:"target"`
	Details map[string]string `json:"details,omitempty"`
}

// Plan is the machine-readable unit shared by CLI, TUI, and config-driven flows.
type Plan struct {
	Kind       string      `json:"kind"`
	Summary    string      `json:"summary"`
	DryRun     bool        `json:"dry_run"`
	PlanOnly   bool        `json:"plan_only"`
	Warnings   []string    `json:"warnings,omitempty"`
	Operations []Operation `json:"operations"`
	CreatedAt  time.Time   `json:"created_at"`
}

// New creates a minimal plan shell with a stable timestamp.
func New(kind, summary string) Plan {
	return Plan{
		Kind:       kind,
		Summary:    summary,
		Operations: make([]Operation, 0),
		CreatedAt:  time.Now().UTC(),
	}
}

// AddOperation appends an operation to the plan.
func (p *Plan) AddOperation(op Operation) {
	p.Operations = append(p.Operations, op)
}

// JSON serializes the plan in an indented form suitable for CLI and tests.
func (p Plan) JSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}
