package plan

// Report is a lightweight execution summary used by Phase 1 placeholders.
type Report struct {
	Status   string   `json:"status"`
	Summary  string   `json:"summary"`
	Warnings []string `json:"warnings,omitempty"`
}
