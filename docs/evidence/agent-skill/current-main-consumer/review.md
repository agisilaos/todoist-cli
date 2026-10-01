# Bounded CLI consumer review after pending main integration

Outcome: the selected public install/list/update/uninstall workflow and its edited-file recovery succeeded. No actionable findings or initial public-path friction were observed for humans or machine clients in this workflow. No implementation, tests, or earlier review evidence was inspected.

Executable: `/tmp/todoist-skill-current-main`; version `todoist dev (local) unreleased`. Documentation: public worktree at `/Users/agis/.codex/worktrees/installable-agent-skill/todoist-cli`, HEAD `8322db33e02e86fd395f166811b2f59902d6573e` plus pending merge of `b12585b` with profile/OAuth support, as supplied by the parent. This is a pending-merge tree review, not a published release or committed merge review. Binary SHA-256 and copied documentation hashes are in `metadata.json`; the binary remained unchanged throughout.

I followed README → root/group help → empty JSON inventory → install help → actual Codex local installation under `My Project/.agents/skills/todoist-cli`. The paths contained spaces. HOME was replaced with an empty `Scratch Home`, the environment was reduced to an explicit nonsecret allowlist, stdin was `/dev/null`, and output was captured in separate files. No credentials, remote calls, native agent-loading probes, repository changes, commits, publication, or merge actions were used.

Observed actual sequence:

- Install: exit 0, `installed`, five managed Markdown files and an ownership manifest. The manifest hashes matched actual installed bytes; relative documentation links resolved.
- List: exit 0, one valid NDJSON inventory item, `installed`. Identical update: exit 0, `unchanged`, bytes and mtimes intact.
- Clean uninstall: exit 0, `uninstalled`; five managed files and manifest removed, unrelated skill note preserved and reported. Project `AGENTS.md` and scratch user's `.zshrc` remained intact. Repeated uninstall: exit 0, `unchanged`.
- Reinstall with unrelated content: exit 0, `installed`. Append a synthetic edit to `SKILL.md`; list: exit 0, `modified` with `SKILL.md` identified. Default update (JSON) and uninstall (NDJSON): exit 5, empty stdout, stderr JSON `SKILL_MODIFIED`, `committed: false`, `recovery_required: false`, and specific `--backup` / `--keep-modified` recovery. Full filesystem snapshots stayed identical after each rejected action.
- Update `--backup`: exit 0, `updated`; exact edited original and old manifest saved outside `.agents/skills`, bundled `SKILL.md` restored, unrelated files retained. Edit again, uninstall `--keep-modified`: exit 0, `uninstalled`; exact edited skill and unrelated note retained and reported, four unchanged references and manifest removed, earlier backup preserved. Repeated uninstall stayed `unchanged`.
- Cheap Claude Code global placement under isolated space-containing HOME: install/list/update/uninstall all exited 0 with expected installed/unchanged/uninstalled states; its managed directory was removed.

Successful JSON/NDJSON payloads parsed and stderr was empty. NDJSON lifecycle results and the classified NDJSON failure each contained exactly one JSON line. List's zero exit with a `modified` inventory item agreed with the documented requirement to inspect item status.

Installed curated references include profile selection/source distinctions, profile list/current, profile mutation limits, OAuth prerequisites, unknown scope evidence, read-only enforcement claims, and refresh/device/live-exchange limits. Relevant usage/global flag declarations for `profile list`, `profile current`, and `auth login` matched focused live help. This check verifies discoverability and reference alignment; it does not verify remote authorization enforcement.

Prior-knowledge limits: the parent supplied the feature scope, binary/revision, profile integration expectation, and prior native loading completion. I had no implementation knowledge. Recovery was discovered from public README/help and actual error output; no source-assisted recovery was needed. Two reviewer harness assumptions were corrected (manifest shape and reference intentionally omitting live Notes/Examples), recorded in `harness-corrections.txt`; these were reviewer mistakes, not CLI friction.

Limits: no TTY behavior, credentials, live Todoist behavior, cross-version update, filesystem fault injection, custom homes/cloud loading, or native skill loading were tested. The final scratch target deliberately retains edited instructions with removed reference links, as documented by `--keep-modified`; it is isolated evidence, not an active user project.

Evidence: `commands.jsonl` contains exact argv/shell commands, cwd, stdin mode, stdout/stderr paths, exit status, and duration. `commands/` contains all streams; `states/` contains before/after path/hash snapshots; `installed-package/` preserves the first installed package; `verification-checks.json` records retained assertions; `public-docs/` preserves the public route; `fixture-actions.jsonl` records synthetic edits. All evidence is redacted/nonsecret and outside the repository, following MEMORY.md.
