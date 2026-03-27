package views

import (
	"fmt"
	"strings"

	"github.com/web-casa/llstack/internal/config"
)

// RenderInstall renders the install wizard summary.
func RenderInstall(cfg config.RuntimeConfig) string {
	return strings.Join([]string{
		"Install Wizard",
		"",
		"Current flow",
		"1. choose backend and PHP versions",
		"2. choose database provider and TLS policy",
		"3. choose cache providers",
		"4. optionally create the first site",
		"5. preview plan or apply directly",
		"",
		fmt.Sprintf("Default site root: %s", cfg.Paths.SitesRootDir),
		"CLI parity: llstack install --backend apache --php_version 8.3 --db mariadb --with_memcached --site example.com",
	}, "\n")
}
