# Contributing

Thanks for your interest in contributing.

## Quick Start

1. Fork and clone the repo.
2. On macOS or Linux, install Go 1.22+, Python 3, Make, and Bash. Native macOS
   builds also need Xcode Command Line Tools. PowerShell is not required for the
   ordinary local gate.
3. During development, run targeted tests, for example:
   ```bash
   go test ./internal/cli -run '^TestDocumentation'
   ```
4. Format changed Go code with `make fmt`, update affected docs, and regenerate
   help with `scripts/update-help.sh` when commands or flags change.
5. Validate the finished revision:
   ```bash
   make check
   ```

## Ready for handoff

`make check` verifies formatting, runs vet and the full Go suite with coverage,
checks documentation and help snapshots, exercises terminal token input with fake
credentials and a local HTTP server, and checks module metadata. It does not
rewrite repository files or require Todoist credentials. A cold dependency cache
may require network access. Coverage must be at least 30% for `internal/cli` and
80% for `internal/output`.

Use a passing local `make check` **or** passing ordinary CI on the finished
revision. Passing CI does not require an identical local rerun. Report the tested
commit (or identify uncommitted changes tested), results, and any pending or
unavailable checks. Earlier evidence does not cover subsequent edits.

If CI is unavailable or pending, a local pass is enough to hand the change off
for review; state which CI environments or specialized checks remain unverified.
Do not describe a local pass as cross-platform validation. Merging still requires
the repository's required CI checks.

Ordinary CI runs the same `make check` on Linux with Go 1.22 and macOS with Go
1.26. Linux also runs the PowerShell completion smoke test. No mandatory live
Todoist testing or per-task consumer review is part of this gate.

### Focused and specialized checks

- Existing `make test`, `make vet`, `make fmt-check`, `make coverage-check`,
  `make docs-check`, and `make check-help` remain available for focused work.
  `coverage-check` runs the full Go suite and enforces the two coverage floors.
- `make auth-terminal-check` runs the credential-free terminal smoke test;
  `make mod-check` checks module consistency without rewriting `go.mod` or
  `go.sum`. If metadata has drifted, run `go mod tidy` and review the changes.
- For PowerShell completion changes, run
  `pwsh -NoProfile -File scripts/test-powershell-completion.ps1` when PowerShell
  is available locally. This check is mandatory in Linux CI.
- For native credential-adapter changes, use the separate opt-in
  [disposable-Keychain test](SECURITY.md#isolated-native-tests). Ordinary tests
  use fake adapters; they do not verify real Keychain integration.
- For portability changes, cross-compile the affected portable builds. Retain
  the [release runbook](RELEASING.md#native-credential-adapter)'s portable-build
  and native-integration checks before release.
- Version, changelog, clean-checkout, stamped-binary, archive, checksum, and
  Homebrew formula validation belong to the [release process](RELEASING.md).

## Guidelines

- Keep changes focused and add tests when possible.
- Follow the CLI UX conventions in `README.md`.
- When changing commands, flags, output, or configuration, update the affected README/spec sections and regenerate help with `scripts/update-help.sh`. See [documentation maintenance](docs/README.md#keeping-docs-in-sync).
- Open a discussion before large or breaking changes.

## Submitting a PR

- Describe the problem and the solution.
- Include the tested revision and validation results as described above.
- Ensure required CI passes before merging.

## Releases

See `RELEASING.md` for the release script and checklist.
