package cache

import "time"

// ProviderName identifies a cache provider.
type ProviderName string

const (
	ProviderMemcached ProviderName = "memcached"
	ProviderRedis     ProviderName = "redis"
	ProviderValkey    ProviderName = "valkey"
)

// InstallOptions controls cache installation behavior.
type InstallOptions struct {
	Provider ProviderName
	DryRun   bool
	PlanOnly bool
}

// ConfigureOptions controls cache configuration behavior.
type ConfigureOptions struct {
	Provider    ProviderName
	Bind        string
	Port        int
	MaxMemoryMB int
	DryRun      bool
	PlanOnly    bool
}

// ProviderCapability captures provider-level support flags.
type ProviderCapability struct {
	Provider            ProviderName `json:"provider"`
	SupportsPersistence bool         `json:"supports_persistence"`
	SupportsEviction    bool         `json:"supports_eviction"`
	Notes               []string     `json:"notes,omitempty"`
}

// ProviderManifest persists managed cache state.
type ProviderManifest struct {
	Provider     ProviderName       `json:"provider"`
	ServiceName  string             `json:"service_name"`
	Packages     []string           `json:"packages,omitempty"`
	ConfigPath   string             `json:"config_path,omitempty"`
	Bind         string             `json:"bind,omitempty"`
	Port         int                `json:"port,omitempty"`
	MaxMemoryMB  int                `json:"max_memory_mb,omitempty"`
	Status       string             `json:"status"`
	Warnings     []string           `json:"warnings,omitempty"`
	Capabilities ProviderCapability `json:"capabilities"`
	InstalledAt  time.Time          `json:"installed_at,omitempty"`
	UpdatedAt    time.Time          `json:"updated_at"`
}
