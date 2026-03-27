package db

import "time"

// ProviderName identifies a database provider.
type ProviderName string

const (
	ProviderMariaDB  ProviderName = "mariadb"
	ProviderMySQL    ProviderName = "mysql"
	ProviderPostgres ProviderName = "postgresql"
	ProviderPercona  ProviderName = "percona"
)

// TLSMode captures the desired database TLS policy.
type TLSMode string

const (
	TLSDisabled TLSMode = "disabled"
	TLSEnabled  TLSMode = "enabled"
	TLSRequired TLSMode = "required"
)

// InstallOptions controls provider installation behavior.
type InstallOptions struct {
	Provider ProviderName
	Version  string
	TLSMode  TLSMode
	DryRun   bool
	PlanOnly bool
}

// InitOptions controls provider initialization behavior.
type InitOptions struct {
	Provider      ProviderName
	AdminUser     string
	AdminPassword string
	TLSMode       TLSMode
	DryRun        bool
	PlanOnly      bool
}

// CreateDatabaseOptions controls database creation behavior.
type CreateDatabaseOptions struct {
	Provider ProviderName
	Name     string
	Owner    string
	Encoding string
	DryRun   bool
	PlanOnly bool
}

// CreateUserOptions controls database user creation behavior.
type CreateUserOptions struct {
	Provider   ProviderName
	Name       string
	Password   string
	Database   string
	Privileges string
	TLSMode    TLSMode
	DryRun     bool
	PlanOnly   bool
}

// DatabaseCapability captures provider-level support flags.
type DatabaseCapability struct {
	Provider               ProviderName `json:"provider"`
	Family                 string       `json:"family"`
	SupportsTLS            bool         `json:"supports_tls"`
	SupportsRequiredTLS    bool         `json:"supports_required_tls"`
	SupportsRoles          bool         `json:"supports_roles"`
	SupportsDatabaseCreate bool         `json:"supports_database_create"`
	Notes                  []string     `json:"notes,omitempty"`
}

// DatabaseTLSProfile captures server and client TLS state for a provider.
type DatabaseTLSProfile struct {
	Mode             TLSMode  `json:"mode"`
	Enabled          bool     `json:"enabled"`
	Status           string   `json:"status"`
	ServerConfigPath string   `json:"server_config_path,omitempty"`
	ServerCAFile     string   `json:"server_ca_file,omitempty"`
	ServerCertFile   string   `json:"server_cert_file,omitempty"`
	ServerKeyFile    string   `json:"server_key_file,omitempty"`
	ClientFlags      []string `json:"client_flags,omitempty"`
}

// ConnectionInfo captures a generated connection profile.
type ConnectionInfo struct {
	Name         string    `json:"name"`
	Host         string    `json:"host"`
	Port         int       `json:"port"`
	Database     string    `json:"database,omitempty"`
	User         string    `json:"user,omitempty"`
	PasswordFile string    `json:"password_file,omitempty"`
	TLSMode      TLSMode   `json:"tls_mode"`
	SSLFlags     []string  `json:"ssl_flags,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// ReconcileOptions controls managed provider artifact reconciliation.
type ReconcileOptions struct {
	Provider ProviderName
	DryRun   bool
	PlanOnly bool
}

// DatabaseRecord captures a created database.
type DatabaseRecord struct {
	Name      string    `json:"name"`
	Owner     string    `json:"owner,omitempty"`
	Encoding  string    `json:"encoding,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// UserRecord captures a created database user.
type UserRecord struct {
	Name       string    `json:"name"`
	Database   string    `json:"database,omitempty"`
	Privileges string    `json:"privileges,omitempty"`
	TLSMode    TLSMode   `json:"tls_mode,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ProviderManifest persists managed provider state.
type ProviderManifest struct {
	Provider        ProviderName       `json:"provider"`
	Version         string             `json:"version,omitempty"`
	Family          string             `json:"family"`
	ServiceName     string             `json:"service_name"`
	Packages        []string           `json:"packages,omitempty"`
	Capabilities    DatabaseCapability `json:"capabilities"`
	TLS             DatabaseTLSProfile `json:"tls"`
	Status          string             `json:"status"`
	Warnings        []string           `json:"warnings,omitempty"`
	AdminConnection *ConnectionInfo    `json:"admin_connection,omitempty"`
	Databases       []DatabaseRecord   `json:"databases,omitempty"`
	Users           []UserRecord       `json:"users,omitempty"`
	InstalledAt     time.Time          `json:"installed_at,omitempty"`
	InitializedAt   time.Time          `json:"initialized_at,omitempty"`
	UpdatedAt       time.Time          `json:"updated_at"`
}
