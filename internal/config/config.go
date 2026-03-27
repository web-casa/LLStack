package config

import "path/filepath"

// RuntimeConfig contains application-wide defaults and path conventions.
type RuntimeConfig struct {
	Paths  Paths        `json:"paths" yaml:"paths"`
	Apache ApacheConfig `json:"apache" yaml:"apache"`
	OLS    OLSConfig    `json:"ols" yaml:"ols"`
	LSWS   LSWSConfig   `json:"lsws" yaml:"lsws"`
	PHP    PHPConfig    `json:"php" yaml:"php"`
	DB     DBConfig     `json:"db" yaml:"db"`
	Cache  CacheConfig  `json:"cache" yaml:"cache"`
	SSL    SSLConfig    `json:"ssl" yaml:"ssl"`
}

// Paths captures default LLStack-managed locations.
type Paths struct {
	ConfigDir    string `json:"config_dir" yaml:"config_dir"`
	StateDir     string `json:"state_dir" yaml:"state_dir"`
	HistoryDir   string `json:"history_dir" yaml:"history_dir"`
	BackupsDir   string `json:"backups_dir" yaml:"backups_dir"`
	LogDir       string `json:"log_dir" yaml:"log_dir"`
	SitesRootDir string `json:"sites_root_dir" yaml:"sites_root_dir"`
}

// ApacheConfig captures managed Apache-specific runtime settings.
type ApacheConfig struct {
	ManagedVhostsDir string   `json:"managed_vhosts_dir" yaml:"managed_vhosts_dir"`
	ConfigTestCmd    []string `json:"config_test_cmd" yaml:"config_test_cmd"`
	ReloadCmd        []string `json:"reload_cmd" yaml:"reload_cmd"`
	RestartCmd       []string `json:"restart_cmd" yaml:"restart_cmd"`
}

// OLSConfig captures managed OpenLiteSpeed-specific runtime settings.
type OLSConfig struct {
	ManagedVhostsRoot   string   `json:"managed_vhosts_root" yaml:"managed_vhosts_root"`
	ManagedListenersDir string   `json:"managed_listeners_dir" yaml:"managed_listeners_dir"`
	MainConfigPath      string   `json:"main_config_path" yaml:"main_config_path"`
	DefaultPHPBinary    string   `json:"default_php_binary" yaml:"default_php_binary"`
	ConfigTestCmd       []string `json:"config_test_cmd" yaml:"config_test_cmd"`
	ReloadCmd           []string `json:"reload_cmd" yaml:"reload_cmd"`
	RestartCmd          []string `json:"restart_cmd" yaml:"restart_cmd"`
}

// LSWSConfig captures managed LiteSpeed Enterprise runtime settings.
type LSWSConfig struct {
	ManagedIncludesDir string   `json:"managed_includes_dir" yaml:"managed_includes_dir"`
	DefaultPHPBinary   string   `json:"default_php_binary" yaml:"default_php_binary"`
	LicenseSerialFile  string   `json:"license_serial_file" yaml:"license_serial_file"`
	DetectCmd          []string `json:"detect_cmd" yaml:"detect_cmd"`
	ConfigTestCmd      []string `json:"config_test_cmd" yaml:"config_test_cmd"`
	ReloadCmd          []string `json:"reload_cmd" yaml:"reload_cmd"`
	RestartCmd         []string `json:"restart_cmd" yaml:"restart_cmd"`
}

// PHPConfig captures managed PHP runtime settings.
type PHPConfig struct {
	ManagedRuntimesDir     string   `json:"managed_runtimes_dir" yaml:"managed_runtimes_dir"`
	ManagedProfilesDir     string   `json:"managed_profiles_dir" yaml:"managed_profiles_dir"`
	ProfileRoot            string   `json:"profile_root" yaml:"profile_root"`
	RuntimeRoot            string   `json:"runtime_root" yaml:"runtime_root"`
	StateRoot              string   `json:"state_root" yaml:"state_root"`
	RemiReleaseRPMTemplate string   `json:"remi_release_rpm_template" yaml:"remi_release_rpm_template"`
	ELMajorOverride        string   `json:"el_major_override" yaml:"el_major_override"`
	SupportedVersions      []string `json:"supported_versions" yaml:"supported_versions"`
	DefaultExtensions      []string `json:"default_extensions" yaml:"default_extensions"`
}

// DBConfig captures managed database provider settings.
type DBConfig struct {
	ManagedProvidersDir      string `json:"managed_providers_dir" yaml:"managed_providers_dir"`
	ManagedConnectionsDir    string `json:"managed_connections_dir" yaml:"managed_connections_dir"`
	CertificatesDir          string `json:"certificates_dir" yaml:"certificates_dir"`
	MariaDBTLSConfigPath     string `json:"mariadb_tls_config_path" yaml:"mariadb_tls_config_path"`
	MySQLTLSConfigPath       string `json:"mysql_tls_config_path" yaml:"mysql_tls_config_path"`
	PerconaTLSConfigPath     string `json:"percona_tls_config_path" yaml:"percona_tls_config_path"`
	PostgreSQLTLSConfigPath  string `json:"postgresql_tls_config_path" yaml:"postgresql_tls_config_path"`
	DefaultPostgreSQLVersion string `json:"default_postgresql_version" yaml:"default_postgresql_version"`
}

// CacheConfig captures managed cache provider settings.
type CacheConfig struct {
	ManagedProvidersDir string `json:"managed_providers_dir" yaml:"managed_providers_dir"`
	MemcachedConfigPath string `json:"memcached_config_path" yaml:"memcached_config_path"`
	RedisConfigPath     string `json:"redis_config_path" yaml:"redis_config_path"`
	ValkeyConfigPath    string `json:"valkey_config_path" yaml:"valkey_config_path"`
}

// SSLConfig captures ACME/certbot integration defaults.
type SSLConfig struct {
	CertbotCandidates  []string `json:"certbot_candidates" yaml:"certbot_candidates"`
	LetsEncryptLiveDir string   `json:"letsencrypt_live_dir" yaml:"letsencrypt_live_dir"`
}

// DefaultRuntimeConfig returns the product defaults agreed in Phase 0.
func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		Paths: Paths{
			ConfigDir:    "/etc/llstack",
			StateDir:     "/var/lib/llstack/state",
			HistoryDir:   "/var/lib/llstack/history",
			BackupsDir:   "/var/lib/llstack/backups",
			LogDir:       "/var/log/llstack",
			SitesRootDir: "/data/www",
		},
		Apache: ApacheConfig{
			ManagedVhostsDir: "/etc/httpd/conf.d/llstack/sites",
			ConfigTestCmd:    []string{"apachectl", "configtest"},
			ReloadCmd:        []string{"systemctl", "reload", "httpd"},
			RestartCmd:       []string{"systemctl", "restart", "httpd"},
		},
		OLS: OLSConfig{
			ManagedVhostsRoot:   "/usr/local/lsws/conf/vhosts",
			ManagedListenersDir: "/usr/local/lsws/conf/llstack/listeners",
			MainConfigPath:      "/usr/local/lsws/conf/httpd_config.conf",
			DefaultPHPBinary:    "/usr/bin/lsphp",
			ConfigTestCmd:       []string{"lswsctrl", "configtest"},
			ReloadCmd:           []string{"lswsctrl", "reload"},
			RestartCmd:          []string{"lswsctrl", "restart"},
		},
		LSWS: LSWSConfig{
			ManagedIncludesDir: "/usr/local/lsws/conf/llstack/includes",
			DefaultPHPBinary:   "/usr/bin/lsphp",
			LicenseSerialFile:  "/usr/local/lsws/conf/serial.no",
			DetectCmd:          []string{"lshttpd", "-v"},
			ConfigTestCmd:      []string{"lswsctrl", "configtest"},
			ReloadCmd:          []string{"lswsctrl", "reload"},
			RestartCmd:         []string{"lswsctrl", "restart"},
		},
		PHP: PHPConfig{
			ManagedRuntimesDir:     "/etc/llstack/php/runtimes",
			ManagedProfilesDir:     "/etc/llstack/php/profiles",
			ProfileRoot:            "/etc/opt/remi",
			RuntimeRoot:            "/opt/remi",
			StateRoot:              "/var/opt/remi",
			RemiReleaseRPMTemplate: "https://rpms.remirepo.net/enterprise/remi-release-%s.rpm",
			SupportedVersions:      []string{"7.4", "8.0", "8.1", "8.2", "8.3", "8.4", "8.5"},
			DefaultExtensions:      []string{"opcache", "mbstring", "xml", "mysqlnd", "pdo"},
		},
		DB: DBConfig{
			ManagedProvidersDir:      "/etc/llstack/db/providers",
			ManagedConnectionsDir:    "/etc/llstack/db/connections",
			CertificatesDir:          "/etc/llstack/db/certs",
			MariaDBTLSConfigPath:     "/etc/my.cnf.d/llstack-mariadb-tls.cnf",
			MySQLTLSConfigPath:       "/etc/my.cnf.d/llstack-mysql-tls.cnf",
			PerconaTLSConfigPath:     "/etc/my.cnf.d/llstack-percona-tls.cnf",
			PostgreSQLTLSConfigPath:  "/var/lib/pgsql/16/data/conf.d/llstack-tls.conf",
			DefaultPostgreSQLVersion: "16",
		},
		Cache: CacheConfig{
			ManagedProvidersDir: "/etc/llstack/cache/providers",
			MemcachedConfigPath: "/etc/sysconfig/memcached",
			RedisConfigPath:     "/etc/redis.conf",
			ValkeyConfigPath:    "/etc/valkey/valkey.conf",
		},
		SSL: SSLConfig{
			CertbotCandidates:  []string{"/usr/bin/certbot", "/bin/certbot", "/usr/local/bin/certbot", "certbot"},
			LetsEncryptLiveDir: "/etc/letsencrypt/live",
		},
	}
}

// ManagedSitesDir stores canonical site definitions.
func (p Paths) ManagedSitesDir() string {
	return filepath.Join(p.ConfigDir, "sites")
}

// SiteLogsDir stores LLStack-managed site logs.
func (p Paths) SiteLogsDir() string {
	return filepath.Join(p.LogDir, "sites")
}

// ParityReportsDir stores backend parity reports.
func (p Paths) ParityReportsDir() string {
	return filepath.Join(p.StateDir, "parity")
}
