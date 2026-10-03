# Contributing

Thanks for your interest in contributing.

## Quick Start

1. Fork and clone the repo.
2. On macOS or Linux, install Go 1.27.1+, Python 3, Make, and Bash. Native macOS
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
credentials and a local HTTP server, verifies the capture/review/replay workflow
against an isolated API fixture with a real PTY, and checks module metadata. It does not
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

Ordinary CI runs the same `make check` on Linux and macOS with Go
1.27.1. Linux also runs the PowerShell completion smoke test. macOS also runs the
[disposable-Keychain test](SECURITY.md#isolated-native-tests), with its default
Keychain and search list checked for isolation. No mandatory live
Todoist testing or per-task consumer review is part of this gate.

### Focused and specialized checks

- Existing `make test`, `make vet`, `make fmt-check`, `make coverage-check`,
  `make docs-check`, and `make check-help` remain available for focused work.
  `coverage-check` runs the full Go suite and enforces the two coverage floors.
- `make auth-terminal-check` runs the credential-free terminal smoke test;
  `make mod-check` checks module consistency without rewriting `go.mod` or
  `go.sum`. If metadata has drifted, run `go mod tidy` and review the changes.
- For review/replay persistence changes, run
  `go test ./internal/cli -run '^(TestReview|TestReplayStore|TestApplyActions)'`.
  Use the existing local API fixture and injected persistence failures to assert
  persisted evidence, reports, and mutation counts at failure boundaries. See the
  [persistence sequence](docs/review-design.md#persistence-sequence-for-maintainers).
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

### Capture, review, and replay

```bash
make capture-review-check
make capture-review-check BINARY=/absolute/path/to/todoist
python3 scripts/test-capture-review.py --binary /absolute/path/to/todoist --verbose
```

The default target builds into a temporary directory; `BINARY` bypasses the build.
It requires macOS or Linux, Python 3's standard library, and Go for the default
build. Run it from a clean checkout without configuring authentication. CLI
invocations use a temporary working directory, HOME, explicit configuration, an
allowlisted environment, and a synthetic token. They never read the user's
credential profiles or Keychain. Temporary plans, replay records, server, and
child processes are cleaned up on failure as well as success.

The fixture checks receipt fields returned by the API, no mutation during dry run
or cancellation, one confirmed completion with a saved review plan, replay with
no duplicate mutation or changes to saved evidence, and refusal of noninteractive
review. Interactive commands use a real PTY; answers wait for their prompts.
Machine replay parses stdout as JSON and checks stderr separately. Each CLI
command has a 15-second deadline; the optional build has a 180-second deadline.
Failures print command output, exit status, fixture state, and request counts;
`--verbose` also prints transcripts on success. All data is synthetic.

The Make target also runs fault checks (rejected capture, wrong returned content,
and an acknowledged completion with unchanged state) and runner tests for missing
prompts, timeout diagnostics, child reaping, and server cleanup. Use explicit
checks in the harness so Python optimization cannot disable assertions.

This is a narrow consumer contract, not a Todoist API emulator or live-service
verification. The separate Go tests cover broader receipt variants, review
selection, persistence failures, and replay recovery. Native credential isolation
is covered separately; this workflow always uses an environment token. Keep
future workflow refactors using this harness rather than copying it. No new CLI
flags, help snapshots, completion entries, domain terms, or ADRs are needed for
changes confined to this gate.

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
