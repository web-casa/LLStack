package php

import "time"

// ProfileName identifies a managed php.ini profile.
type ProfileName string

const (
	ProfileGeneric ProfileName = "generic"
	ProfileWP      ProfileName = "wp"
	ProfileLaravel ProfileName = "laravel"
	ProfileAPI     ProfileName = "api"
	ProfileCustom  ProfileName = "custom"
)

// RuntimeManifest stores the managed state of a PHP runtime.
type RuntimeManifest struct {
	Version     string            `json:"version"`
	Packages    []string          `json:"packages"`
	Extensions  []string          `json:"extensions"`
	Profile     ProfileName       `json:"profile"`
	ProfilePath string            `json:"profile_path"`
	Bindings    map[string]string `json:"bindings,omitempty"`
	InstalledAt time.Time         `json:"installed_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// InstallOptions controls runtime installation.
type InstallOptions struct {
	Version      string
	Extensions   []string
	Profile      ProfileName
	DryRun       bool
	PlanOnly     bool
	IncludeFPM   bool
	IncludeLSAPI bool
}

// ExtensionsOptions controls extension changes.
type ExtensionsOptions struct {
	Version    string
	Extensions []string
	DryRun     bool
	PlanOnly   bool
}

// ProfileOptions controls php.ini profile application.
type ProfileOptions struct {
	Version  string
	Profile  ProfileName
	DryRun   bool
	PlanOnly bool
}
