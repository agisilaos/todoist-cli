# Documentation

## Core

These are maintained contracts and reference documentation. Design documents may
also retain historical acceptance dates, base revisions, and delivery notes;
those record provenance, not a current review base or standing authorization.
Use the [contributor handoff gate](../CONTRIBUTING.md#ready-for-handoff) for current validation.

- [CLI specification](SPEC.md): supported behavior and machine output contracts
- [Architecture overview](ARCHITECTURE.md): package responsibilities and boundaries
- [Complete task editing](task-editing-design.md): presence, hierarchy, recurrence/time, collections, and recovery
- [Task data fidelity](task-data-fidelity.md): frozen task resource projections and returned evidence
- [Domain vocabulary](../CONTEXT.md): shared terminology
- [Architecture decisions](adr/): accepted tradeoffs
- [Product roadmap](ROADMAP.md): implemented capabilities and future-work tracker
- [CLI help snapshots](help/): generated from the current binary
- [Agent skill lifecycle](agent-skill.md): supported targets, ownership, updates, machine output, and recovery
- [Errors and recovery](error-recovery.md): failure classifications, safe next steps, and compatibility boundaries
- [Task-data fidelity](task-data-fidelity.md): versioned task-resource output, field presence, and compatibility
- [Authorization contract](authorization-design.md): scope evidence, credential lifecycle, and mutation enforcement
- [Authorization command inventory](authorization-command-inventory.md): remote, local, planning, and dry-run effects
- [Authorization verification](authorization-implementation.md): regression boundaries and verification limits
- [Credential storage](credential-store-design.md): backend selection, token/metadata pairing, migration, and recovery
- [Security policy](../SECURITY.md): vulnerability reporting, secret handling, and isolated native tests
- [Daily review](review-design.md): interaction, review-plan compatibility, and recovery
- [Credential profiles and OAuth onboarding](profile-oauth-design.md): selection, removal recovery, and external OAuth prerequisites
- [Contributor guidance](../CONTRIBUTING.md): development workflow, validation, and handoff requirements

## Historical review evidence

These records describe particular revisions and review environments. They are
supporting evidence, not maintained behavior contracts or validation of later changes.

- [Profile consumer review](profile-consumer-review.md): recorded public-path execution, repairs, and verification limits
- [Review evidence](evidence/): retained task, recovery, and agent-skill execution records

## Release

- [Release runbook](../RELEASING.md)
- [Release workflow commands](../README.md#release)
- [Release history](../CHANGELOG.md)

## Keeping docs in sync

Run `make docs-check` for focused documentation work on macOS or Linux with Go
and Python 3 installed. It is included in `make check`, the shared local and CI
[handoff gate](../CONTRIBUTING.md#ready-for-handoff), and in release validation.

When changing a command or flag:

1. Update the implementation and its help text together.
2. Update the affected examples and behavior descriptions in [README.md](../README.md)
   and [SPEC.md](SPEC.md). Update architecture/domain docs when responsibilities or
   terminology change.
3. Run `scripts/update-help.sh` and review the generated diff in `docs/help/`.
   Run `python3 scripts/agent-skill-reference.py --write` to update bundled command
   references from live help; review curated workflows separately when behavior changes.
4. Run `make docs-check` while iterating, then follow the
   [handoff workflow](../CONTRIBUTING.md#ready-for-handoff) for the finished revision.

The gate checks:

- Required README headings and release references, changelog presence, and the
  no-`Unreleased` policy across root Markdown files and `docs/`.
- Existing targets for inline relative Markdown links (external URLs and anchors
  are not validated).
- Exact help snapshots, including every dispatched top-level command and child leaf. Add a new
  command to `scripts/help-snapshots.txt`; snapshots are never refreshed by CI.
- Exact bundled agent command references generated from live help, plus strict
  command/long-flag checks for skill examples. Unknown commands fail this check;
  examples are inspected without executing their mutations. CI checks never rewrite the bundle.
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

## Command discovery metadata

`internal/cli/command_metadata.go` owns command paths, parent/child relationships,
help aliases, and the short descriptions and sections used by root help. Catalog
order controls completion inventories; `rootHelpOrder` preserves the separate
editorial ordering of root help. Execution switches and their alias maps remain
authoritative in the existing command handlers.

When adding or changing a command:

1. Implement its execution and parsing in the existing handler.
2. Add its canonical path and aliases to `commandCatalog`. A group has `group: true`;
   a root command also needs its summary, section, and help order. Root listings,
   help lookup, and existing shell inventory slots derive from this entry.
3. Maintain detailed usage, flags, examples, and notes in `leaf_help.go` or the
   command's existing help function. Group usage prose remains hand-written.
   Add the focused page to `scripts/help-snapshots.txt`, update affected README/spec
   examples, and regenerate snapshots with `scripts/update-help.sh`.
4. Keep shell-specific flags, value/file completion, and traversal in
   `completion_scripts.go` and `completion_powershell.go`. A new group may need a
   shell context with a `{{commands:parent}}` slot; adding a child to an existing
   context does not require copying its name into each shell. PowerShell derives
   group and alias tables directly. This does not promise equal completion depth
   or flag support across shells.
5. For a new routing function, extend the explicit router inventory in
   `dispatchedCommandInventory` in `command_metadata_test.go`. This independent
   source check compares executable canonical commands and aliases against discovery,
   in both directions. It checks known Go source shapes, not arbitrary control flow;
   adjust it deliberately if routing structure changes. Do not generate its expected
   commands from the discovery catalog. The shared-omission regression proves that
   dropping `review` from discovery still fails the execution contract.
6. Run targeted metadata/help/completion checks during development, then use the
   [ordinary handoff gate](../CONTRIBUTING.md#ready-for-handoff). Preserve the
   separate PowerShell smoke test in CI.

Flags are intentionally outside this catalog. Detailed help and shell completion
use them differently; the existing parser-registration checks still validate
leaf-help flag names. Help-only `examples`, shell normalization (`pwsh`, case,
whitespace), and completion-install shell arguments keep their existing owners.

For PowerShell command-flag completion, add a flag once: switches belong in
`$todoistSwitchFlags`, and flags that consume a value belong in
`$todoistValueFlags`. Suggestions combine both tables. Keep the lists disjoint;
value candidates still belong in `$todoistValues`. These are PowerShell completion
hints, not parser registrations or a shared cross-shell flag schema. Global flags
retain their existing separate inventory. Changes must preserve value skipping,
space-separated and `=` values, aliases, and the `--` boundary; exercise them with
the PowerShell smoke test. Public help and other shells retain their own flag
presentation and behavior.
