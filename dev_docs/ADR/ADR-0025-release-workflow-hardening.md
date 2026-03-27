# ADR-0025: Release Workflow Hardening

## Status

Accepted

## Context

LLStack already had:

- build/package scripts
- detached signing
- metadata verification
- CI skeleton
- tag-driven GitHub release workflow

The next gap was not another new release path, but hardening the existing one:

- releases needed a repository-owned notes template instead of ad hoc body text
- tag/version mismatches needed to fail before publishing
- post-release verification needed a written summary rather than relying on operator inspection

## Decision

Harden the existing release workflow by adding three script-backed pieces:

1. `scripts/release/validate-version.sh`
   - validates semver-like release versions
   - enforces optional `v` prefix
   - enforces optional tag/ref match
   - can require an existing git tag before hosted publish

2. `scripts/release/render-notes.sh`
   - renders release notes from `.github/release-notes.md`
   - derives archive/checksum/signing state from packaged artifacts
   - keeps hosted release notes aligned with actual release contents

3. `scripts/release/post-release-report.sh`
   - writes markdown/json verification summaries
   - compares expected local package assets with published remote release asset names when available
   - appends the summary to `GITHUB_STEP_SUMMARY` in hosted execution

## Consequences

Positive:

- release publication fails earlier on tag/version mistakes
- release notes stay tied to actual packaged outputs
- post-release verification becomes auditable instead of manual
- the workflow remains script-driven and testable from the repository

Tradeoffs:

- post-release verification is currently GitHub Release asset-name based rather than a full remote checksum fetch/verify loop
- trusted provenance and cross-channel verification remain future work
