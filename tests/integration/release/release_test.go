package release_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseBuildPackageInstallUpgradeAndSmoke(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	version := "0.1.0-test"
	platform := runtime.GOOS + "/" + runtime.GOARCH
	distDir := filepath.Join(t.TempDir(), "dist")
	pkgDir := filepath.Join(t.TempDir(), "packages")
	prefix := filepath.Join(t.TempDir(), "prefix")

	runScript(t, repoRoot, "scripts/release/build.sh", map[string]string{
		"LLSTACK_VERSION":   version,
		"LLSTACK_DIST_DIR":  distDir,
		"LLSTACK_PLATFORMS": platform,
	})

	binaryPath := filepath.Join(distDir, runtime.GOOS+"-"+runtime.GOARCH, "llstack")
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatalf("expected built binary: %v", err)
	}
	metadataPath := filepath.Join(distDir, "metadata.json")
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if !strings.Contains(string(metadata), `"version": "`+version+`"`) {
		t.Fatalf("expected metadata version %s, got %s", version, string(metadata))
	}

	runScript(t, repoRoot, "scripts/release/package.sh", map[string]string{
		"LLSTACK_VERSION":     version,
		"LLSTACK_DIST_DIR":    distDir,
		"LLSTACK_PACKAGE_DIR": pkgDir,
	})
	runScript(t, repoRoot, "scripts/release/verify.sh", map[string]string{
		"LLSTACK_VERSION":     version,
		"LLSTACK_PACKAGE_DIR": pkgDir,
	})

	archives, err := filepath.Glob(filepath.Join(pkgDir, "llstack-"+version+"-*.tar.gz"))
	if err != nil {
		t.Fatalf("glob package archives: %v", err)
	}
	if len(archives) != 1 {
		t.Fatalf("expected exactly one package archive, got %#v", archives)
	}
	archivePath := archives[0]
	checksumsPath := filepath.Join(pkgDir, "checksums.txt")
	checksumRaw, err := os.ReadFile(checksumsPath)
	if err != nil {
		t.Fatalf("read checksums: %v", err)
	}
	checksum := firstField(string(checksumRaw))
	if checksum == "" {
		t.Fatalf("expected checksum entry, got %q", string(checksumRaw))
	}
	indexPath := filepath.Join(pkgDir, "index.json")
	indexRaw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read package index: %v", err)
	}
	if !strings.Contains(string(indexRaw), filepath.Base(archivePath)) {
		t.Fatalf("expected package index to contain archive name, got %s", string(indexRaw))
	}
	sbomPath := filepath.Join(pkgDir, "sbom.spdx.json")
	sbomRaw, err := os.ReadFile(sbomPath)
	if err != nil {
		t.Fatalf("read sbom: %v", err)
	}
	if !strings.Contains(string(sbomRaw), `"spdxVersion": "SPDX-2.3"`) {
		t.Fatalf("expected SPDX version in sbom, got %s", string(sbomRaw))
	}
	if !strings.Contains(string(sbomRaw), filepath.Base(archivePath)) {
		t.Fatalf("expected sbom to describe archive %s, got %s", filepath.Base(archivePath), string(sbomRaw))
	}
	provenancePath := filepath.Join(pkgDir, "provenance.json")
	provenanceRaw, err := os.ReadFile(provenancePath)
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}
	for _, want := range []string{
		`"version": "` + version + `"`,
		`"sbom": "sbom.spdx.json"`,
		filepath.Base(archivePath),
		checksum,
		`"commit":`,
		`"ref":`,
		`"repository":`,
		`"go_version":`,
		`"build_os":`,
		`"build_arch":`,
	} {
		if !strings.Contains(string(provenanceRaw), want) {
			t.Fatalf("expected provenance to contain %s, got %s", want, string(provenanceRaw))
		}
	}

	runCommand(t, repoRoot, "bash", "scripts/install.sh", "--from", archivePath, "--prefix", prefix, "--sha256", checksum)
	installedBinary := filepath.Join(prefix, "bin", "llstack")
	if _, err := os.Stat(installedBinary); err != nil {
		t.Fatalf("expected installed binary: %v", err)
	}

	runCommand(t, repoRoot, "bash", "scripts/upgrade.sh", "--from", archivePath, "--prefix", prefix, "--sha256", checksum)
	matches, err := filepath.Glob(installedBinary + ".bak.*")
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected backup file after upgrade in %s", prefix)
	}

	runCommand(t, repoRoot, "bash", "tests/e2e/smoke.sh", installedBinary)

	if hasDownloader() {
		server := httptest.NewServer(http.FileServer(http.Dir(pkgDir)))
		defer server.Close()

		remotePrefix := filepath.Join(t.TempDir(), "remote-prefix")
		remoteURL := server.URL + "/" + filepath.Base(archivePath)
		runCommand(t, repoRoot, "bash", "scripts/install.sh", "--from", remoteURL, "--prefix", remotePrefix, "--sha256", checksum)
		remoteBinary := filepath.Join(remotePrefix, "bin", "llstack")
		if _, err := os.Stat(remoteBinary); err != nil {
			t.Fatalf("expected remotely installed binary: %v", err)
		}
		runCommand(t, repoRoot, "bash", "scripts/upgrade.sh", "--from", remoteURL, "--prefix", remotePrefix, "--sha256", checksum)

		indexPrefix := filepath.Join(t.TempDir(), "index-prefix")
		indexURL := server.URL + "/index.json"
		runCommand(t, repoRoot, "bash", "scripts/install-release.sh", "--index", indexURL, "--platform", runtime.GOOS+"-"+runtime.GOARCH, "--prefix", indexPrefix)
		indexBinary := filepath.Join(indexPrefix, "bin", "llstack")
		if _, err := os.Stat(indexBinary); err != nil {
			t.Fatalf("expected index-installed binary: %v", err)
		}
		runCommand(t, repoRoot, "bash", "scripts/install-release.sh", "--index", indexURL, "--platform", runtime.GOOS+"-"+runtime.GOARCH, "--prefix", indexPrefix, "--upgrade")
	}
}

func TestInstallReleaseVerifiesIndexAndArchiveSignatures(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl not available")
	}
	if !hasDownloader() {
		t.Skip("curl or wget required")
	}

	version := "0.1.0-test-install-release-signature"
	platform := runtime.GOOS + "/" + runtime.GOARCH
	distDir := filepath.Join(t.TempDir(), "dist")
	pkgDir := filepath.Join(t.TempDir(), "packages")
	prefix := filepath.Join(t.TempDir(), "prefix")
	keyDir := t.TempDir()
	privateKey := filepath.Join(keyDir, "release-private.pem")
	publicKey := filepath.Join(keyDir, "release-public.pem")

	runScript(t, repoRoot, "scripts/release/build.sh", map[string]string{
		"LLSTACK_VERSION":   version,
		"LLSTACK_DIST_DIR":  distDir,
		"LLSTACK_PLATFORMS": platform,
	})
	runScript(t, repoRoot, "scripts/release/package.sh", map[string]string{
		"LLSTACK_VERSION":     version,
		"LLSTACK_DIST_DIR":    distDir,
		"LLSTACK_PACKAGE_DIR": pkgDir,
	})
	runCommand(t, repoRoot, "openssl", "genpkey", "-algorithm", "RSA", "-pkeyopt", "rsa_keygen_bits:2048", "-out", privateKey)
	runCommand(t, repoRoot, "openssl", "rsa", "-in", privateKey, "-pubout", "-out", publicKey)
	runScript(t, repoRoot, "scripts/release/sign.sh", map[string]string{
		"LLSTACK_VERSION":        version,
		"LLSTACK_PACKAGE_DIR":    pkgDir,
		"LLSTACK_SIGNING_KEY":    privateKey,
		"LLSTACK_SIGNING_PUBKEY": publicKey,
	})

	server := httptest.NewServer(http.FileServer(http.Dir(pkgDir)))
	defer server.Close()

	runCommand(t, repoRoot, "bash", "scripts/install-release.sh",
		"--index", server.URL+"/index.json",
		"--platform", runtime.GOOS+"-"+runtime.GOARCH,
		"--prefix", prefix,
		"--pubkey", publicKey,
		"--require-signature",
	)
	if _, err := os.Stat(filepath.Join(prefix, "bin", "llstack")); err != nil {
		t.Fatalf("expected signed index install to succeed: %v", err)
	}
}

func TestInstallReleaseFailsOnIndexSignatureMismatch(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl not available")
	}
	if !hasDownloader() {
		t.Skip("curl or wget required")
	}

	version := "0.1.0-test-install-release-signature-bad-index"
	platform := runtime.GOOS + "/" + runtime.GOARCH
	distDir := filepath.Join(t.TempDir(), "dist")
	pkgDir := filepath.Join(t.TempDir(), "packages")
	keyDir := t.TempDir()
	privateKey := filepath.Join(keyDir, "release-private.pem")
	publicKey := filepath.Join(keyDir, "release-public.pem")

	runScript(t, repoRoot, "scripts/release/build.sh", map[string]string{
		"LLSTACK_VERSION":   version,
		"LLSTACK_DIST_DIR":  distDir,
		"LLSTACK_PLATFORMS": platform,
	})
	runScript(t, repoRoot, "scripts/release/package.sh", map[string]string{
		"LLSTACK_VERSION":     version,
		"LLSTACK_DIST_DIR":    distDir,
		"LLSTACK_PACKAGE_DIR": pkgDir,
	})
	runCommand(t, repoRoot, "openssl", "genpkey", "-algorithm", "RSA", "-pkeyopt", "rsa_keygen_bits:2048", "-out", privateKey)
	runCommand(t, repoRoot, "openssl", "rsa", "-in", privateKey, "-pubout", "-out", publicKey)
	runScript(t, repoRoot, "scripts/release/sign.sh", map[string]string{
		"LLSTACK_VERSION":        version,
		"LLSTACK_PACKAGE_DIR":    pkgDir,
		"LLSTACK_SIGNING_KEY":    privateKey,
		"LLSTACK_SIGNING_PUBKEY": publicKey,
	})

	indexPath := filepath.Join(pkgDir, "index.json")
	if err := os.WriteFile(indexPath, []byte(`{"version":"tampered","packages":[]}`), 0o644); err != nil {
		t.Fatalf("tamper index: %v", err)
	}

	server := httptest.NewServer(http.FileServer(http.Dir(pkgDir)))
	defer server.Close()

	cmd := exec.Command("bash", "scripts/install-release.sh",
		"--index", server.URL+"/index.json",
		"--platform", runtime.GOOS+"-"+runtime.GOARCH,
		"--prefix", filepath.Join(t.TempDir(), "prefix"),
		"--pubkey", publicKey,
		"--require-signature",
	)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected install-release to fail after tampered index signature, got success: %s", string(output))
	}
	if !strings.Contains(string(output), "signature verification failed for release index") {
		t.Fatalf("expected release index signature mismatch, got %s", string(output))
	}
}

func TestReleaseSignAndVerifyWithPublicKey(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl not available")
	}

	version := "0.1.0-test-sign"
	platform := runtime.GOOS + "/" + runtime.GOARCH
	distDir := filepath.Join(t.TempDir(), "dist")
	pkgDir := filepath.Join(t.TempDir(), "packages")
	keyDir := t.TempDir()
	privateKey := filepath.Join(keyDir, "release-private.pem")
	publicKey := filepath.Join(keyDir, "release-public.pem")

	runScript(t, repoRoot, "scripts/release/build.sh", map[string]string{
		"LLSTACK_VERSION":   version,
		"LLSTACK_DIST_DIR":  distDir,
		"LLSTACK_PLATFORMS": platform,
	})
	runScript(t, repoRoot, "scripts/release/package.sh", map[string]string{
		"LLSTACK_VERSION":     version,
		"LLSTACK_DIST_DIR":    distDir,
		"LLSTACK_PACKAGE_DIR": pkgDir,
	})

	runCommand(t, repoRoot, "openssl", "genpkey", "-algorithm", "RSA", "-pkeyopt", "rsa_keygen_bits:2048", "-out", privateKey)
	runCommand(t, repoRoot, "openssl", "rsa", "-in", privateKey, "-pubout", "-out", publicKey)

	runScript(t, repoRoot, "scripts/release/sign.sh", map[string]string{
		"LLSTACK_VERSION":        version,
		"LLSTACK_PACKAGE_DIR":    pkgDir,
		"LLSTACK_SIGNING_KEY":    privateKey,
		"LLSTACK_SIGNING_PUBKEY": publicKey,
	})
	runScript(t, repoRoot, "scripts/release/verify.sh", map[string]string{
		"LLSTACK_VERSION":            version,
		"LLSTACK_PACKAGE_DIR":        pkgDir,
		"LLSTACK_VERIFY_PUBKEY":      publicKey,
		"LLSTACK_REQUIRE_SIGNATURES": "1",
	})

	signaturesPath := filepath.Join(pkgDir, "signatures.json")
	raw, err := os.ReadFile(signaturesPath)
	if err != nil {
		t.Fatalf("read signatures manifest: %v", err)
	}
	for _, want := range []string{
		`"scheme": "openssl-rsa-sha256"`,
		`"public_key_hint": "` + filepath.Base(publicKey) + `"`,
		`"file":"checksums.txt"`,
		`"file":"index.json"`,
		`"file":"sbom.spdx.json"`,
		`"file":"provenance.json"`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("expected signatures manifest to contain %s, got %s", want, string(raw))
		}
	}
}

func TestReleaseVerifyFailsOnSignatureMismatch(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl not available")
	}

	version := "0.1.0-test-bad-signature"
	platform := runtime.GOOS + "/" + runtime.GOARCH
	distDir := filepath.Join(t.TempDir(), "dist")
	pkgDir := filepath.Join(t.TempDir(), "packages")
	keyDir := t.TempDir()
	privateKey := filepath.Join(keyDir, "release-private.pem")
	publicKey := filepath.Join(keyDir, "release-public.pem")

	runScript(t, repoRoot, "scripts/release/build.sh", map[string]string{
		"LLSTACK_VERSION":   version,
		"LLSTACK_DIST_DIR":  distDir,
		"LLSTACK_PLATFORMS": platform,
	})
	runScript(t, repoRoot, "scripts/release/package.sh", map[string]string{
		"LLSTACK_VERSION":     version,
		"LLSTACK_DIST_DIR":    distDir,
		"LLSTACK_PACKAGE_DIR": pkgDir,
	})

	runCommand(t, repoRoot, "openssl", "genpkey", "-algorithm", "RSA", "-pkeyopt", "rsa_keygen_bits:2048", "-out", privateKey)
	runCommand(t, repoRoot, "openssl", "rsa", "-in", privateKey, "-pubout", "-out", publicKey)

	runScript(t, repoRoot, "scripts/release/sign.sh", map[string]string{
		"LLSTACK_VERSION":        version,
		"LLSTACK_PACKAGE_DIR":    pkgDir,
		"LLSTACK_SIGNING_KEY":    privateKey,
		"LLSTACK_SIGNING_PUBKEY": publicKey,
	})

	checksumSig := filepath.Join(pkgDir, "checksums.txt.sig")
	if err := os.WriteFile(checksumSig, []byte("tampered-signature"), 0o644); err != nil {
		t.Fatalf("tamper detached signature: %v", err)
	}

	cmd := exec.Command("bash", "scripts/release/verify.sh")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"LLSTACK_VERSION="+version,
		"LLSTACK_PACKAGE_DIR="+pkgDir,
		"LLSTACK_VERIFY_PUBKEY="+publicKey,
		"LLSTACK_REQUIRE_SIGNATURES=1",
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected verify.sh to fail after signature tamper, got success: %s", string(output))
	}
	if !strings.Contains(string(output), "detached signature verification failed for checksums.txt") {
		t.Fatalf("expected signature mismatch error, got %s", string(output))
	}
}

func TestReleaseVerifyFailsOnIndexMismatch(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	version := "0.1.0-test-bad-index"
	platform := runtime.GOOS + "/" + runtime.GOARCH
	distDir := filepath.Join(t.TempDir(), "dist")
	pkgDir := filepath.Join(t.TempDir(), "packages")

	runScript(t, repoRoot, "scripts/release/build.sh", map[string]string{
		"LLSTACK_VERSION":   version,
		"LLSTACK_DIST_DIR":  distDir,
		"LLSTACK_PLATFORMS": platform,
	})
	runScript(t, repoRoot, "scripts/release/package.sh", map[string]string{
		"LLSTACK_VERSION":     version,
		"LLSTACK_DIST_DIR":    distDir,
		"LLSTACK_PACKAGE_DIR": pkgDir,
	})

	indexPath := filepath.Join(pkgDir, "index.json")
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	bad := strings.Replace(string(raw), `"sha256":"`, `"sha256":"broken-`, 1)
	if err := os.WriteFile(indexPath, []byte(bad), 0o644); err != nil {
		t.Fatalf("write bad index: %v", err)
	}

	cmd := exec.Command("bash", "scripts/release/verify.sh")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"LLSTACK_VERSION="+version,
		"LLSTACK_PACKAGE_DIR="+pkgDir,
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected verify.sh to fail after index mismatch, got success: %s", string(output))
	}
	if !strings.Contains(string(output), "index.json does not match checksum entry") {
		t.Fatalf("expected mismatch error, got %s", string(output))
	}
}

func TestReleaseVersionGuardAndNotesReportScripts(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	version := "v0.1.0-test-release-guard"
	platform := runtime.GOOS + "/" + runtime.GOARCH
	distDir := filepath.Join(t.TempDir(), "dist")
	pkgDir := filepath.Join(t.TempDir(), "packages")
	notesOut := filepath.Join(t.TempDir(), "release-notes.md")
	summaryOut := filepath.Join(t.TempDir(), "release-summary.md")
	summaryJSON := filepath.Join(t.TempDir(), "release-summary.json")
	assetsFile := filepath.Join(t.TempDir(), "release-assets.txt")
	urlFile := filepath.Join(t.TempDir(), "release-url.txt")

	runScript(t, repoRoot, "scripts/release/build.sh", map[string]string{
		"LLSTACK_VERSION":   version,
		"LLSTACK_DIST_DIR":  distDir,
		"LLSTACK_PLATFORMS": platform,
	})
	runScript(t, repoRoot, "scripts/release/package.sh", map[string]string{
		"LLSTACK_VERSION":     version,
		"LLSTACK_DIST_DIR":    distDir,
		"LLSTACK_PACKAGE_DIR": pkgDir,
	})
	runScript(t, repoRoot, "scripts/release/validate-version.sh", map[string]string{
		"LLSTACK_VERSION":          version,
		"LLSTACK_GIT_REF_NAME":     version,
		"LLSTACK_REQUIRE_V_PREFIX": "1",
		"LLSTACK_EXPECT_TAG_MATCH": "1",
	})
	runScript(t, repoRoot, "scripts/release/render-notes.sh", map[string]string{
		"LLSTACK_VERSION":           version,
		"LLSTACK_PACKAGE_DIR":       pkgDir,
		"LLSTACK_DIST_DIR":          distDir,
		"LLSTACK_RELEASE_NOTES_OUT": notesOut,
		"LLSTACK_GITHUB_REPOSITORY": "web-casa/llstack",
	})

	archives, err := filepath.Glob(filepath.Join(pkgDir, "llstack-"+version+"-*.tar.gz"))
	if err != nil {
		t.Fatalf("glob package archives: %v", err)
	}
	if len(archives) != 1 {
		t.Fatalf("expected exactly one package archive, got %#v", archives)
	}
	if err := os.WriteFile(assetsFile, []byte(strings.Join([]string{
		filepath.Base(archives[0]),
		"checksums.txt",
		"index.json",
		"sbom.spdx.json",
		"provenance.json",
	}, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write release assets file: %v", err)
	}
	if err := os.WriteFile(urlFile, []byte("https://github.com/web-casa/llstack/releases/tag/"+version+"\n"), 0o644); err != nil {
		t.Fatalf("write release url file: %v", err)
	}

	runScript(t, repoRoot, "scripts/release/post-release-report.sh", map[string]string{
		"LLSTACK_VERSION":                  version,
		"LLSTACK_PACKAGE_DIR":              pkgDir,
		"LLSTACK_RELEASE_ASSETS_FILE":      assetsFile,
		"LLSTACK_RELEASE_URL_FILE":         urlFile,
		"LLSTACK_RELEASE_SUMMARY_OUT":      summaryOut,
		"LLSTACK_RELEASE_SUMMARY_JSON_OUT": summaryJSON,
	})

	notesRaw, err := os.ReadFile(notesOut)
	if err != nil {
		t.Fatalf("read release notes: %v", err)
	}
	for _, want := range []string{
		version,
		filepath.Base(archives[0]),
		"https://github.com/web-casa/llstack/releases/download/" + version + "/index.json",
	} {
		if !strings.Contains(string(notesRaw), want) {
			t.Fatalf("expected release notes to contain %s, got %s", want, string(notesRaw))
		}
	}

	summaryRaw, err := os.ReadFile(summaryOut)
	if err != nil {
		t.Fatalf("read release summary: %v", err)
	}
	for _, want := range []string{
		"`passed`",
		"https://github.com/web-casa/llstack/releases/tag/" + version,
		filepath.Base(archives[0]),
	} {
		if !strings.Contains(string(summaryRaw), want) {
			t.Fatalf("expected release summary to contain %s, got %s", want, string(summaryRaw))
		}
	}

	summaryJSONRaw, err := os.ReadFile(summaryJSON)
	if err != nil {
		t.Fatalf("read release summary json: %v", err)
	}
	for _, want := range []string{
		`"status": "passed"`,
		`"version": "` + version + `"`,
		filepath.Base(archives[0]),
	} {
		if !strings.Contains(string(summaryJSONRaw), want) {
			t.Fatalf("expected release summary json to contain %s, got %s", want, string(summaryJSONRaw))
		}
	}
}

func TestReleaseVersionGuardFailsOnTagMismatch(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	cmd := exec.Command("bash", "scripts/release/validate-version.sh")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"LLSTACK_VERSION=v0.1.0",
		"LLSTACK_GIT_REF_NAME=v0.1.1",
		"LLSTACK_REQUIRE_V_PREFIX=1",
		"LLSTACK_EXPECT_TAG_MATCH=1",
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected validate-version.sh to fail on tag mismatch, got success: %s", string(output))
	}
	if !strings.Contains(string(output), "release version does not match current tag") {
		t.Fatalf("expected tag mismatch error, got %s", string(output))
	}
}

func TestPublishDirectoryProvider(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	version := "0.1.0-test-publish-dir"
	platform := runtime.GOOS + "/" + runtime.GOARCH
	distDir := filepath.Join(t.TempDir(), "dist")
	pkgDir := filepath.Join(t.TempDir(), "packages")
	publishDir := filepath.Join(t.TempDir(), "published")

	runScript(t, repoRoot, "scripts/release/build.sh", map[string]string{
		"LLSTACK_VERSION":   version,
		"LLSTACK_DIST_DIR":  distDir,
		"LLSTACK_PLATFORMS": platform,
	})
	runScript(t, repoRoot, "scripts/release/package.sh", map[string]string{
		"LLSTACK_VERSION":     version,
		"LLSTACK_DIST_DIR":    distDir,
		"LLSTACK_PACKAGE_DIR": pkgDir,
	})

	assetsOut := filepath.Join(pkgDir, "release-assets.txt")
	urlOut := filepath.Join(pkgDir, "release-url.txt")

	runScript(t, repoRoot, "scripts/release/publish.sh", map[string]string{
		"LLSTACK_VERSION":            version,
		"LLSTACK_PACKAGE_DIR":        pkgDir,
		"LLSTACK_PUBLISH_PROVIDER":   "directory",
		"LLSTACK_PUBLISH_TARGET":     publishDir,
		"LLSTACK_PUBLISH_ASSETS_OUT": assetsOut,
		"LLSTACK_PUBLISH_URL_OUT":    urlOut,
	})

	// Verify target directory has assets
	entries, err := os.ReadDir(publishDir)
	if err != nil {
		t.Fatalf("read publish dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected published assets in %s", publishDir)
	}
	hasArchive := false
	hasIndex := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tar.gz") {
			hasArchive = true
		}
		if e.Name() == "index.json" {
			hasIndex = true
		}
	}
	if !hasArchive {
		t.Fatalf("expected archive in published dir")
	}
	if !hasIndex {
		t.Fatalf("expected index.json in published dir")
	}

	// Verify output files
	urlRaw, err := os.ReadFile(urlOut)
	if err != nil {
		t.Fatalf("read url output: %v", err)
	}
	if !strings.HasPrefix(string(urlRaw), "file://") {
		t.Fatalf("expected file:// URL, got %s", string(urlRaw))
	}

	assetsRaw, err := os.ReadFile(assetsOut)
	if err != nil {
		t.Fatalf("read assets output: %v", err)
	}
	if !strings.Contains(string(assetsRaw), "index.json") {
		t.Fatalf("expected assets listing to contain index.json, got %s", string(assetsRaw))
	}
	if !strings.Contains(string(assetsRaw), "checksums.txt") {
		t.Fatalf("expected assets listing to contain checksums.txt, got %s", string(assetsRaw))
	}
}

func TestPublishDirectoryWithPipelineIntegration(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	if !hasDownloader() {
		t.Skip("curl or wget required")
	}

	version := "v0.1.0-test-pipeline-publish"
	platform := runtime.GOOS + "/" + runtime.GOARCH
	distDir := filepath.Join(t.TempDir(), "dist")
	pkgDir := filepath.Join(t.TempDir(), "packages")
	publishDir := filepath.Join(t.TempDir(), "published")

	runScript(t, repoRoot, "scripts/release/pipeline.sh", map[string]string{
		"LLSTACK_VERSION":           version,
		"LLSTACK_DIST_DIR":          distDir,
		"LLSTACK_PACKAGE_DIR":       pkgDir,
		"LLSTACK_PLATFORMS":         platform,
		"LLSTACK_REQUIRE_V_PREFIX":  "1",
		"LLSTACK_RUN_TESTS":         "0",
		"LLSTACK_RUN_SIGN":          "0",
		"LLSTACK_RUN_PUBLISH":       "1",
		"LLSTACK_PUBLISH_PROVIDER":  "directory",
		"LLSTACK_PUBLISH_TARGET":    publishDir,
		"LLSTACK_GIT_REF_NAME":      "",
		"LLSTACK_GITHUB_REPOSITORY": "web-casa/llstack",
	})

	// Verify publish happened
	entries, err := os.ReadDir(publishDir)
	if err != nil {
		t.Fatalf("read publish dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected published assets in %s", publishDir)
	}

	// Verify remote-verify works against published directory
	remoteJSON := filepath.Join(t.TempDir(), "remote-verify.json")
	server := httptest.NewServer(http.FileServer(http.Dir(publishDir)))
	defer server.Close()

	runScript(t, repoRoot, "scripts/release/verify-remote.sh", map[string]string{
		"LLSTACK_VERSION":                version,
		"LLSTACK_REMOTE_BASE_URL":        server.URL,
		"LLSTACK_REMOTE_VERIFY_JSON_OUT": remoteJSON,
	})

	remoteRaw, err := os.ReadFile(remoteJSON)
	if err != nil {
		t.Fatalf("read remote verify json: %v", err)
	}
	if !strings.Contains(string(remoteRaw), `"status": "passed"`) {
		t.Fatalf("expected remote verify success, got %s", string(remoteRaw))
	}
}

func TestPublishFailsWithoutProvider(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	cmd := exec.Command("bash", "scripts/release/publish.sh")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"LLSTACK_VERSION=v0.1.0",
		"LLSTACK_PUBLISH_PROVIDER=",
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected publish.sh to fail without provider, got success: %s", string(output))
	}
	if !strings.Contains(string(output), "LLSTACK_PUBLISH_PROVIDER is required") {
		t.Fatalf("expected provider required error, got %s", string(output))
	}
}

func TestReleasePipelineAndRemoteVerifyScripts(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	if !hasDownloader() {
		t.Skip("curl or wget required")
	}

	version := "v0.1.0-test-pipeline"
	platform := runtime.GOOS + "/" + runtime.GOARCH
	distDir := filepath.Join(t.TempDir(), "dist")
	pkgDir := filepath.Join(t.TempDir(), "packages")
	notesPath := filepath.Join(distDir, "release-notes.md")
	remoteJSON := filepath.Join(t.TempDir(), "remote-verify.json")
	remoteMD := filepath.Join(t.TempDir(), "remote-verify.md")

	runScript(t, repoRoot, "scripts/release/pipeline.sh", map[string]string{
		"LLSTACK_VERSION":           version,
		"LLSTACK_DIST_DIR":          distDir,
		"LLSTACK_PACKAGE_DIR":       pkgDir,
		"LLSTACK_PLATFORMS":         platform,
		"LLSTACK_REQUIRE_V_PREFIX":  "1",
		"LLSTACK_RUN_TESTS":         "0",
		"LLSTACK_RUN_SIGN":          "0",
		"LLSTACK_GIT_REF_NAME":      "",
		"LLSTACK_GITHUB_REPOSITORY": "web-casa/llstack",
	})

	if _, err := os.Stat(notesPath); err != nil {
		t.Fatalf("expected pipeline to render release notes: %v", err)
	}

	server := httptest.NewServer(http.FileServer(http.Dir(pkgDir)))
	defer server.Close()

	runScript(t, repoRoot, "scripts/release/verify-remote.sh", map[string]string{
		"LLSTACK_VERSION":                version,
		"LLSTACK_REMOTE_BASE_URL":        server.URL,
		"LLSTACK_REMOTE_VERIFY_JSON_OUT": remoteJSON,
		"LLSTACK_REMOTE_VERIFY_MD_OUT":   remoteMD,
	})

	remoteRaw, err := os.ReadFile(remoteJSON)
	if err != nil {
		t.Fatalf("read remote verify json: %v", err)
	}
	if !strings.Contains(string(remoteRaw), `"status": "passed"`) {
		t.Fatalf("expected remote verify success, got %s", string(remoteRaw))
	}
	mdRaw, err := os.ReadFile(remoteMD)
	if err != nil {
		t.Fatalf("read remote verify markdown: %v", err)
	}
	if !strings.Contains(string(mdRaw), server.URL) {
		t.Fatalf("expected remote verify markdown to reference remote base url, got %s", string(mdRaw))
	}
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func runScript(t *testing.T, repoRoot string, script string, env map[string]string) {
	t.Helper()
	cmd := exec.Command("bash", script)
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s: %v\n%s", script, err, string(output))
	}
}

func runCommand(t *testing.T, repoRoot string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s %s: %v\n%s", name, strings.Join(args, " "), err, string(output))
	}
}

func hasDownloader() bool {
	if _, err := exec.LookPath("curl"); err == nil {
		return true
	}
	if _, err := exec.LookPath("wget"); err == nil {
		return true
	}
	return false
}

func firstField(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
