# ADR-0023: GitHub Actions CI Skeleton

## Status

Accepted

## Context

LLStack already has a repository-local release toolchain for build, package, detached signing, metadata verification, host smoke, and Docker functional smoke. The repository did not yet have a first-party CI workflow that exercised these paths in a predictable way.

Without a CI baseline:

- release scripts stay manually documented but not automatically exercised
- build/package regressions can slip between local operator runs
- signature verification remains disconnected from the normal validation path
- Docker functional smoke has no standard opt-in automation hook

## Decision

Add a GitHub Actions workflow at `.github/workflows/ci.yml` with two jobs:

1. `validate`
   - run `go test ./...`
   - run `go build ./...`
   - run cross-platform release build/package/verify
   - validate Docker compose syntax and smoke service discovery
   - optionally sign and re-verify artifacts when signing secrets exist
   - upload release artifacts

2. `docker-functional-smoke`
   - only available through `workflow_dispatch`
   - only runs when `run_docker_smoke=true`
   - runs `make docker-smoke`
   - uploads `dist/docker-smoke/` artifacts

## Consequences

Positive:

- the repository now has an auditable CI baseline for build/package/verify
- detached signing can be exercised in automation without changing operator scripts
- Docker smoke remains explicit and controllable instead of always-on
- release artifacts become inspectable from CI runs

Tradeoffs:

- this is still a CI skeleton, not a full release pipeline
- secret-backed signing depends on repository configuration
- Docker functional smoke is slower and intentionally opt-in
- CI still does not publish releases, manage tags, or maintain a trusted provenance chain
