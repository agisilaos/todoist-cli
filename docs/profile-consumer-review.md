# Profile workflow consumer review

Reviewed on 2026-09-30 using README → root/profile/auth help → actual CLI
execution. Binary: `todoist dev (local) unreleased`, built from main `1ea11d5`
plus this feature's worktree changes. Docs were from the same worktree.

The reviewer already knew command-integration details, and the implementing team
knew the internals. This is an assisted bounded review, not a cold-user study.
Initial execution was recorded before further implementation/test inspection.
Pipes were noninteractive; cwd, XDG directory, config, and credentials were scratch.
TODOIST variables were cleared except deliberate overrides. New profiles explicitly
used file storage and synthetic tokens validated by a local HTTP resource fixture.
No existing user credential or real Keychain entry was accessed or deleted.

## Public route and observed results

Every invocation retained the same explicit scratch `--config` and mock
`--base-url`; stdin supplied synthetic tokens without recording their values.

| Public command/action | Observed result |
| --- | --- |
| `--profile scratch-alpha auth login --token-stdin --credential-store=file --no-input` | Exit 0; candidate validated and actual file profile stored |
| Equivalent login for `scratch-beta` | Exit 0; both stored credentials present |
| `profile list --json` | Exit 0; both names, safe metadata; no API request |
| `profile use scratch-beta --json` | Exit 0; only user default updated |
| `profile current --json` | Exit 0; selected scratch-beta, user source, legacy/manual unknown authorization |
| Same current with synthetic `TODOIST_TOKEN` | Exit 0; source env, external unknown authorization, inactive selected profile, no stored inspection |
| Project `default_profile` override + use/current | Override reported; project file unchanged |
| `profile remove scratch-alpha` | Exit 0; other credential retained |
| Remove selected scratch-beta; current | Removal exit 0; current exit 4, `PROFILE_NOT_FOUND`, dangling default retained |
| Repeated remove / missing use | Idempotent remove exit 0; missing use exit 4 |
| Manual/OAuth login without required noninteractive input/client | Useful errors, exit 2; no credential replacement |

## Initial friction and scoped repairs

**Humans and agents:** initial login suggested `todoist --profile scratch-beta today`,
dropping explicit scratch config and endpoint. Following it in the isolated XDG
environment returned exit 3 and missing-auth guidance. The implementation now
prints the resolved config, target profile, and explicit endpoint. The rerun executed
the exact suggested command: exit 0, the expected read-only `/tasks/filter` request,
and no unintended configuration access.

A separate safe storage review found nonselected-profile cleanup advice pointing
to unqualified `auth repair`, which repairs current selection. Removal errors now
include the explicit target and concrete retry/repair commands retaining config.
The rerun used a cgo-disabled scratch native descriptor, never a real native entry:
remove returned exit 3, `CREDENTIAL_CLEANUP_PENDING`, `committed=true`; the target
was disabled and the unrelated active profile/default stayed intact. Following
both recovery commands stayed safely pending because that backend was intentionally
unavailable. Successful cleanup recovery passed the fake-adapter contract test.

Harness assumptions about current's JSON field (`selected_profile`) and cancellation
exit were corrected from actual output and disclosed. They were not CLI defects.

## OAuth boundary and remaining limits

Local mock-provider execution exercised no-browser public PKCE, an already-ready
callback, `data:read`, explicit authorization-code exchange/verifier and no secret.
Offline current inspected the stored read-only profile. Environment override reported
unknown authorization without borrowing its evidence.

The rerun also covered rejected HTTP exchanges (`OAUTH_EXCHANGE_FAILED`, exit 3),
refresh-bearing/one-hour grants (`OAUTH_LIFECYCLE_UNSUPPORTED`, exit 3), provider
denial (`OAUTH_AUTHORIZATION_DENIED`, exit 3), and SIGINT (`OAUTH_CANCELLED`, exit 1).
Failure before storage byte-preserved prior credentials; no synthetic token or
provider canary appeared in output. OAuth scratch profiles were removed afterwards.

These are actual local CLI storage changes and mock-provider exchanges, not dry
runs and not live Todoist authorization verification. Official public discovery
was checked separately; a registered client, accepted redirect, and completed
Todoist exchange remain unverified. Real native cleanup, browser auto-opening, and
interactive terminal profile behavior were not exercised. The repository's manual
token terminal smoke test remains a separate fake-provider check.

No remaining actionable finding was observed within this selected workflow after
the assisted rerun. This conclusion does not cover the CLI's other resource workflows.
Full safe before/after transcripts were retained outside Git under the local evidence
directory `/tmp/todoist-profile-consumer.oxVauK/`.
The final working-tree rerun used a fresh scratch config and passed the same
profile/mock-OAuth workflow, including list/current repair hints retaining config
and target. Its transcripts are in `/tmp/todoist-profile-consumer-final.eFGdLk/`.
