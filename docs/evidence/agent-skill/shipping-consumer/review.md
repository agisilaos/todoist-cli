# Bounded CLI consumer review

No actionable findings in the selected workflow. Completed 36 public-help/lifecycle/schema invocations using noninteractive subprocess pipes on macOS. No implementation, internal test, or prior-review artifact was consulted.

Task: discover installable agent integration from the README introduction, install into a supported project path containing spaces, inspect inventory and installed guidance, update, recover from edited/missing/unmanaged files, and uninstall while preserving unrelated files. A secondary check exercised Claude Code global placement in an isolated chosen home path with spaces.

Version: repository HEAD 1ea11d5f35da4faf3ce221b034146264d9724b5e plus uncommitted installable-agent-skill changes. Built `/tmp/todoist-skill-consumer` via `go build -o /tmp/todoist-skill-consumer ./cmd/todoist`; reports `todoist dev (local) unreleased`. Reviewed bundled digest: dad5b29dff7cac6d0317d68dcaea5208914ad926f17e9ca94fcb9d3405ce8d7a. Binary SHA-256: 8aaba8fe0291d9c32e73b44bd7fabb35d51597dc7f203723c237e5517b3eafd2.

Public route: README opening paragraph -> Agent skills -> `skill list --json` and focused install help -> explicit target/scope/full path command. Focused list/update/uninstall help and linked public lifecycle contract supplied recovery. The initial attempt was recorded in consumer-transcript.txt before recovery work; no initial discovery or execution friction required guessing. `/tmp` and the first temporary folder canonicalized through macOS aliases as publicly documented.

Observed checks:

- Codex local install wrote SKILL.md, four references, and a manifest to the expected `.agents/skills/todoist-cli` tree. List reported the exact installed digest. Repeated install and same-bundle update returned unchanged.
- JSON success went to stdout with empty stderr. NDJSON list/result emitted one parseable record per selected item/result. JSON and NDJSON failures went to stderr with empty stdout. Modified/unmanaged failures returned 5; invalid/missing placement returned 2. Inventory with a conflict returned 0 and explicit conflict status, as documented.
- Edited SKILL.md blocked update/uninstall with SKILL_MODIFIED and byte-for-byte unchanged destination state. `update --backup` wrote exact originals plus the ownership manifest outside `.agents/skills`, restored bundled bytes, and retained unrelated notes. A missing managed reference also blocked default update and was repaired by `--backup`.
- Default uninstall removed owned package files/manifest, retained the exact unrelated notes, and preserved the backup. Repeated uninstall returned unchanged.
- `uninstall --keep-modified` retained the exact edited SKILL.md and unrelated notes, removed unchanged references and ownership, and later list reported the remaining entrypoint as an unmanaged conflict. The public warning correctly explains that such a retained entrypoint can remain discoverable with removed references.
- Unmanaged existing SKILL.md was refused without changes. Moving that original aside using the public recovery guidance allowed install/uninstall; the preserved original remained exact.
- Missing scope and relative path failed before mutation. Claude Code explicit global `.claude/skills/todoist-cli` install/list/uninstall worked under isolated home, including quoted paths with spaces and readable plain output.
- Installed SKILL.md and curated references were readable and all local Markdown links resolved. Main-body guidance provides live help/schema checks and conditional references for machine calls, credentials, authorization, plans, replay and uncertain writes. Generated command reference has searchable command headings. Installed snapshot is retained separately from loading paths.

Limits: this review proves local lifecycle behavior and public discoverability only. It did not call Todoist, use real credentials, run an external planner, initiate model calls, inspect interactive terminal behavior, or re-run native agent loading (existing supplied loading evidence was deliberately not repeated). It exercised same-bundle update/replacement, not an older/newer binary upgrade. Claude global placement was explicitly selected scratch layout, not automatic real-home discovery. The initial unqualified read-only inventory inspected conventional real-home presence but changed no user-global state. No source or repository files were edited.

Evidence: consumer-transcript.txt contains exact commands, cwd, stdout, stderr, exits and state checks. initial-owned-state.json contains initial hashes/manifest. installed-package-snapshot/ preserves installed content. backup-state.json identifies the exact modified-original backup. Remaining scratch customized/unrelated files and backup are retained for inspection.

Environment:

```text
ProductName:		macOS
ProductVersion:		27.2
BuildVersion:		26B5091g
go version go1.27.1 darwin/arm64
```
