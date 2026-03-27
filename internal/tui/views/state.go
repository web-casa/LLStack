package views

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/web-casa/llstack/internal/cache"
	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/db"
	phpruntime "github.com/web-casa/llstack/internal/php"
)

func readSiteManifests(cfg config.RuntimeConfig) ([]model.SiteManifest, error) {
	dir := cfg.Paths.ManagedSitesDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]model.SiteManifest, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var manifest model.SiteManifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, err
		}
		out = append(out, manifest)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Site.Name < out[j].Site.Name })
	return out, nil
}

func readPHPRuntimes(cfg config.RuntimeConfig) ([]phpruntime.RuntimeManifest, error) {
	dir := cfg.PHP.ManagedRuntimesDir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]phpruntime.RuntimeManifest, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var manifest phpruntime.RuntimeManifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, err
		}
		out = append(out, manifest)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func readDBProviders(cfg config.RuntimeConfig) ([]db.ProviderManifest, error) {
	dir := cfg.DB.ManagedProvidersDir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]db.ProviderManifest, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var manifest db.ProviderManifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, err
		}
		out = append(out, manifest)
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i].Provider) < string(out[j].Provider) })
	return out, nil
}

func readCacheProviders(cfg config.RuntimeConfig) ([]cache.ProviderManifest, error) {
	dir := cfg.Cache.ManagedProvidersDir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]cache.ProviderManifest, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var manifest cache.ProviderManifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, err
		}
		out = append(out, manifest)
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i].Provider) < string(out[j].Provider) })
	return out, nil
}

func readRecentLogLines(cfg config.RuntimeConfig, limit int) ([]string, error) {
	dir := cfg.Paths.SiteLogsDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	raw, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	for i := range lines {
		lines[i] = fmt.Sprintf("%s: %s", entries[0].Name(), lines[i])
	}
	return lines, nil
}

func siteDriftCount(site model.SiteManifest) int {
	count := 0
	for _, path := range site.ManagedAssetPaths {
		if _, err := os.Stat(path); err != nil {
			count++
		}
	}
	return count
}
