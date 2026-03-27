package system_test

import (
	"testing"

	"github.com/web-casa/llstack/internal/system"
	"github.com/web-casa/llstack/internal/tuning"
)

func TestDetectHardware(t *testing.T) {
	hw := system.DetectHardware()
	if hw.CPUCores < 1 {
		t.Fatalf("expected at least 1 CPU core, got %d", hw.CPUCores)
	}
	if hw.MemoryMB < 256 {
		t.Fatalf("expected at least 256MB memory, got %d", hw.MemoryMB)
	}
	if hw.MemoryGB <= 0 {
		t.Fatalf("expected positive MemoryGB, got %f", hw.MemoryGB)
	}
}

func TestTuningCalculation1GB1Site(t *testing.T) {
	hw := system.HardwareInfo{CPUCores: 1, MemoryMB: 1024, MemoryGB: 1.0}
	p := tuning.Calculate(hw, 1)

	if p.PHPMaxChildrenSite < 3 {
		t.Fatalf("expected min 3 PHP children, got %d", p.PHPMaxChildrenSite)
	}
	if p.DBBufferPoolMB < 128 {
		t.Fatalf("expected min 128MB buffer pool, got %d", p.DBBufferPoolMB)
	}
	if p.ApacheMaxRequestWorkers < 25 {
		t.Fatalf("expected min 25 Apache workers, got %d", p.ApacheMaxRequestWorkers)
	}
	if p.RedisMaxMemoryMB < 64 {
		t.Fatalf("expected min 64MB Redis, got %d", p.RedisMaxMemoryMB)
	}
}

func TestTuningCalculation8GB5Sites(t *testing.T) {
	hw := system.HardwareInfo{CPUCores: 4, MemoryMB: 8192, MemoryGB: 8.0}
	p := tuning.Calculate(hw, 5)

	// PHP: 8192*0.4 = 3276MB, /100 = 32 total, /5 = 6 per site
	if p.PHPMaxChildrenSite < 5 {
		t.Fatalf("expected PHP children >= 5 for 8GB/5sites, got %d", p.PHPMaxChildrenSite)
	}
	// DB: 8192*0.25 = 2048MB
	if p.DBBufferPoolMB < 1500 {
		t.Fatalf("expected buffer pool >= 1500MB for 8GB, got %d", p.DBBufferPoolMB)
	}
	// Apache: min(8192/20, 4*50) = min(409, 200) = 200
	if p.ApacheMaxRequestWorkers < 100 {
		t.Fatalf("expected Apache workers >= 100 for 8GB/4cores, got %d", p.ApacheMaxRequestWorkers)
	}
}

func TestTuningCalculation32GB20Sites(t *testing.T) {
	hw := system.HardwareInfo{CPUCores: 8, MemoryMB: 32768, MemoryGB: 32.0}
	p := tuning.Calculate(hw, 20)

	// PHP: 32768*0.4 = 13107MB, /100 = 131 total, /20 = 6 per site
	if p.PHPMaxChildrenSite < 3 {
		t.Fatalf("expected min 3 PHP children per site, got %d", p.PHPMaxChildrenSite)
	}
	if p.PHPMaxChildrenSite > 50 {
		t.Fatalf("expected max 50 PHP children per site, got %d", p.PHPMaxChildrenSite)
	}
	// DB: 32768*0.25 = 8192MB
	if p.DBBufferPoolMB < 5000 {
		t.Fatalf("expected buffer pool >= 5000MB for 32GB, got %d", p.DBBufferPoolMB)
	}
}
