package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/web-casa/llstack/internal/core/plan"
	"github.com/web-casa/llstack/internal/system"
)

// TuneOptions controls database parameter tuning.
type TuneOptions struct {
	Provider ProviderName
	DryRun   bool
	Force    bool // overwrite existing tuning config
}

// TuneResult captures the tuning outcome.
type TuneResult struct {
	Provider   string            `json:"provider"`
	ConfigPath string            `json:"config_path"`
	Parameters map[string]string `json:"parameters"`
}

// Tune generates hardware-aware database configuration.
func (m Manager) Tune(ctx context.Context, opts TuneOptions) (plan.Plan, TuneResult, error) {
	p := plan.New("db.tune", fmt.Sprintf("Tune %s parameters", opts.Provider))
	p.DryRun = opts.DryRun

	hw := system.DetectHardware()
	ramMB := hw.MemoryMB
	dbRAM := ramMB * 25 / 100 // 25% of total RAM for DB
	cores := hw.CPUCores

	spec, err := ResolveProvider(m.cfg, opts.Provider, "")
	if err != nil {
		return p, TuneResult{}, err
	}

	result := TuneResult{Provider: string(opts.Provider), Parameters: map[string]string{}}

	var configPath, content string

	switch spec.Family {
	case "mysql":
		configPath = fmt.Sprintf("/etc/my.cnf.d/llstack-%s-tune.cnf", opts.Provider)
		bufferPool := max64(dbRAM, 128)
		maxConn := max64(int64(cores)*25, 50)
		if maxConn > 500 {
			maxConn = 500
		}
		logFileSize := max64(bufferPool/4, 48)
		tmpTableSize := max64(ramMB*2/100, 32)

		params := map[string]string{
			"innodb_buffer_pool_size": fmt.Sprintf("%dM", bufferPool),
			"innodb_log_file_size":    fmt.Sprintf("%dM", logFileSize),
			"max_connections":         fmt.Sprintf("%d", maxConn),
			"tmp_table_size":          fmt.Sprintf("%dM", tmpTableSize),
			"max_heap_table_size":     fmt.Sprintf("%dM", tmpTableSize),
			"innodb_flush_log_at_trx_commit": "2",
			"innodb_flush_method":     "O_DIRECT",
			"key_buffer_size":         "32M",
			"table_open_cache":        fmt.Sprintf("%d", max64(maxConn*2, 400)),
			"sort_buffer_size":        "2M",
			"read_buffer_size":        "2M",
			"join_buffer_size":        "2M",
		}
		result.Parameters = params

		lines := []string{"# Managed by LLStack db:tune", "[mysqld]"}
		for k, v := range params {
			lines = append(lines, fmt.Sprintf("%s = %s", k, v))
		}
		content = strings.Join(lines, "\n") + "\n"

	case "postgresql":
		configPath = "/var/lib/pgsql/data/conf.d/llstack-tune.conf"
		if _, err := os.Stat("/var/lib/pgsql/16/data"); err == nil {
			configPath = "/var/lib/pgsql/16/data/conf.d/llstack-tune.conf"
		}
		sharedBuffers := max64(dbRAM, 128)
		effectiveCache := max64(ramMB*50/100, 256)
		workMem := max64(ramMB/100, 4)
		maintWorkMem := max64(dbRAM/4, 64)
		maxConn := max64(int64(cores)*25, 50)

		params := map[string]string{
			"shared_buffers":         fmt.Sprintf("%dMB", sharedBuffers),
			"effective_cache_size":   fmt.Sprintf("%dMB", effectiveCache),
			"work_mem":               fmt.Sprintf("%dMB", workMem),
			"maintenance_work_mem":   fmt.Sprintf("%dMB", maintWorkMem),
			"max_connections":        fmt.Sprintf("%d", maxConn),
			"checkpoint_completion_target": "0.9",
			"wal_buffers":            "16MB",
			"random_page_cost":       "1.1",
			"effective_io_concurrency": fmt.Sprintf("%d", max64(int64(cores)*2, 2)),
		}
		result.Parameters = params

		lines := []string{"# Managed by LLStack db:tune"}
		for k, v := range params {
			lines = append(lines, fmt.Sprintf("%s = %s", k, v))
		}
		content = strings.Join(lines, "\n") + "\n"

	default:
		return p, result, fmt.Errorf("tuning not supported for %s", spec.Family)
	}

	result.ConfigPath = configPath
	p.AddOperation(plan.Operation{
		ID: "write-tune-config", Kind: "file.write", Target: configPath,
		Details: result.Parameters,
	})

	if opts.DryRun {
		return p, result, nil
	}

	// Check existing
	if !opts.Force {
		if _, err := os.Stat(configPath); err == nil {
			return p, result, fmt.Errorf("tuning config already exists at %s; use --force to overwrite", configPath)
		}
	}

	os.MkdirAll(filepath.Dir(configPath), 0o755)
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		return p, result, err
	}

	// Restart DB service
	if spec.ServiceName != "" {
		m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"restart", spec.ServiceName}})
	}

	return p, result, nil
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
