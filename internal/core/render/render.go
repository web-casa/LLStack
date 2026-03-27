package render

import (
	"encoding/json"
	"io/fs"

	"github.com/web-casa/llstack/internal/core/model"
)

// SiteRenderOptions provides backend-specific render destinations.
type SiteRenderOptions struct {
	VHostPath          string
	OutputDir          string
	ListenerMapPath    string
	ParityReportPath   string
	FPMPoolConfigPath  string // per-site FPM pool config output path (Apache)
	SystemUser         string // per-site Linux user for isolation
}

// Asset is a rendered file produced by a backend renderer.
type Asset struct {
	Path    string
	Content []byte
	Mode    fs.FileMode
}

// SiteRenderResult contains rendered files and warnings.
type SiteRenderResult struct {
	Assets       []Asset                    `json:"assets"`
	Warnings     []string                   `json:"warnings,omitempty"`
	ParityReport *ParityReport              `json:"parity_report,omitempty"`
	Capabilities *model.BackendCapabilities `json:"capabilities,omitempty"`
}

// ParityStatus describes how a canonical feature mapped to a backend.
type ParityStatus string

const (
	ParityMapped      ParityStatus = "mapped"
	ParityDegraded    ParityStatus = "degraded"
	ParityUnsupported ParityStatus = "unsupported"
)

// ParityItem records the mapping status of a feature.
type ParityItem struct {
	Feature string       `json:"feature"`
	Status  ParityStatus `json:"status"`
	Target  string       `json:"target,omitempty"`
	Note    string       `json:"note,omitempty"`
}

// ParityReport summarizes backend compatibility for a rendered site.
type ParityReport struct {
	Backend string       `json:"backend"`
	Site    string       `json:"site"`
	Items   []ParityItem `json:"items"`
}

// JSON returns the parity report in indented JSON form.
func (p ParityReport) JSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// SiteRenderer renders a canonical site definition for a backend.
type SiteRenderer interface {
	RenderSite(site model.Site, opts SiteRenderOptions) (SiteRenderResult, error)
}
