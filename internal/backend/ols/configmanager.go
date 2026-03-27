package ols

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	coremodel "github.com/web-casa/llstack/internal/core/model"
)

const (
	vhostBeginMarker = "# LLSTACK_VHOST_BEGIN "
	vhostEndMarker   = "# LLSTACK_VHOST_END "
	mapBeginMarker   = "# LLSTACK_MAP_BEGIN "
	mapEndMarker     = "# LLSTACK_MAP_END "
)

// RegisterSiteInMainConfig appends a virtualhost block and listener map entry
// to the OLS main config for the given site. Uses comment markers so the
// blocks can be identified, updated, and removed.
func RegisterSiteInMainConfig(configPath string, site coremodel.Site, vhostConfigFile string) error {
	content, err := readConfigOrEmpty(configPath)
	if err != nil {
		return fmt.Errorf("read OLS main config: %w", err)
	}

	siteName := site.Name
	docroot := site.DocumentRoot

	// Remove existing blocks for this site (idempotent)
	content = removeMarkedBlock(content, vhostBeginMarker+siteName, vhostEndMarker+siteName)
	content = removeMarkedBlock(content, mapBeginMarker+siteName, mapEndMarker+siteName)

	// Build virtualhost block
	vhostBlock := fmt.Sprintf(`
%s%s
virtualhost %s {
  vhRoot                  %s
  configFile              %s
  allowSymbolLink         1
  enableScript            1
}
%s%s
`, vhostBeginMarker, siteName, siteName, docroot, vhostConfigFile, vhostEndMarker, siteName)

	// Build listener map line
	domains := site.Domain.ServerName
	for _, alias := range site.Domain.Aliases {
		domains += ", " + alias
	}
	mapBlock := fmt.Sprintf("%s%s\n  map                     %s %s\n%s%s\n",
		mapBeginMarker, siteName, siteName, domains, mapEndMarker, siteName)

	// Append virtualhost block at end of file
	content = strings.TrimRight(content, "\n") + "\n" + vhostBlock

	// Insert map block into first listener block (before its closing brace)
	content = insertIntoListenerBlock(content, mapBlock)

	return os.WriteFile(configPath, []byte(content), 0o644)
}

// UnregisterSiteFromMainConfig removes the virtualhost block and listener map
// entry for the given site from the OLS main config.
func UnregisterSiteFromMainConfig(configPath string, siteName string) error {
	content, err := readConfigOrEmpty(configPath)
	if err != nil {
		return fmt.Errorf("read OLS main config: %w", err)
	}

	content = removeMarkedBlock(content, vhostBeginMarker+siteName, vhostEndMarker+siteName)
	content = removeMarkedBlock(content, mapBeginMarker+siteName, mapEndMarker+siteName)

	return os.WriteFile(configPath, []byte(content), 0o644)
}

func readConfigOrEmpty(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(raw), nil
}

func removeMarkedBlock(content, beginMarker, endMarker string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inside := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == beginMarker {
			inside = true
			continue
		}
		if trimmed == endMarker {
			inside = false
			continue
		}
		if !inside {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func insertIntoListenerBlock(content, mapBlock string) string {
	lines := strings.Split(content, "\n")
	inListener := false
	braceDepth := 0
	insertIdx := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inListener && strings.HasPrefix(trimmed, "listener ") {
			inListener = true
			braceDepth = 0
		}
		if inListener {
			braceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			if braceDepth == 0 && strings.Contains(trimmed, "}") {
				insertIdx = i
				break
			}
		}
	}

	if insertIdx < 0 {
		// No listener block found, append map block at end
		return content + "\n" + mapBlock
	}

	// Insert map block before the closing brace of the listener
	var result []string
	result = append(result, lines[:insertIdx]...)
	result = append(result, mapBlock)
	result = append(result, lines[insertIdx:]...)
	return strings.Join(result, "\n")
}

// MainConfigPath returns the default OLS main config path from runtime config.
func MainConfigPath(olsRoot string) string {
	return filepath.Join(olsRoot, "conf", "httpd_config.conf")
}
