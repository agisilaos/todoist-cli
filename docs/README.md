# Documentation

## Core

- [CLI specification](SPEC.md): supported behavior and machine output contracts
- [Architecture overview](ARCHITECTURE.md): package responsibilities and boundaries
- [Domain vocabulary](../CONTEXT.md): shared terminology
- [Architecture decisions](adr/): accepted tradeoffs
- [Product roadmap](ROADMAP.md): implemented capabilities and future-work tracker
- [CLI help snapshots](help/): generated from the current binary
- [Daily review](review-design.md): interaction, review-plan compatibility, and recovery

## Release

- [Release runbook](../RELEASING.md)
- [Release workflow commands](../README.md#release)
- [Release history](../CHANGELOG.md)

## Keeping docs in sync

Run `make docs-check` on macOS or Linux with Go and Python 3 installed. Both the
normal CI workflow and the macOS release check run this gate.

When changing a command or flag:

1. Update the implementation and its help text together.
2. Update the affected examples and behavior descriptions in [README.md](../README.md)
   and [SPEC.md](SPEC.md). Update architecture/domain docs when responsibilities or
   terminology change.
3. Run `scripts/update-help.sh` and review the generated diff in `docs/help/`.
4. Run `make docs-check` and `go test ./...`.

The gate checks:

- Required README headings and release references, changelog presence, and the
  no-`Unreleased` policy across root Markdown files and `docs/`.
- Existing targets for inline relative Markdown links (external URLs and anchors
  are not validated).
- Exact help snapshots, including every dispatched top-level command and child leaf. Add a new
  command to `scripts/help-snapshots.txt`; snapshots are never refreshed by CI.
- Global flag inventory in README and root help against the global parser.
- Shell quoting in README command lines.
- Long flag names in command snippets in README, SPEC, and help snapshots against
  Go flag registrations and global parsing. The test reads Go syntax; it never
  executes examples, authenticates, or changes Todoist data.

Flag checking covers recognized command prefixes, including common aliases. It
is not a shell interpreter: it does not validate positional arguments, flag value
semantics, arbitrary shell syntax, or command groups without a flag parser.
Behavior descriptions and API compatibility still require review and behavioral
tests; a passing docs check does not prove every prose claim.
