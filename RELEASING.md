# Releasing

This repository uses the unified release workflow.

## Release Flow

```bash
make release-check VERSION=vX.Y.Z
make release-dry-run VERSION=vX.Y.Z
make release VERSION=vX.Y.Z
```

## What Each Step Does

- `make release-check VERSION=vX.Y.Z`
  - validates version/tag preconditions
  - runs tests, vet, docs checks, and formatting checks
  - verifies version-stamped binary output
- `make release-dry-run VERSION=vX.Y.Z`
  - builds darwin release archives and checksums
  - performs no remote mutations
- `make release VERSION=vX.Y.Z`
  - runs release-check
  - updates changelog from git history
  - tags and pushes release
  - publishes GitHub release artifacts
  - updates Homebrew formula in `agisilaos/homebrew-tap`

## Changelog Policy

- Do not use `## [Unreleased]`.
- Add concrete released sections only (for example `## [v0.7.0] - 2026-02-19`).

## Notes

- Run releases from macOS with a clean git worktree.
- Required tools are validated by release scripts.
