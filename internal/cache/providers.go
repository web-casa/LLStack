package cache

import (
	"fmt"

	"github.com/web-casa/llstack/internal/config"
)

// ProviderSpec captures install/runtime details for a cache provider.
type ProviderSpec struct {
	Name         ProviderName
	ServiceName  string
	Packages     []string
	ConfigPath   string
	Capabilities ProviderCapability
}

// ResolveProvider returns the managed cache provider spec.
func ResolveProvider(cfg config.RuntimeConfig, name ProviderName) (ProviderSpec, error) {
	switch name {
	case ProviderMemcached:
		return ProviderSpec{
			Name:        ProviderMemcached,
			ServiceName: "memcached",
			Packages:    []string{"memcached"},
			ConfigPath:  cfg.Cache.MemcachedConfigPath,
			Capabilities: ProviderCapability{
				Provider:            ProviderMemcached,
				SupportsPersistence: false,
				SupportsEviction:    true,
			},
		}, nil
	case ProviderRedis:
		return ProviderSpec{
			Name:        ProviderRedis,
			ServiceName: "redis",
			Packages:    []string{"redis"},
			ConfigPath:  cfg.Cache.RedisConfigPath,
			Capabilities: ProviderCapability{
				Provider:            ProviderRedis,
				SupportsPersistence: true,
				SupportsEviction:    true,
			},
		}, nil
	case ProviderValkey:
		return ProviderSpec{
			Name:        ProviderValkey,
			ServiceName: "valkey",
			Packages:    []string{"valkey"},
			ConfigPath:  cfg.Cache.ValkeyConfigPath,
			Capabilities: ProviderCapability{
				Provider:            ProviderValkey,
				SupportsPersistence: true,
				SupportsEviction:    true,
				Notes:               []string{"Valkey is a Redis-compatible fork; API and config format are identical to Redis"},
			},
		}, nil
	default:
		return ProviderSpec{}, fmt.Errorf("unsupported cache provider %q", name)
	}
}
