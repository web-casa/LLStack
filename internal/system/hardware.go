package system

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// HardwareInfo captures detected server hardware resources.
type HardwareInfo struct {
	CPUCores  int   `json:"cpu_cores"`
	MemoryMB  int64 `json:"memory_mb"`
	MemoryGB  float64 `json:"memory_gb"`
}

// DetectHardware returns CPU core count and total physical memory.
func DetectHardware() HardwareInfo {
	cores := runtime.NumCPU()
	memMB := detectMemoryMB()

	return HardwareInfo{
		CPUCores:  cores,
		MemoryMB:  memMB,
		MemoryGB:  float64(memMB) / 1024.0,
	}
}

// detectMemoryMB reads total memory from /proc/meminfo (Linux).
func detectMemoryMB() int64 {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 1024 // default fallback: 1GB
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseInt(fields[1], 10, 64)
				if err == nil {
					return kb / 1024
				}
			}
		}
	}
	return 1024
}
