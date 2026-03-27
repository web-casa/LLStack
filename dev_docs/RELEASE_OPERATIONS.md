# Release Operations

## Scope

This document describes the current release/operator workflow for LLStack:

- build
- package
- sign
- verify
- version-guard
- release-notes
- release-summary
- remote-verify
- provider-neutral release pipeline
- install
- upgrade
- install-from-index
- host smoke
- Docker functional smoke
- GitHub Actions CI skeleton
- tag-driven GitHub release workflow

It reflects the current repository state and should stay aligned with the release scripts under `scripts/`.

## Build

Build the host platform:

```bash
make build
```

Build multiple platforms:

```bash
make build-cross PLATFORMS="linux/amd64 linux/arm64"
```

Output layout:

- `dist/releases/<version>/<platform>/llstack`

Build metadata is injected with `ldflags` and surfaced by:

```bash
llstack version
llstack version --json
```

## Package

Create release archives and metadata:

```bash
make package
```

Output layout:

- `dist/packages/<version>/llstack-<version>-<platform>.tar.gz`
- `dist/packages/<version>/checksums.txt`
- `dist/packages/<version>/index.json`
- `dist/packages/<version>/sbom.spdx.json`
- `dist/packages/<version>/provenance.json`

`index.json` is the machine-readable entry point for platform-aware installation.
`sbom.spdx.json` is the current SPDX SBOM artifact for the packaged release set.
`provenance.json` is the release provenance manifest with build context (git commit, Go version, build platform).

## Version Guard

Validate a release/tag version before publishing:

```bash
make validate-release-version VERSION=v0.1.0
```

This delegates to `scripts/release/validate-version.sh`.

Current guard behavior:

- validates semver-like release strings
- can require `v`-prefixed tag versions
- can require the resolved version to match the current tag/ref
- can require that a git tag already exists before a hosted publish step runs

## Sign

Create detached signatures for archives and release metadata:

```bash
make sign-release SIGNING_KEY=/path/to/release-private.pem SIGNING_PUBKEY=/path/to/release-public.pem
```

This delegates to `scripts/release/sign.sh` and currently uses:

- `openssl dgst -sha256 -sign`

Output layout after signing:

- `dist/packages/<version>/*.sig`
- `dist/packages/<version>/signatures.json`

`signatures.json` describes the detached signature set and the public-key hint.

## Verify

Validate package checksums:

```bash
make verify-release
```

This delegates to `scripts/release/verify.sh` and verifies `checksums.txt` against packaged archives.
It also cross-checks `checksums.txt` against `index.json`, `sbom.spdx.json`, and `provenance.json`.

If signatures are present and a public key is provided, verify detached signatures:

```bash
LLSTACK_VERIFY_PUBKEY=/path/to/release-public.pem \
LLSTACK_REQUIRE_SIGNATURES=1 \
make verify-release
```

Without `LLSTACK_VERIFY_PUBKEY`, `verify.sh` still validates checksums and metadata consistency, but will only note that signatures were not cryptographically verified.

## Release Notes

Render release notes from the repository template:

```bash
make release-notes VERSION=v0.1.0
```

This delegates to `scripts/release/render-notes.sh` and uses:

- `.github/release-notes.md`

By default it writes:

- `dist/releases/<version>/release-notes.md`

The template is filled from packaged artifacts and release metadata, so the generated notes stay aligned with the actual release bundle.

## Release Summary

Write a post-release verification summary:

```bash
make release-summary VERSION=v0.1.0
```

This delegates to `scripts/release/post-release-report.sh`.

By default it writes:

- `dist/packages/<version>/release-summary.md`
- `dist/packages/<version>/release-summary.json`

When remote release asset listings are provided, it compares the published release asset names against the expected local package directory and fails on missing assets.

## Remote Verify

Fetch a published release bundle and verify it with the normal local verifier:

```bash
LLSTACK_VERSION=v0.1.0 \
LLSTACK_REMOTE_BASE_URL=https://github.com/example/llstack/releases/download/v0.1.0 \
make remote-verify-release
```

This delegates to `scripts/release/verify-remote.sh`.

Current behavior:

- downloads `index.json`, `checksums.txt`, `sbom.spdx.json`, and `provenance.json`
- downloads packaged archives listed by the remote `index.json`
- downloads detached signatures when present
- reuses `scripts/release/verify.sh` against the fetched remote bundle
- emits `remote-verify.json` and `remote-verify.md`

## Provider-Neutral Release Pipeline

The repository now also exposes a provider-neutral orchestration entrypoint:

```bash
make release-pipeline VERSION=v0.1.0 MODE=validate
```

This delegates to `scripts/release/pipeline.sh`.

Current modes:

- `validate`
- `release`

Current behavior:

- validates the release version/tag guard
- runs `go test ./...`
- runs `go build ./...`
- runs build/package
- optionally runs detached signing
- runs package verification
- renders release notes

GitHub Actions now uses the same script rather than maintaining a separate release implementation path.

## Install

Install from a local archive or binary:

```bash
bash scripts/install.sh --from dist/packages/<version>/llstack-<version>-linux-amd64.tar.gz
```

Install from a remote URL:

```bash
bash scripts/install.sh --from https://example.invalid/llstack-<version>-linux-amd64.tar.gz
```

Install with checksum verification:

```bash
bash scripts/install.sh \
  --from https://example.invalid/llstack-<version>-linux-amd64.tar.gz \
  --sha256 <hex>
```

Important behavior:

- existing binaries are backed up by default
- `curl` or `wget` is required for remote downloads
- `sha256sum` or `shasum` is required when `--sha256` is used
- `openssl` plus `--pubkey` can be used to verify detached signatures

Install with detached signature verification:

```bash
bash scripts/install.sh \
  --from https://example.invalid/llstack-<version>-linux-amd64.tar.gz \
  --sha256 <hex> \
  --pubkey /path/to/release-public.pem \
  --require-signature
```

## Upgrade

Upgrade uses the same source-resolution path as install:

```bash
bash scripts/upgrade.sh --from dist/packages/<version>/llstack-<version>-linux-amd64.tar.gz
```

Upgrade from a remote URL:

```bash
bash scripts/upgrade.sh \
  --from https://example.invalid/llstack-<version>-linux-amd64.tar.gz \
  --sha256 <hex>
```

If no existing install is found at `<prefix>/bin/llstack`, the script falls back to a fresh install and prints that fact.

## Install From Release Index

Install from `index.json`:

```bash
bash scripts/install-release.sh \
  --index dist/packages/<version>/index.json \
  --platform linux-amd64
```

Upgrade from `index.json`:

```bash
bash scripts/install-release.sh \
  --index https://example.invalid/index.json \
  --platform linux-amd64 \
  --upgrade
```

Platform autodetection is supported when `--platform` is omitted.

Index install can also verify detached signatures for both `index.json` and the selected archive:

```bash
bash scripts/install-release.sh \
  --index https://example.invalid/index.json \
  --platform linux-amd64 \
  --pubkey /path/to/release-public.pem \
  --require-signature
```

## Smoke Tests

Host smoke:

```bash
make smoke
```

This validates:

- `version`
- `status`
- `install --dry-run`
- `site:create --dry-run`
- `doctor --json`

Docker functional smoke:

```bash
make docker-smoke
```

Current default services:

- `el9-apache`
- `el9-lsws`
- `el9-ols`
- `el10-apache`
- `el10-lsws`
- `el10-ols`

The Docker runner currently does:

- `docker compose config -q`
- `docker compose config --services`
- `docker compose up --build`
- service log capture
- `docker compose down`

Artifacts are written to:

- `dist/docker-smoke/<service>.log`

Each service log is expected to contain a structured `"status": "passed"` marker from the container smoke fixture.
For Apache images, the fixture now also validates a real `httpd` service path with apply + reload + HTTP response checks.
For OLS images, the fixture validates generated managed config assets, parity report outputs on disk, and performs runtime verification (OLS service startup, configtest, HTTP request).
For LSWS images, the fixture validates generated managed config assets and parity report outputs on disk.

If the current user cannot access Docker, the script exits with an explicit preflight error.

## CI Skeleton

The repository now includes `.github/workflows/ci.yml`.

Current CI behavior:

- runs `go test ./...`
- runs `go build ./...`
- builds cross-platform release artifacts
- packages and verifies release metadata
- validates `docker/compose/functional.yaml`
- lists Docker smoke services
- optionally signs and re-verifies artifacts when signing secrets are configured
- uploads release artifacts from `dist/releases/` and `dist/packages/`

There is also an opt-in `docker-functional-smoke` workflow job:

- only available through `workflow_dispatch`
- requires `run_docker_smoke=true`
- runs `make docker-smoke`
- uploads `dist/docker-smoke/` artifacts

## GitHub Release Automation

The repository now also includes `.github/workflows/release.yml`.

Current release workflow behavior:

- triggers on `push` tags matching `v*`
- supports manual `workflow_dispatch`
- runs a release version/tag guard before publishing
- runs the provider-neutral release pipeline script
- renders release notes from the repository template
- optionally signs artifacts when signing secrets are configured
- fetches the published GitHub Release asset list
- performs remote artifact verification against the published release URL
- writes a post-release verification summary back into workflow artifacts and `GITHUB_STEP_SUMMARY`
- uploads packaged artifacts to the workflow run
- creates a GitHub Release and attaches `dist/packages/<version>/` outputs

Expected signing secrets:

- `RELEASE_SIGNING_KEY`
- `RELEASE_SIGNING_PUBKEY`

If signing secrets are not configured, the workflow still produces checksums, SBOM, provenance, and packaged archives, but detached signatures remain absent.

## Publish

Publish release artifacts to a target:

### GitHub

```bash
LLSTACK_PUBLISH_PROVIDER=github \
LLSTACK_PUBLISH_TARGET=owner/repo \
GH_TOKEN=<token> \
make publish-release VERSION=v0.1.0
```

This uses `gh` CLI to create a GitHub Release and upload all packaged artifacts.

### Directory

```bash
LLSTACK_PUBLISH_PROVIDER=directory \
LLSTACK_PUBLISH_TARGET=/var/www/releases/v0.1.0 \
make publish-release VERSION=v0.1.0
```

This copies release artifacts to the target directory. Useful for local web servers, S3-mounted directories, or any filesystem target.

### Pipeline Integration

Publish can be integrated into the release pipeline:

```bash
LLSTACK_RUN_PUBLISH=1 \
LLSTACK_PUBLISH_PROVIDER=directory \
LLSTACK_PUBLISH_TARGET=/var/www/releases/v0.1.0 \
make release-pipeline VERSION=v0.1.0 MODE=release
```

Output files:

- `release-assets.txt` — published asset listing (compatible with `post-release-report.sh`)
- `release-url.txt` — release URL

## Operator Notes

- `provenance.json` now includes build context: git commit, repository, ref, Go version, and build platform. This enables source-to-artifact traceability without introducing external dependencies.
- Release publishing is now provider-neutral via `scripts/release/publish.sh` with `github` and `directory` providers. The GitHub Actions workflow uses the same script.
- Detached signing now exists via OpenSSL, but third-party transparency log integration is still not implemented.
- GitHub Actions CI now exists as a repository-local baseline, while the primary release orchestration logic lives in provider-neutral scripts under `scripts/release/`.
- Tag-driven GitHub release automation now uses provider-neutral `publish.sh` instead of `softprops/action-gh-release`. Key lifecycle management is still future work.
- Docker functional coverage now spans EL9/EL10 × Apache/OLS/LSWS smoke scenarios, but it is still smoke-first rather than a deeper service matrix.
- Config-driven install now supports a nested schema plus legacy flat compatibility; a richer scenario model is still future work.
