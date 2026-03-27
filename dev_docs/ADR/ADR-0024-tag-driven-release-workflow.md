# ADR-0024: Tag-Driven Release Workflow

## Status

Accepted

## Context

LLStack already has repository-local scripts for:

- build
- package
- detached signing
- metadata verification
- install / upgrade
- smoke validation

The repository now also has a CI baseline, but it still lacked a first-party release automation path that turns a version tag into a reproducible packaged release.

Without a tag-driven release workflow:

- releases remain a manual operator process
- build/package/sign/verify can drift between local and hosted execution
- GitHub Releases are not guaranteed to match the repository-local release scripts

## Decision

Add `.github/workflows/release.yml` with:

- trigger on `push` tags matching `v*`
- optional `workflow_dispatch` input for manual release builds
- test/build/package flow reusing existing repository scripts
- optional detached signing when signing secrets are configured
- metadata verification after package/sign
- artifact upload to the workflow run
- GitHub Release creation with attached packaged artifacts

## Consequences

Positive:

- release automation now reuses the same scripts operators use locally
- tagged releases become reproducible and auditable
- packaged artifacts, metadata, and optional detached signatures stay aligned

Tradeoffs:

- this is still GitHub-centric release automation rather than provider-neutral CI/CD
- trusted provenance, transparency logs, and key lifecycle management remain future work
- manual workflow dispatch still depends on the operator choosing the correct version/tag semantics
