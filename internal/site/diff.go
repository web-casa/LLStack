package site

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/core/render"
)

// DiffEntry captures drift between managed assets and freshly rendered output.
type DiffEntry struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	Preview string `json:"preview,omitempty"`
}

// DiffReport summarizes the current drift state of a managed site.
type DiffReport struct {
	Name    string      `json:"name"`
	Backend string      `json:"backend"`
	Entries []DiffEntry `json:"entries,omitempty"`
}

// Diff compares managed assets against the current canonical manifest rendering.
func (m Manager) Diff(ctx context.Context, name string) (DiffReport, error) {
	manifest, err := m.loadManifest(name)
	if err != nil {
		return DiffReport{}, err
	}
	renderer, _, renderOpts, err := m.backendComponents(ctx, manifest.Site)
	if err != nil {
		return DiffReport{}, err
	}
	rendered, err := renderer.RenderSite(manifest.Site, renderOpts)
	if err != nil {
		return DiffReport{}, err
	}
	assets := append([]render.Asset{}, rendered.Assets...)
	assets = append(assets, ScaffoldAssets(manifest.Site)...)

	report := DiffReport{
		Name:    name,
		Backend: manifest.Site.Backend,
		Entries: make([]DiffEntry, 0),
	}
	for _, asset := range assets {
		current, err := os.ReadFile(asset.Path)
		if err != nil {
			if os.IsNotExist(err) {
				report.Entries = append(report.Entries, DiffEntry{
					Path:    asset.Path,
					Status:  "missing",
					Preview: previewDiff("", string(asset.Content)),
				})
				continue
			}
			return DiffReport{}, err
		}
		if string(current) == string(asset.Content) {
			continue
		}
		report.Entries = append(report.Entries, DiffEntry{
			Path:    asset.Path,
			Status:  "changed",
			Preview: previewDiff(string(current), string(asset.Content)),
		})
	}

	expected := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		expected[asset.Path] = struct{}{}
	}
	for _, path := range manifest.ManagedAssetPaths {
		if _, ok := expected[path]; ok {
			continue
		}
		report.Entries = append(report.Entries, DiffEntry{
			Path:    path,
			Status:  "unexpected",
			Preview: "managed asset exists on disk but is no longer generated from the canonical model",
		})
	}

	return report, nil
}

func previewDiff(before, after string) string {
	beforeLines := splitLines(before)
	afterLines := splitLines(after)

	lines := []string{"--- current", "+++ planned"}
	shown := 0
	maxLen := max(len(beforeLines), len(afterLines))
	for i := 0; i < maxLen && shown < 20; i++ {
		var left, right string
		if i < len(beforeLines) {
			left = beforeLines[i]
		}
		if i < len(afterLines) {
			right = afterLines[i]
		}
		if left == right {
			continue
		}
		if left != "" {
			lines = append(lines, fmt.Sprintf("- %s", left))
			shown++
		}
		if right != "" {
			lines = append(lines, fmt.Sprintf("+ %s", right))
			shown++
		}
	}
	return strings.Join(lines, "\n")
}

func splitLines(value string) []string {
	value = strings.TrimRight(value, "\n")
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// PreviewAssets exposes the current generated asset set for tests and TUI use.
func (m Manager) PreviewAssets(ctx context.Context, site model.Site) ([]render.Asset, error) {
	renderer, _, renderOpts, err := m.backendComponents(ctx, site)
	if err != nil {
		return nil, err
	}
	rendered, err := renderer.RenderSite(site, renderOpts)
	if err != nil {
		return nil, err
	}
	assets := append([]render.Asset{}, rendered.Assets...)
	assets = append(assets, ScaffoldAssets(site)...)
	return assets, nil
}
