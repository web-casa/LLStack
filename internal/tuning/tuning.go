package tuning

import (
	"github.com/web-casa/llstack/internal/system"
)

// Profile holds the calculated tuning parameters for all components.
type Profile struct {
	Hardware system.HardwareInfo `json:"hardware"`

	// PHP-FPM
	PHPMaxChildrenTotal int `json:"php_max_children_total"`
	PHPMaxChildrenSite  int `json:"php_max_children_site"`
	PHPStartServers     int `json:"php_start_servers"`
	PHPMinSpare         int `json:"php_min_spare"`
	PHPMaxSpare         int `json:"php_max_spare"`

	// Apache
	ApacheMaxRequestWorkers int `json:"apache_max_request_workers"`
	ApacheServerLimit       int `json:"apache_server_limit"`

	// OLS / LSWS
	OLSMaxConns     int `json:"ols_max_conns"`
	OLSLSAPIChildren int `json:"ols_lsapi_children"`
	LSWSMaxConn     int `json:"lsws_max_conn"`

	// Database
	DBBufferPoolMB  int `json:"db_buffer_pool_mb"`
	DBMaxConnections int `json:"db_max_connections"`
	PGSharedBuffersMB int `json:"pg_shared_buffers_mb"`
	PGEffectiveCacheMB int `json:"pg_effective_cache_mb"`

	// Cache
	RedisMaxMemoryMB int `json:"redis_max_memory_mb"`
	MemcachedCacheMB int `json:"memcached_cache_mb"`
}

// Calculate generates tuning parameters based on hardware and site count.
func Calculate(hw system.HardwareInfo, siteCount int) Profile {
	if siteCount < 1 {
		siteCount = 1
	}

	ramMB := hw.MemoryMB
	if ramMB < 512 {
		ramMB = 512
	}
	cores := hw.CPUCores
	if cores < 1 {
		cores = 1
	}

	// Allocation ratios
	phpMB := ramMB * 40 / 100
	dbMB := ramMB * 25 / 100
	cacheMB := ramMB * 10 / 100
	// webMB := ramMB * 10 / 100  (used implicitly)
	// osMB := ramMB * 15 / 100   (reserved)

	// PHP-FPM
	workerMem := int64(100) // MB per worker (average)
	totalChildren := int(phpMB / workerMem)
	if totalChildren < 3 {
		totalChildren = 3
	}
	perSiteChildren := totalChildren / siteCount
	if perSiteChildren < 3 {
		perSiteChildren = 3
	}
	if perSiteChildren > 50 {
		perSiteChildren = 50
	}
	startServers := max(1, perSiteChildren/4)
	minSpare := 1
	maxSpare := max(2, perSiteChildren/2)

	// Apache
	apacheWorkers := min(int(ramMB/20), cores*50)
	if apacheWorkers < 25 {
		apacheWorkers = 25
	}
	if apacheWorkers > 400 {
		apacheWorkers = 400
	}

	// OLS / LSWS
	olsConns := perSiteChildren
	if olsConns < 5 {
		olsConns = 5
	}
	lswsMaxConn := perSiteChildren
	if lswsMaxConn < 5 {
		lswsMaxConn = 5
	}

	// Database
	bufferPool := int(dbMB)
	if bufferPool < 128 {
		bufferPool = 128
	}
	dbMaxConn := max(50, min(500, cores*25))
	pgShared := int(dbMB)
	pgEffective := int(ramMB * 50 / 100)

	// Cache
	redisMem := int(cacheMB)
	if redisMem < 64 {
		redisMem = 64
	}
	memcachedMem := int(cacheMB)
	if memcachedMem < 64 {
		memcachedMem = 64
	}

	return Profile{
		Hardware:             hw,
		PHPMaxChildrenTotal:  totalChildren,
		PHPMaxChildrenSite:   perSiteChildren,
		PHPStartServers:      startServers,
		PHPMinSpare:          minSpare,
		PHPMaxSpare:          maxSpare,
		ApacheMaxRequestWorkers: apacheWorkers,
		ApacheServerLimit:    apacheWorkers,
		OLSMaxConns:          olsConns,
		OLSLSAPIChildren:     olsConns,
		LSWSMaxConn:          lswsMaxConn,
		DBBufferPoolMB:       bufferPool,
		DBMaxConnections:     dbMaxConn,
		PGSharedBuffersMB:    pgShared,
		PGEffectiveCacheMB:   pgEffective,
		RedisMaxMemoryMB:     redisMem,
		MemcachedCacheMB:     memcachedMem,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
