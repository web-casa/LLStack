package php

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/web-casa/llstack/internal/config"
)

// Resolver resolves Remi package names and runtime paths.
type Resolver struct {
	cfg config.RuntimeConfig
}

// NewResolver constructs a PHP resolver.
func NewResolver(cfg config.RuntimeConfig) Resolver {
	return Resolver{cfg: cfg}
}

// ValidateVersion ensures the version is supported.
func (r Resolver) ValidateVersion(version string) error {
	for _, supported := range r.cfg.PHP.SupportedVersions {
		if version == supported {
			return nil
		}
	}
	return fmt.Errorf("unsupported php version %q", version)
}

// CollectionPrefix returns the Remi collection prefix for a version.
func (r Resolver) CollectionPrefix(version string) string {
	return "php" + strings.ReplaceAll(version, ".", "") + "-php"
}

// BasePackages returns the standard runtime package set.
func (r Resolver) BasePackages(version string, includeFPM, includeLSAPI bool) ([]string, error) {
	if err := r.ValidateVersion(version); err != nil {
		return nil, err
	}
	prefix := r.CollectionPrefix(version)
	pkgs := []string{
		prefix + "-cli",
		prefix + "-common",
		prefix + "-opcache",
	}
	if includeFPM {
		pkgs = append(pkgs, prefix+"-fpm")
	}
	if includeLSAPI {
		pkgs = append(pkgs, prefix+"-litespeed")
	}
	return pkgs, nil
}

// ExtensionPackages returns Remi extension packages.
func (r Resolver) ExtensionPackages(version string, extensions []string) ([]string, error) {
	if err := r.ValidateVersion(version); err != nil {
		return nil, err
	}
	prefix := r.CollectionPrefix(version)
	out := make([]string, 0, len(extensions))
	for _, ext := range extensions {
		name := strings.TrimSpace(ext)
		if name == "" {
			continue
		}
		out = append(out, prefix+"-"+name)
	}
	return out, nil
}

// FPMSocketPath returns the Remi php-fpm socket path.
func (r Resolver) FPMSocketPath(version string) string {
	return filepath.Join(r.cfg.PHP.StateRoot, "php"+strings.ReplaceAll(version, ".", ""), "run", "php-fpm", "www.sock")
}

// FPMServiceName returns the systemd service name for php-fpm.
func (r Resolver) FPMServiceName(version string) string {
	return "php" + strings.ReplaceAll(version, ".", "") + "-php-fpm"
}

// LSPHPCommand returns the lsphp command path for Remi php-litespeed.
func (r Resolver) LSPHPCommand(version string) string {
	return filepath.Join(r.cfg.PHP.RuntimeRoot, "php"+strings.ReplaceAll(version, ".", ""), "root", "usr", "bin", "lsphp")
}

// ManagedProfilePath returns the managed ini snippet path for a version.
func (r Resolver) ManagedProfilePath(version string) string {
	return filepath.Join(r.cfg.PHP.ProfileRoot, "php"+strings.ReplaceAll(version, ".", ""), "php.d", "90-llstack-profile.ini")
}

// RemiReleaseURL returns the remi-release RPM URL for the detected EL version.
func (r Resolver) RemiReleaseURL(elMajor string) string {
	return fmt.Sprintf(r.cfg.PHP.RemiReleaseRPMTemplate, elMajor)
}
