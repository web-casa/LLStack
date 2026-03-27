package docker_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerComposeConfigIsValid(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}

	repoRoot := mustRepoRoot(t)
	cmd := exec.Command("docker", "compose", "-f", "docker/compose/functional.yaml", "config", "-q")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose config failed: %v\n%s", err, string(output))
	}
}

func TestDockerComposeDeclaresExpectedServices(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}

	repoRoot := mustRepoRoot(t)
	cmd := exec.Command("docker", "compose", "-f", "docker/compose/functional.yaml", "config", "--services")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose config --services failed: %v\n%s", err, string(output))
	}

	services := strings.Fields(string(output))
	for _, want := range []string{"el9-apache", "el9-ols", "el9-lsws", "el10-apache", "el10-ols", "el10-lsws"} {
		if !containsService(services, want) {
			t.Fatalf("expected compose services to contain %q, got %v", want, services)
		}
	}
}

func TestDockerfilesBuildForTargetArch(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	for _, rel := range []string{
		"docker/images/el9-apache/Dockerfile",
		"docker/images/el9-ols/Dockerfile",
		"docker/images/el9-lsws/Dockerfile",
		"docker/images/el10-apache/Dockerfile",
		"docker/images/el10-ols/Dockerfile",
		"docker/images/el10-lsws/Dockerfile",
	} {
		data, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		content := string(data)
		if !strings.Contains(content, "ARG TARGETARCH") {
			t.Fatalf("%s should declare TARGETARCH", rel)
		}
		if strings.Contains(content, "GOARCH=amd64") {
			t.Fatalf("%s should not hardcode GOARCH=amd64", rel)
		}
		if strings.Contains(rel, "el10-") && !strings.Contains(content, "FROM rockylinux/rockylinux:10") {
			t.Fatalf("%s should use rockylinux/rockylinux:10", rel)
		}
	}
}

func TestDockerFunctionalSmoke(t *testing.T) {
	if os.Getenv("LLSTACK_RUN_DOCKER_TESTS") != "1" {
		t.Skip("set LLSTACK_RUN_DOCKER_TESTS=1 to run docker functional smoke")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	if !dockerDaemonAccessible() {
		t.Skip("docker daemon not accessible for current user")
	}

	repoRoot := mustRepoRoot(t)
	artifactsDir := filepath.Join(repoRoot, "dist", "docker-smoke-test")
	cmd := exec.Command("bash", "scripts/docker/functional-smoke.sh")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "LLSTACK_DOCKER_ARTIFACTS_DIR="+artifactsDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker functional smoke failed: %v\n%s", err, string(output))
	}
	for _, service := range []string{"el9-apache", "el9-ols", "el9-lsws", "el10-apache", "el10-ols", "el10-lsws"} {
		logPath := filepath.Join(artifactsDir, service+".log")
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read docker smoke artifact %s: %v", logPath, err)
		}
		if !strings.Contains(string(data), `"status": "passed"`) {
			t.Fatalf("expected docker smoke artifact to contain passed marker: %s\n%s", logPath, string(data))
		}
	}
}

func TestDockerFunctionalReportSummarizesArtifacts(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	artifactsDir := t.TempDir()

	for _, service := range []string{"el9-apache", "el9-ols", "el9-lsws", "el10-apache", "el10-ols", "el10-lsws"} {
		if err := os.WriteFile(filepath.Join(artifactsDir, service+".log"), []byte(`{"status": "passed"}`), 0o644); err != nil {
			t.Fatalf("write artifact for %s: %v", service, err)
		}
	}

	cmd := exec.Command("bash", "scripts/docker/functional-report.sh")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "LLSTACK_DOCKER_ARTIFACTS_DIR="+artifactsDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker functional report failed: %v\n%s", err, string(output))
	}
	if !strings.Contains(string(output), "overall_status: `passed`") {
		t.Fatalf("expected passed markdown summary, got:\n%s", string(output))
	}

	summaryPath := filepath.Join(artifactsDir, "summary.json")
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}

	var summary struct {
		OverallStatus string `json:"overall_status"`
		Services      []struct {
			Service string `json:"service"`
			Status  string `json:"status"`
		} `json:"services"`
	}
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatalf("unmarshal summary: %v\n%s", err, string(data))
	}
	if summary.OverallStatus != "passed" {
		t.Fatalf("expected overall_status passed, got %q", summary.OverallStatus)
	}
	if len(summary.Services) != 6 {
		t.Fatalf("expected 6 services in summary, got %d", len(summary.Services))
	}
}

func TestDockerFunctionalReportFailsForMissingMarker(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	artifactsDir := t.TempDir()

	for _, service := range []string{"el9-apache", "el9-ols", "el9-lsws", "el10-apache", "el10-ols", "el10-lsws"} {
		content := `{"status": "passed"}`
		if service == "el10-lsws" {
			content = `{"status": "failed"}`
		}
		if err := os.WriteFile(filepath.Join(artifactsDir, service+".log"), []byte(content), 0o644); err != nil {
			t.Fatalf("write artifact for %s: %v", service, err)
		}
	}

	cmd := exec.Command("bash", "scripts/docker/functional-report.sh")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "LLSTACK_DOCKER_ARTIFACTS_DIR="+artifactsDir)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected report script to fail when a service log is missing passed marker\n%s", string(output))
	}
	if !strings.Contains(string(output), "overall_status: `failed`") {
		t.Fatalf("expected failed markdown summary, got:\n%s", string(output))
	}
}

func dockerDaemonAccessible() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func containsService(services []string, want string) bool {
	for _, service := range services {
		if service == want {
			return true
		}
	}
	return false
}
