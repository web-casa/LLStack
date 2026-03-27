package model

import "time"

// Site models the canonical site definition used across backends.
type Site struct {
	Name              string              `json:"name"`
	Backend           string              `json:"backend"`
	State             string              `json:"state,omitempty"`
	Profile           string              `json:"profile,omitempty"`
	DocumentRoot      string              `json:"document_root"`
	IndexFiles        []string            `json:"index_files,omitempty"`
	Domain            DomainBinding       `json:"domain"`
	TLS               TLSConfig           `json:"tls"`
	PHP               PHPRuntimeBinding   `json:"php"`
	RewriteRules      []RewriteRule       `json:"rewrite_rules,omitempty"`
	HeaderRules       []HeaderRule        `json:"header_rules,omitempty"`
	AccessRules       []AccessControlRule `json:"access_rules,omitempty"`
	ReverseProxyRules []ReverseProxyRule  `json:"reverse_proxy_rules,omitempty"`
	LSWS              *LSWSOptions        `json:"lsws,omitempty"`
	Logs              LogConfig           `json:"logs"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
}

// DeployProfile captures a reusable site bootstrap profile.
type DeployProfile struct {
	Name              string             `json:"name"`
	Description       string             `json:"description,omitempty"`
	EnablePHP         bool               `json:"enable_php"`
	DefaultIndexFiles []string           `json:"default_index_files,omitempty"`
	RewriteRules      []RewriteRule      `json:"rewrite_rules,omitempty"`
	HeaderRules       []HeaderRule       `json:"header_rules,omitempty"`
	ReverseProxyRules []ReverseProxyRule `json:"reverse_proxy_rules,omitempty"`
}

// DomainBinding captures server name and alias bindings.
type DomainBinding struct {
	ServerName string   `json:"server_name"`
	Aliases    []string `json:"aliases,omitempty"`
	HTTPPort   int      `json:"http_port,omitempty"`
	HTTPSPort  int      `json:"https_port,omitempty"`
}

// TLSConfig captures a site's TLS configuration.
type TLSConfig struct {
	Enabled         bool   `json:"enabled"`
	Mode            string `json:"mode,omitempty"`
	CertificateFile string `json:"certificate_file,omitempty"`
	CertificateKey  string `json:"certificate_key,omitempty"`
}

// PHPRuntimeBinding captures the PHP handler for the site.
type PHPRuntimeBinding struct {
	Enabled    bool   `json:"enabled"`
	Version    string `json:"version,omitempty"`
	Handler    string `json:"handler,omitempty"`
	FPMService string `json:"fpm_service,omitempty"`
	Socket     string `json:"socket,omitempty"`
	Command    string `json:"command,omitempty"`
}

// RewriteRule models a canonical rewrite directive.
type RewriteRule struct {
	Pattern      string   `json:"pattern"`
	Substitution string   `json:"substitution"`
	Flags        []string `json:"flags,omitempty"`
}

// HeaderRule models a response header mutation.
type HeaderRule struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Action string `json:"action"`
}

// AccessControlRule models a basic access control statement.
type AccessControlRule struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Source string `json:"source,omitempty"`
}

// ReverseProxyRule models a basic reverse proxy route.
type ReverseProxyRule struct {
	PathPrefix   string `json:"path_prefix"`
	Upstream     string `json:"upstream"`
	PreserveHost bool   `json:"preserve_host"`
}

// LogConfig captures site log file destinations.
type LogConfig struct {
	AccessLog string `json:"access_log"`
	ErrorLog  string `json:"error_log"`
}

// LSWSOptions captures LiteSpeed Enterprise-specific configuration extensions.
type LSWSOptions struct {
	CustomDirectives      []string `json:"custom_directives,omitempty"`
	RequestedFeatureFlags []string `json:"requested_feature_flags,omitempty"`
}

// BackendCapabilities records backend-specific capability state detected at render time.
type BackendCapabilities struct {
	Backend     string          `json:"backend"`
	LicenseMode string          `json:"license_mode,omitempty"`
	Flags       map[string]bool `json:"flags,omitempty"`
	Notes       []string        `json:"notes,omitempty"`
}

// SiteManifest persists the canonical site plus rendered paths.
type SiteManifest struct {
	Site              Site                 `json:"site"`
	SystemUser        string               `json:"system_user,omitempty"`
	VHostPath         string               `json:"vhost_path,omitempty"`
	ManagedAssetPaths []string             `json:"managed_asset_paths,omitempty"`
	ParityReportPath  string               `json:"parity_report_path,omitempty"`
	Capabilities      *BackendCapabilities `json:"capabilities,omitempty"`
	CreatedAt         time.Time            `json:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at"`
}
