# Agent skill lifecycle

The bundled `todoist-cli` skill supplies curated workflows and a generated command
reference for this CLI. It is guidance, not a planner executable, credential grant,
or enforcement boundary. Read [the public workflow](../README.md#agent-skills)
and [the ownership decision](adr/0005-own-installed-skill-files-and-update-explicitly.md).

## Targets and placement

The initial supported targets are `codex` and `claude-code`. Codex uses
`.agents/skills/todoist-cli`; Claude Code uses `.claude/skills/todoist-cli`.
Both layouts apply under the selected project (`local`) or user home (`global`).
Every mutation requires exactly one named target, `--scope local|global`, and a
full absolute `--path` ending in that target's layout. The scope records intended
placement; it does not prove a custom directory is a home or that an agent scans it.
No implicit installation, default global write, all-target operation, or network
download exists. Shared agent instruction files and shell profiles are not edited.

Paths are validated before owned-file changes. Parent traversal, mismatched target
layout, symlinks in the managed agent hierarchy, and nonregular managed files fail.
Ancestor aliases above the agent directory are canonicalized; returned paths
identify the destination actually used. This is cooperative local filesystem
maintenance, not a security boundary against an adversarial process racing paths.

Read-only `list` examines conventional paths under the current directory and
user home, optionally restricted by target/scope. It does not walk project
ancestors. An explicit path requires a target and scope. It reports filesystem
state; agent catalog discovery and instruction loading need separate checks.

Official conventions were checked on 2026-09-30: [Codex skills](https://learn.chatgpt.com/docs/build-skills)
and [Claude Code skills](https://code.claude.com/docs/en/skills). Codex also retains
legacy/custom `CODEX_HOME` skill behavior, and Claude can apply enterprise and
personal precedence. The installer selects these current conventional layouts
without promising custom-home, duplicate-name precedence, cloud distribution,
or visibility exclusive to one agent. Claude's `/skills` is an interactive
command; print-mode catalog inspection and explicit loading are separate paths.

## Ownership and updates

`.todoist-skill.json` records format version 1, selected target/scope, CLI version,
package digest, and owned relative paths with SHA-256 content hashes. Package
identity is deterministic over sorted relative names and hashes, independent of
destination and build version. A matching filename or byte sequence does not
establish ownership. Invalid or unsupported manifests fail closed.

Install refuses unmanaged conflicting package files and a different existing
managed package. Repeated install/update of the same digest and recorded CLI
version leaves files unchanged. Update explicitly installs the running binary's
bundle, including an older bundle if the caller deliberately uses an older binary.
Updating the binary itself changes no installed skills.

Missing or edited owned files block replacement/removal by default. Update
`--backup` explicitly permits replacement after saving existing modified originals
and their manifest in a unique backup directory outside the skills loading root.
Missing files have no original to back up. Newly conflicting unowned files still
block updates. Backup directories are retained and reported; uninstall never
deletes them. Customizations in unrelated files are always retained.

Uninstall removes only unchanged owned files and the manifest. Edited/missing
files cause a conflict unless `--keep-modified` explicitly ends management while
leaving existing edits behind. Unrelated files stay in place and are reported in
`retained`. Empty managed directories may be removed; the tree is never recursively
deleted. Repeated uninstall without a manifest is unchanged, retaining unowned
content. A retained skill entrypoint may remain discoverable by an agent, with
links to removed unchanged references. Inspect or move it before continued use.

## Machine output

JSON install/update/uninstall returns a result object with required `operation`,
`status`, `target`, `scope`, `path`, `files`, and `retained` fields. Status is
`installed`, `updated`, `uninstalled`, or `unchanged`. Package digest, recorded
CLI version, and backup path are included when applicable. File paths in arrays
are relative to the selected installation. Empty arrays are `[]`, not `null`.
NDJSON emits that same result as one line. Explicit plain output uses tab-separated
operation/status/target/scope and a quoted destination, with labeled retained/backup
lines; use structured modes for detailed automation.

JSON list returns an array; NDJSON emits one item per line. Each item identifies
target/scope/path, `installed`, `status`, `available_digest`, `modified`, and
`missing`, with installed digest/version when known. Status is `absent`,
`installed`, `outdated`, `modified`, `conflict`, or `error`. Per-location inspection
problems have `code` and `error`. Inventory production exits 0 even for problem
entries, so callers must inspect each status. Invalid invocation/path selection
fails the command. Schemas: `todoist schema --name skill_result` and
`todoist schema --name skill_list`.

Lifecycle errors leave stdout empty unless writing the result itself fails, which
can leave partial output after file changes. Errors use the existing `error`/`meta` envelope
on stderr, adding `code` and `details`. Both JSON and NDJSON lifecycle errors are
JSON; NDJSON and `--json --quiet-json` errors are one line. Details include
`path`, affected `files`, `committed`, and `recovery_required`; backup/recovery
paths appear when applicable. This does not change other commands' NDJSON errors.
Human errors also report the code, affected files, and quoted recovery paths.
When the process can report an output failure, its stderr error preserves
`committed` evidence; inspect
state before retrying rather than assuming failure rolled back file changes.

| Exit | Codes and meaning |
| --- | --- |
| 2 | `SKILL_USAGE`, `SKILL_PATH_INVALID`: fix arguments or explicit destination |
| 4 | `SKILL_NOT_INSTALLED`: use install before update |
| 5 | `SKILL_CONFLICT`, `SKILL_MODIFIED`, `SKILL_MANIFEST_INVALID`, `SKILL_BUSY`: resolve ownership, edits, metadata, or another writer |
| 1 | `SKILL_IO`, `SKILL_PACKAGE_INVALID`, `SKILL_RECOVERY_REQUIRED`: local failure or recovery requiring inspection |

No lifecycle command prompts, contacts Todoist, loads its configuration or
credentials, or creates a requested Todoist progress log. Read-only credentials
do not restrict local installation. `--force` and `--dry-run` are rejected;
inspect with list and use the specific customization options deliberately.

## Failure recovery

Writers use an exclusive per-target lock and stage replacements and restoration
copies outside the loadable skills directory. Owned destinations are rechecked
before replacement; the manifest is published last. Ordinary failures attempt
rollback. These steps do not make a multi-file operation atomic to concurrent
readers, crashes, or power loss, and do not promise storage durability. Start
agent sessions after lifecycle maintenance finishes.

On `SKILL_MODIFIED`, restore files yourself or deliberately choose update backup
or uninstall retention. On `SKILL_CONFLICT`, move conflicting unowned content;
the backup option never adopts it. On invalid manifest, preserve the directory
and restore a valid ownership record; filenames alone cannot establish ownership.

On `SKILL_BUSY`, wait for another operation. After an interruption, verify no
writer is running before removing the reported lock. If recovery is required,
preserve the installation, backup, and reported staging copies, compare their
owned files and manifest, and reconcile the intended state before retrying.
`committed=true` means file changes were established before a cleanup failure;
a nonzero exit alone does not mean nothing changed. No automatic recovery
command or cross-process editor lock is promised.

## Maintenance and comparison

`internal/agentskill/bundle/SKILL.md` and conditional workflow references are
curated. `references/commands.md` is generated from live CLI help using the
complete help-snapshot manifest. Run `python3 scripts/agent-skill-reference.py
--write` after help changes. `make check` checks stale references and nonexistent
command/long-flag examples without running their mutations. CI never refreshes
references. Curated claims still need contract review; a valid example does not
prove remote mutation behavior.

The comparison used Doist CLI 5.4.3 at commit
`b0f9932c912dd74cdea6168c8b22a74ed04e94b7`. Its visible skill lifecycle and
shared package are useful precedents, but it updates detected global skills
automatically and lacks these ownership/customization checks. Our package uses
`todoist` commands, current Inbox/pagination/output contracts, and this project's
authorization and replay limits. It does not copy Doist's broader target or
command availability. [Doist installer](https://github.com/Doist/todoist-cli/blob/b0f9932c912dd74cdea6168c8b22a74ed04e94b7/src/lib/skills/create-installer.ts),
[postinstall updates](https://github.com/Doist/todoist-cli/blob/b0f9932c912dd74cdea6168c8b22a74ed04e94b7/src/postinstall.ts).
