# Errors and recovery

This change adds human guidance at the existing error and review-report rendering
boundaries. It does not change credential selection, authorization policy,
mutation ordering, replay evidence, error codes, or machine payloads. The user
workflow and executable examples live in [README](../README.md#errors-and-recovery).

## Bounded failure inventory

| Failure boundary | Evidence available | Safe next action |
| --- | --- | --- |
| Missing token before authenticated work | No credential resolved; operation did not start | Noninteractive token input or deliberate environment selection; retain invocation overrides. |
| Malformed manual token | No verification request or credential save | Supply the token alone through stdin. |
| Rejected manual token | Read-only validation failed; stored credentials unchanged | Obtain a current token, then repeat login using stdin. |
| API 401 during an authenticated operation | That request was rejected; earlier actions may have succeeded | Inspect credential source offline and prior action results before resubmission. API 403 is not classified as an invalid token. |
| Typed native-store unavailable or missing | Requested backend cannot supply the credential; any committed status remains in the existing error | Inspect metadata. Restore native support or replace a missing credential. File login is only suitable for a new/file profile; migration needs source access. |
| Command usage error | Existing usage classification and resolved command metadata | Read that command's help. Do not infer rollback or mutation absence from exit 2. |
| Pending review action | Persisted pending evidence marks a possibly dispatched mutation | Read exact task state, preserve evidence, manually reconcile before a fresh review. Never blindly retry. |

Other public failures remain on their existing paths: configuration parsing,
permission denial, locked/denied/corrupt credential stores, not-found and ambiguous
references, rate limits, generic transport/server errors, policy denial, stale
review snapshots, and ordinary-plan replay recording failures. This is not a
redesign of all command errors. Existing storage errors retain their code and
committed/cleanup details; the new hint does not imply a credential rollback.

## Rendering and compatibility

`writeError` retains original error text and envelopes. Error identity, existing
typed credential/API errors, and `CodeError` drive guidance; no arbitrary error
string matching is used. Usage help derives from command discovery metadata.
`writeReviewReport` uses its existing uncertain-outcome field, after structured
output has returned. Explicit JSON, NDJSON, plain, IDs-only, and quiet-JSON options
receive no new hints. NDJSON error stderr remains text where it already was text.
Default redirected output can include human advice; select an explicit machine
flag when depending on a stable contract. New diagnostics use stderr.

Authentication examples require the original configuration/profile/endpoint and
credential source. An active environment token keeps overriding stored credentials
after login. `auth status` is offline metadata inspection, not authentication.
No prompt is required for the documented inspection or stdin-login commands.

## Uncertain review limits

An uncertain flag means a mutation may have happened, not that it definitely did.
The saved plan and journal must remain intact. Inspect exact IDs with `task view`,
compare the plan with current fields and reported applied actions, and use Todoist
completed history/activity for completion or recurring-occurrence verification.
A missing task does not establish success. There is no safe automated way here to
adopt a remote result, clear pending evidence, or decide that resubmission is safe.
A fresh review is appropriate only after manual reconciliation; when the outcome
cannot be established, stop and verify in Todoist.

A hard process kill cannot emit advice. The plan saved before dispatch and retained
pending evidence remain the recovery basis. If the journal cannot be read, the
human renderer cannot derive its uncertainty flags; preserve files and verify the
remote state manually. The existing single-applying-process and durability limits
still apply. Ordinary task actions also retain pending-write evidence and block
uncertain replay; review plans additionally require task checkpoints. Ordinary
non-task actions retain their existing replay-recording limitations.

## Verification and domain impact

`go test ./internal/cli -run '^TestRecovery' -count=1` injects selected credential,
API, input, and review failures. It follows suggested login, help, metadata, and
exact-task inspection operations, checks secret redaction and machine modes, and
asserts that inspection does not mutate or change replay evidence.

`python3 scripts/test-recovery.py` exercises the bounded consumer path with a
local API fixture, isolated configuration, synthetic credentials, real PTY input,
and SIGINT after the fixture accepts a mutation. It is fixture verification, not
live Todoist verification. Run the ordinary `make check` gate as well.

Existing glossary terms cover credential source/profile, review plan, applied
action, and replay record. No CONTEXT.md or ADR change is proposed: ADR-0002 and
ADR-0004 already define the recovery and explicit-storage decisions. Published
changelog sections stay untouched, and repository policy forbids an unreleased
section; the PR carries the change evidence for the release orchestrator.

## Task writes and editing

`task delete` (including `task rm` and `task del`) requires a nonblank `--id`
or reference before credential lookup. A missing target exits 2 with a concise
human example and leaf-help pointer; explicit machine modes omit those hints.
Supply a target and `--yes`, including for a dry run.

Task writes dispatch once without retries or redirects. Transport failures,
HTTP 408/5xx, redirects, and malformed/missing required Sync acknowledgements
leave the outcome uncertain. Preserve the request ID; inspect exact task state
through read commands before any further mutation. Native Sync errors with
valid per-command rejection evidence are definite rejection; HTTP 200 alone is
insufficient acceptance.

A task_write_ack with result_available:false means accepted mutation with
unavailable optional resource data. Inspect state; do not treat it as rejection
or retry merely to obtain a task resource. A task_partial_edit means due clearing
was accepted but the subsequent fields were rejected/uncertain/not dispatched.
Stdout contains both step outcomes; stderr and nonzero exit signal incomplete
work. No automatic rollback or resubmission occurs.

Filtered move/complete batches preflight all pages and hierarchy before writing.
Preflight rejection dispatches zero and returns task_batch accounting when the
selection is available; failed selection fetch has empty stdout. Definite write
rejections continue; uncertainty stops remaining mutation targets. Known no-ops
remain accepted/dispatched:false. Reconcile uncertain targets separately from
rejected/unattempted targets. Dry runs dispatch zero writes.

Ordinary agent task actions now retain pending evidence before dispatch and
block replay of the current plan when a pending key exists, even with --on-error
continue. Acceptance atomically replaces pending with applied; definite
rejection clears pending. Preserve plan/journal/diagnostics and manually reconcile
state. Review plans retain their required checkpoints. An accepted write with
unavailable required checkpoint still stops and retains pending evidence.
