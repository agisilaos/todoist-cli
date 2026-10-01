# Releasing

## Go toolchain

Release checks, dry runs and publication select Go 1.27.1 through
`RELEASE_GO_TOOLCHAIN` in `scripts/release-config.sh`. Release/current CI uses
that same version. Go downloads and verifies it if needed. Ordinary verification
retains the caller's toolchain; the existing module minimum remains supported.

Releases are prepared by an agent, reviewed by a human, and published from a clean macOS checkout of the default branch.

## Prepare the changelog

Ask an agent to prepare `vX.Y.Z`. The agent must start from the repository evidence:

```bash
make changelog-context VERSION=vX.Y.Z
```

The agent updates only the new top section of `CHANGELOG.md` and must:

- describe user-visible outcomes rather than copy commit subjects;
- group related implementation commits into one useful bullet;
- use clear headings such as `Added`, `Changed`, `Fixed`, or `Removed` when they help;
- link every bullet to its verified merged GitHub pull request, or to a GitHub commit when no pull request exists;
- call out breaking changes explicitly;
- preserve all existing release sections.

Use commit messages, changed-file evidence, and PR metadata to understand impact. Never infer a PR association without evidence. Review the generated section, then commit it before running release checks.

## Validate and publish

Run these commands in order:

```bash
make release-check VERSION=vX.Y.Z
make release-dry-run VERSION=vX.Y.Z
make release VERSION=vX.Y.Z
```

`release-check` validates the clean worktree, version, changelog, script-tool portability, and version-stamped binary, and runs the shared `make check` gate described in [Contributing](CONTRIBUTING.md#ready-for-handoff). `release-dry-run` builds both macOS archives and checksums, extracts the approved changelog section as release notes, and renders the Homebrew formula without remote writes. Formula validation requires Ruby and checks its syntax; both archives include the MIT license.

The final command creates and pushes the tag, publishes the GitHub Release with the approved changelog section, and updates the configured Homebrew tap.

The `release-check` GitHub workflow is manual-only and runs
`make release-check-ci` on macOS with Go 1.26. Ordinary pull-request and push CI
runs `make check` on Linux/Go 1.22 and macOS/Go 1.26 without release-specific
preparation requirements. Both `release-dry-run` and `release` retain their own
release preflight; ordinary CI evidence does not bypass these release checks.

## Changelog policy

- Keep concrete release headings in the form `## [vX.Y.Z] - YYYY-MM-DD`.
- Do not add an `Unreleased` section.
- Treat the reviewed changelog section as the source of truth for GitHub release notes.

## Native credential adapter

macOS release archives build both amd64 and arm64 with `CGO_ENABLED=1` so Keychain
support is included. Build on macOS with Xcode Command Line Tools/Apple clang and
Security.framework available. `CGO_ENABLED=0` still compiles a portable CLI whose
native adapter reports unavailable; saved login then requires explicit file storage.
Before release, run the opt-in disposable-Keychain test documented in SECURITY.md
and cross-compile the portable builds for macOS, Linux, and Windows.

## Local verification and recovery

`make verify` is an alias for the existing `make check` gate. It checks the pinned
shared helper bundle and module metadata before formatting, vet, coverage, docs
and auth-terminal checks. Linux/macOS ordinary CI and manual release-check CI
remain supported. Release archives retain native Keychain support and LICENSE.

Publication requires `main`; the selected existing Homebrew branch and formula
are prepared before creating a tag. Dry run validates both architecture archives,
checksums, changelog notes and Ruby syntax. See [release recovery](docs/release-recovery.md)
for phase outcomes and retained originals after interruption.
