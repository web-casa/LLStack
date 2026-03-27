package db

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/web-casa/llstack/internal/config"
)

// ProviderSpec captures install/runtime details for a database provider.
type ProviderSpec struct {
	Name          ProviderName
	Family        string
	Version       string
	ServiceName   string
	Packages      []string
	ClientBin     string
	Port          int
	RepoRequired  bool
	TLSConfigPath string
	Capabilities  DatabaseCapability
	Warnings      []string
}

// ResolveProvider returns the managed provider specification.
func ResolveProvider(cfg config.RuntimeConfig, name ProviderName, version string) (ProviderSpec, error) {
	switch name {
	case ProviderMariaDB:
		return ProviderSpec{
			Name:          ProviderMariaDB,
			Family:        "mysql",
			Version:       firstNonEmpty(version, "10.x"),
			ServiceName:   "mariadb",
			Packages:      []string{"mariadb-server", "mariadb"},
			ClientBin:     "mariadb",
			Port:          3306,
			TLSConfigPath: cfg.DB.MariaDBTLSConfigPath,
			Capabilities: DatabaseCapability{
				Provider:               ProviderMariaDB,
				Family:                 "mysql",
				SupportsTLS:            true,
				SupportsRequiredTLS:    true,
				SupportsRoles:          false,
				SupportsDatabaseCreate: true,
				Notes:                  []string{"Uses local root or socket auth by default on EL systems."},
			},
		}, nil
	case ProviderMySQL:
		return ProviderSpec{
			Name:          ProviderMySQL,
			Family:        "mysql",
			Version:       firstNonEmpty(version, "8.4"),
			ServiceName:   "mysqld",
			Packages:      []string{"mysql-community-server", "mysql-community-client"},
			ClientBin:     "mysql",
			Port:          3306,
			RepoRequired:  true,
			TLSConfigPath: cfg.DB.MySQLTLSConfigPath,
			Capabilities: DatabaseCapability{
				Provider:               ProviderMySQL,
				Family:                 "mysql",
				SupportsTLS:            true,
				SupportsRequiredTLS:    true,
				SupportsRoles:          true,
				SupportsDatabaseCreate: true,
			},
			Warnings: []string{"MySQL Community packages require the vendor yum repository to be configured on EL9/EL10."},
		}, nil
	case ProviderPercona:
		return ProviderSpec{
			Name:          ProviderPercona,
			Family:        "mysql",
			Version:       firstNonEmpty(version, "8.4"),
			ServiceName:   "mysqld",
			Packages:      []string{"percona-server-server", "percona-server-client"},
			ClientBin:     "mysql",
			Port:          3306,
			RepoRequired:  true,
			TLSConfigPath: cfg.DB.PerconaTLSConfigPath,
			Capabilities: DatabaseCapability{
				Provider:               ProviderPercona,
				Family:                 "mysql",
				SupportsTLS:            true,
				SupportsRequiredTLS:    true,
				SupportsRoles:          true,
				SupportsDatabaseCreate: true,
			},
			Warnings: []string{"Percona Server packages require the Percona repository to be configured on EL9/EL10."},
		}, nil
	case ProviderPostgres:
		version = firstNonEmpty(version, cfg.DB.DefaultPostgreSQLVersion)
		return ProviderSpec{
			Name:          ProviderPostgres,
			Family:        "postgresql",
			Version:       version,
			ServiceName:   "postgresql-" + version,
			Packages:      []string{"postgresql" + version + "-server", "postgresql" + version},
			ClientBin:     "psql",
			Port:          5432,
			RepoRequired:  true,
			TLSConfigPath: cfg.DB.PostgreSQLTLSConfigPath,
			Capabilities: DatabaseCapability{
				Provider:               ProviderPostgres,
				Family:                 "postgresql",
				SupportsTLS:            true,
				SupportsRequiredTLS:    true,
				SupportsRoles:          true,
				SupportsDatabaseCreate: true,
				Notes:                  []string{"TLS enforcement also depends on pg_hba.conf hostssl rules, which Phase 6 does not rewrite yet."},
			},
			Warnings: []string{"PostgreSQL packages assume the PGDG repository layout and service naming convention."},
		}, nil
	default:
		return ProviderSpec{}, fmt.Errorf("unsupported database provider %q", name)
	}
}

// BuildTLSProfile returns a provider TLS profile and optional warnings.
func BuildTLSProfile(cfg config.RuntimeConfig, spec ProviderSpec, mode TLSMode) (DatabaseTLSProfile, []string) {
	if mode == "" {
		mode = TLSDisabled
	}
	baseDir := filepath.Join(cfg.DB.CertificatesDir, string(spec.Name))
	profile := DatabaseTLSProfile{
		Mode:             mode,
		Enabled:          mode != TLSDisabled,
		Status:           "disabled",
		ServerConfigPath: spec.TLSConfigPath,
		ServerCAFile:     filepath.Join(baseDir, "ca.pem"),
		ServerCertFile:   filepath.Join(baseDir, "server-cert.pem"),
		ServerKeyFile:    filepath.Join(baseDir, "server-key.pem"),
	}
	var warnings []string
	if mode == TLSDisabled {
		return profile, nil
	}

	profile.Status = "planned"
	switch spec.Family {
	case "mysql":
		if spec.Name == ProviderMariaDB {
			profile.ClientFlags = []string{"--ssl", "--ssl-ca=" + profile.ServerCAFile}
		} else {
			profile.ClientFlags = []string{"--ssl-mode=REQUIRED", "--ssl-ca=" + profile.ServerCAFile}
		}
	case "postgresql":
		profile.ClientFlags = []string{"sslmode=verify-full", "sslrootcert=" + profile.ServerCAFile}
		if mode == TLSRequired {
			warnings = append(warnings, "PostgreSQL required TLS still needs matching hostssl entries in pg_hba.conf.")
		}
	}
	return profile, warnings
}

// RenderTLSConfigSnippet renders provider-native TLS config.
func RenderTLSConfigSnippet(spec ProviderSpec, profile DatabaseTLSProfile) string {
	if !profile.Enabled {
		return ""
	}
	switch spec.Family {
	case "mysql":
		lines := []string{
			"[mysqld]",
			"ssl-ca=" + profile.ServerCAFile,
			"ssl-cert=" + profile.ServerCertFile,
			"ssl-key=" + profile.ServerKeyFile,
		}
		if profile.Mode == TLSRequired {
			lines = append(lines, "require_secure_transport=ON")
		}
		return strings.Join(lines, "\n") + "\n"
	case "postgresql":
		lines := []string{
			"ssl = on",
			"ssl_ca_file = '" + profile.ServerCAFile + "'",
			"ssl_cert_file = '" + profile.ServerCertFile + "'",
			"ssl_key_file = '" + profile.ServerKeyFile + "'",
		}
		return strings.Join(lines, "\n") + "\n"
	default:
		return ""
	}
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
