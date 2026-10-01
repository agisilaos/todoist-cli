# Agent skill consumer review — 2026-09-30

The public workflow completed for Codex and Claude Code, with local and global
placement. Initial friction was recorded, the feature-specific findings were
fixed, and the consumer workflow was rerun. No actionable findings remain within
this bounded installation workflow.

The [October 1 shipment checks](shipping-checks.md) record a fresh independent
consumer attempt, a repaired lock-cleanup diagnostic, and the subsequently passing
local PowerShell check. The September 30 observations below remain historical.

The reviewed feature is on `fix/installable-agent-skill`, based on main
`1ea11d5f35da4faf3ce221b034146264d9724b5e`, with uncommitted implementation and
contemporaneous README/help. The executable reports `todoist dev (local)
unreleased`. Commands ran on macOS arm64 with stdin closed, isolated credential
state, scratch projects and user homes, and paths containing spaces.

## Public route and initial experience

The route began at README's introductory link and Agent skills section, then used
root help, `skill list --json`, focused lifecycle help, explicit installation,
scoped inventory, and the installed SKILL.md and references. The reviewer had
researched target conventions and native loading probes beforehand, but did not
read installer source or internal tests. This knowledge limit prevents treating
the review as a fully blind study. The initial route required no guessing.

The complete install/list/update/uninstall workflow checked the resulting files,
not only exit status. Identical install/update operations preserved file hashes;
ordinary uninstall preserved unrelated files, sibling content, and backups.
JSON and NDJSON were parsed, successful stderr was empty, and classified failure
stdout was empty. Default modified-file operations failed with exit 5
`SKILL_MODIFIED` and preserved the customization. Explicit backup update preserved
exact original bytes outside the loading root. Explicit retained uninstall left
edited files and unrelated notes while removing ownership and unchanged files.
Reinstallation then refused the unowned conflicting entrypoint. Invalid paths,
missing scope, absent update, and actual concurrent writers exercised relevant
recovery.

Initial P3 documentation friction affected humans and agents: retained edited
SKILL.md still linked to unchanged references that uninstall removed. The existing
warning about continued discoverability did not explain that these instructions
could be incomplete. README, uninstall help, and the lifecycle contract now warn
about removed references and advise inspection or moving the retained skill before
continued use. The original observation is preserved below.

Artifacts retain invocations, stdout/stderr, exit status, and filesystem checks:

- [Initial context](context.json), [initial public route](initial-route.json),
  [completed initial route](public-route.json), and [initial friction](initial-friction.md).
- [Original retention observation](retained-reference-observation.json),
  [repeat checks](repeat-state-check.json), and [recovery checks](recovery-state-check.json).
- [Final rerun](final-rerun/public-route.json),
  [final state checks](final-rerun/state-checks.json),
  [focused transcript](final-rerun/focused-transcript.md), and
  [final consumer report](final-rerun/final-consumer-review.md).

## Actual target loading

Codex 0.139.0 and Claude Code 2.1.280 each discovered and explicitly expanded the
installed main SKILL body into a provider request. Both project loading and global
user-home loading from a separate project were checked. Claude's initial skill
catalog included `todoist-cli`. The installed main entrypoint remained byte-identical
after the fixes, so these loading observations apply to the final package.

The native CLIs used fake credentials, isolated state, a read-only Codex sandbox,
an empty Claude tool list, and an HTTP sink on loopback. The sink parsed requests and checked skill body and
target-directory presence, then deliberately returned HTTP 400. Their exit 1 is
the expected probe stop, not an installation failure. Raw provider requests were
not saved. Since both targets share the same SKILL body, both body comparisons
match; only the intended target directory must match.

See [local loading](native-local-loading.json) and
[global loading](native-global-loading.json) for runtime versions, invocation
arguments, body hashes, request checks, and limitations. This proves discovery and
instruction prompt construction. It does not prove model compliance, automatic
invocation quality, or model consumption of reference files.

## Implementation review and checks

The bounded code review found omitted human recovery paths, insufficient error
evidence on result-output failures after mutation, and zsh suggestions of flags
unsupported by the selected lifecycle operation. All three were fixed and verified
with targeted regression tests. The reviewer authored the bundle and therefore
did not independently review that content; a separate consumer checked the public
instructions and loaded entrypoint. Root contract review supplied the content
assessment. No remaining actionable code-review findings were reported within
the reviewed lifecycle scope.

Final `go test ./...` and `make check` passed, including formatting, vet, coverage,
module metadata, help snapshots, documentation, terminal authentication, generated
reference freshness, and 14 reference-generator regressions checking 28 curated
examples. Installer race tests passed with 88.5% coverage. Bash, fish, and zsh
completion syntax was checked; bash and zsh lifecycle behavior was exercised.
Linux amd64 and Windows amd64 cross-compilation passed. PowerShell completion
assertions were added to the existing CI smoke script; local execution was
unavailable because `pwsh` is not installed. Evidence-only docs were subsequently
checked by the documentation gate.

## Limits

No live Todoist requests, real model inference, interactive target selector,
cloud distribution, enterprise override, duplicate-skill precedence, or changed
release/downgrade comparison was exercised by this consumer review. Linux/Windows
native target consumption and local PowerShell execution were not tested.
Partial filesystem and cleanup failures were injected in installer tests; the
consumer route did not simulate crashes or power loss. Cooperative filesystem
maintenance is not an adversarial race or durability guarantee. This review is
bounded to the requested installation lifecycle and relevant recovery, not a
general review of the CLI.
