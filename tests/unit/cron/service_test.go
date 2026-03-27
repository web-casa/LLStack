package cron_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/web-casa/llstack/internal/cron"
)

func TestAddAndListCronJob(t *testing.T) {
	cronDir := filepath.Join(t.TempDir(), "cron.d")
	manifestDir := filepath.Join(t.TempDir(), "manifests")
	mgr := cron.NewManager(cronDir, manifestDir)

	job, err := mgr.Add("test.example.com", "testuser", "*/5 * * * *", "echo hello", "")
	if err != nil {
		t.Fatalf("add cron: %v", err)
	}
	if job.ID == "" || job.Site != "test.example.com" {
		t.Fatalf("unexpected job: %+v", job)
	}

	jobs, err := mgr.List("test.example.com")
	if err != nil {
		t.Fatalf("list cron: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != job.ID {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	// Verify cron.d file written
	entries, _ := os.ReadDir(cronDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 cron.d file, got %d", len(entries))
	}
}

func TestRemoveCronJob(t *testing.T) {
	cronDir := filepath.Join(t.TempDir(), "cron.d")
	manifestDir := filepath.Join(t.TempDir(), "manifests")
	mgr := cron.NewManager(cronDir, manifestDir)

	job, _ := mgr.Add("test.example.com", "testuser", "0 * * * *", "echo test", "")
	if err := mgr.Remove("test.example.com", job.ID); err != nil {
		t.Fatalf("remove cron: %v", err)
	}

	jobs, _ := mgr.List("test.example.com")
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs after removal, got %d", len(jobs))
	}
}

func TestAddPresetWPCron(t *testing.T) {
	cronDir := filepath.Join(t.TempDir(), "cron.d")
	manifestDir := filepath.Join(t.TempDir(), "manifests")
	mgr := cron.NewManager(cronDir, manifestDir)

	job, err := mgr.AddPreset("wp.example.com", "wpuser", "/data/www/wp.example.com", "wp-cron")
	if err != nil {
		t.Fatalf("add preset: %v", err)
	}
	if job.Preset != "wp-cron" {
		t.Fatalf("expected wp-cron preset, got %s", job.Preset)
	}
	if job.Schedule != "*/5 * * * *" {
		t.Fatalf("expected */5 schedule, got %s", job.Schedule)
	}
}

func TestAddPresetLaravelScheduler(t *testing.T) {
	cronDir := filepath.Join(t.TempDir(), "cron.d")
	manifestDir := filepath.Join(t.TempDir(), "manifests")
	mgr := cron.NewManager(cronDir, manifestDir)

	job, err := mgr.AddPreset("laravel.example.com", "laraveluser", "/data/www/laravel.example.com", "laravel-scheduler")
	if err != nil {
		t.Fatalf("add preset: %v", err)
	}
	if job.Preset != "laravel-scheduler" || job.Schedule != "* * * * *" {
		t.Fatalf("unexpected preset job: %+v", job)
	}
}

func TestListAllSites(t *testing.T) {
	cronDir := filepath.Join(t.TempDir(), "cron.d")
	manifestDir := filepath.Join(t.TempDir(), "manifests")
	mgr := cron.NewManager(cronDir, manifestDir)

	mgr.Add("site1.com", "user1", "0 0 * * *", "echo 1", "")
	mgr.Add("site2.com", "user2", "0 0 * * *", "echo 2", "")

	jobs, err := mgr.List("")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs across sites, got %d", len(jobs))
	}
}
