# Architecture

This project follows a layered design:

1. CLI adapter (`internal/cli`) parses flags, handles terminal UX, and renders output.
2. App services (`internal/app/*`) hold command use-case rules and payload/query planning.
3. Domain logic (`internal/agent`) holds planner types and validation rules.
4. API adapter (`internal/api`) performs Todoist HTTP calls.

The intent is to keep business/use-case logic in app/domain packages and keep `internal/cli` focused on transport concerns (arguments, prompts, output modes, exit codes).

Installable agent guidance lives in `internal/agentskill`: curated workflows plus
an embedded command reference generated from live help. `internal/skillinstall`
owns explicit placement, deterministic package identity, owned-file manifests,
staging, backup, rollback, and removal. CLI lifecycle adapters render structured
results and classified errors before Todoist configuration or credential loading.
The [lifecycle contract](agent-skill.md) and [ownership decision](adr/0007-own-installed-skill-files-and-update-explicitly.md)
describe customization and recovery boundaries. Reference/example checks run in
the shared documentation gate; pending feature branches cannot supply references.

## Current flow

```text
cmd/todoist
    |
    v
internal/cli (flags, help, UX, output formatting)
    |
    v
internal/app/* + internal/agent (validation, planning, payload/query builders)
    |
    v
internal/api (HTTP client, request/response types)
    |
    v
Todoist API v1
```

## Where to change what

Choose the entry point for the behavior being changed; the links within a row are
alternatives, not a required reading bundle. Tests are representative starting
points. For command changes, use the existing [command-change checklist](README.md#command-discovery-metadata).

| Area | Implementation entry points | Existing behavior contract | Focused tests |
| --- | --- | --- | --- |
| Invocation state, global parsing, request context | [cli.go](../internal/cli/cli.go): `Context`, `Execute`, `parseGlobalFlags`, and `requestContext`. | [Parsing rules](SPEC.md#parsing-rules), [configuration](SPEC.md#config). | [flags_test.go](../internal/cli/flags_test.go): values, conflicts, interspersed globals; [config_load_test.go](../internal/cli/config_load_test.go): config loading. |
| Command dispatch, discovery, help, completion | Start at [dispatch.go](../internal/cli/dispatch.go) for execution or [command_metadata.go](../internal/cli/command_metadata.go) for discovery. Help routing: [command_help.go](../internal/cli/command_help.go); root/leaf content: [help.go](../internal/cli/help.go), [leaf_help.go](../internal/cli/leaf_help.go). Completion entry: [completion.go](../internal/cli/completion.go). | [Help and command recovery](SPEC.md#help-and-command-recovery); [discovery metadata and shell-specific ownership](README.md#command-discovery-metadata). | [command_metadata_test.go](../internal/cli/command_metadata_test.go): dispatch inventory; [help_routing_test.go](../internal/cli/help_routing_test.go): routing and side effects; [completion_test.go](../internal/cli/completion_test.go): shell selection/install. |
| Error rendering and recovery | [util.go](../internal/cli/util.go): `writeError`; [recovery.go](../internal/cli/recovery.go): human recovery hints; [cli.go](../internal/cli/cli.go): `toExitCode`. | [Errors and recovery](error-recovery.md). | [recovery_test.go](../internal/cli/recovery_test.go): recovery guidance and machine-error compatibility. |
| Task decoding, presence, output projection, schemas | Decode in [task_response.go](../internal/api/task_response.go); presence and faithful projection in [task_facts.go](../internal/api/task_facts.go). CLI output selection: [task_resource.go](../internal/cli/task_resource.go); task schemas: [task_schema.go](../internal/cli/task_schema.go), registered in [schema.go](../internal/cli/schema.go). | [Task-data contract](task-data-fidelity.md); [legacy compatibility decision](adr/0008-preserve-legacy-task-output-with-a-faithful-projection.md). | [task_facts_test.go](../internal/api/task_facts_test.go): presence, snapshot ownership, malformed facts; [task_resource_test.go](../internal/cli/task_resource_test.go): output selection; [schema_test.go](../internal/cli/schema_test.go): schema contracts. |
| Credential selection, storage, authorization | Selection: [cli.go](../internal/cli/cli.go) (`loadConfig`); lazy retrieval: [credential_store.go](../internal/cli/credential_store.go). Persistence boundary: [store.go](../internal/credentials/store.go). Policy: [authorization.go](../internal/authorization/authorization.go); HTTP guard: [api/authorization.go](../internal/api/authorization.go). | [Profile selection](profile-oauth-design.md#profile-selection-and-inspection), [storage](credential-store-design.md), [authorization](authorization-design.md), [security boundary](../SECURITY.md). | [profile_contract_test.go](../internal/cli/profile_contract_test.go): selection/output; [store_test.go](../internal/credentials/store_test.go): persistence/recovery; [authorization_test.go](../internal/api/authorization_test.go): resource and redirect guards. |
| Review, agent application, replay persistence | Review selection/snapshots: [app/review/review.go](../internal/app/review/review.go); interaction: [cli/review.go](../internal/cli/review.go). Shared apply loop: [agent_apply.go](../internal/cli/agent_apply.go); review preconditions/checkpoints: [review_apply.go](../internal/cli/review_apply.go); journal: [agent_replay.go](../internal/cli/agent_replay.go). | [Daily review](review-design.md); [replay success boundary and limitations](adr/0002-treat-replay-recording-as-part-of-action-success.md). | [agent_apply_test.go](../internal/cli/agent_apply_test.go): success recording and reruns; [review_apply_test.go](../internal/cli/review_apply_test.go): pending/checkpoint failures; [agent_replay_test.go](../internal/cli/agent_replay_test.go): journal persistence. |

## Invocation inputs

`internal/cli.Execute` uses process defaults and delegates to
`ExecuteWithEnvironment`. Its invocation inputs allow tests and embedded callers
to supply a clock, stdin, and environment lookup without replacing shared state.
CLI environment reads (including credential/profile overrides, OAuth settings,
planner selection, and display/completion settings) use that lookup. Filesystem
state, working directory, home-directory discovery, and child-process environments
remain process-owned; callers should select scratch paths explicitly.
OAuth adapters are held on each CLI context rather than mutable package globals.
Date-sensitive command contracts pin their reference clock, while concurrent
invocation tests exercise different credentials and UTC date windows.

An optional operation context follows the invocation through task resolution,
credential inspection, manual-token verification, and planner processes. Review
and OAuth signal contexts derive from that parent. Task service adapters honor
the supplied context and restore the enclosing invocation context afterward;
lookup caches and request metadata remain shared within the invocation.
Cancellation and expired deadlines stop bulk task operations and agent application
even in continue mode. Successfully recorded actions remain recorded; cancellation
does not undo Todoist mutations or resolve an uncertain remote outcome.

## Service coverage

- `internal/app/tasks`: list planning, single-task resolution, move/complete/delete guards, task mutation payload builders, returned due evidence and recurrence-preserving rescheduling, and hierarchy destination/selection rules. The CLI retains invocation-local ancestry fetching and caching.
- `internal/app/projects`: add/update/move validation plus project URL planning for browse flows.
- `internal/app/filters`: add/update/delete validation, payload construction, and filter reference resolution rules (exact/direct/fuzzy/ambiguous).
- `internal/app/comments`: comment list/add/update validation and payload construction.
- `internal/app/labels`: label list query planning and add/update payload validation.
- `internal/app/sections`: section list query planning and add/update payload validation.
- `internal/app/agent`: plan preparation and agent action-to-API request planning for `agent apply/run`.
- `internal/agent`: plan/action types, action validation, and summary derivation.
- `internal/app/activities`: activity query validation and date/type filters.
- `internal/app/assignees`: assignee lookup and reference resolution.
- `internal/app/notifications`: notification filtering and action validation.
- `internal/app/refs`: shared URL parsing and reference matching.
- `internal/app/reminders`: reminder target and schedule validation.
- `internal/app/settings`: user settings validation and update payloads.
- `internal/app/stats`: productivity goals and vacation updates.

## CLI-local orchestration

Task creation shares a capture receipt renderer in `internal/cli`; other task
rendering and machine output remain separate. The API task decoder retains due
field presence, recurrence, and timezone as returned facts excluded from
legacy JSON/NDJSON serialization. An immutable response snapshot retains supported
task and nested facts independently of typed defaults. An explicitly selected
task-resource v2 projection preserves their presence and values; it does not change
`api.Task` serialization used by planner/review contracts. Receipts use best-effort destination name
lookups and never infer saved state from capture input.

Human task detail reuses complete project and section collections held by the
CLI's invocation-local lookup cache. A complete global sections collection can
serve detail enrichment after selection; failed or partial loads cannot. Detail
still checks section ownership, and the cache does not change reference matching
or persist between commands.

Output writers serialize raw values; the CLI owns the machine error envelope and
its request metadata. Machine-output pagination notices remain on stderr.

Section deletion owns its confirmation and dry-run checks in `internal/cli`.

Agent status rendering, apply, and replay persistence remain in `internal/cli`;
the app layer prepares plans and plans requests. See the [replay-recording decision](adr/0002-treat-replay-recording-as-part-of-action-success.md)
for the success boundary and persistence limitations.

Review selection/snapshot ordering lives in `internal/app/review`; the CLI owns
line prompts and rendering. Optional `internal/agent.Review` metadata accounts
for every selected task and maps dispositions to existing actions. Both agent
application entry points enforce review preconditions through the existing apply
loop. The replay journal stores review pending markers and post-update snapshots
alongside ordinary action records. Existing task API output models are unchanged;
review reads its own snapshot projection. See [daily review](review-design.md).

## HTTP and Sync transport

JSON and Sync form requests share `internal/api.Client.doRequest` for dispatch,
request-body replay, transient status/transport retries, and response reads. Every
attempt still passes through the authorization boundary. REST mutation retries
require an idempotency request ID. Sync reads are retry-safe; Sync mutations are
retried only when every command carries its original non-empty UUID, as described
in [Todoist's Sync command contract](https://developer.todoist.com/api/v1/).
Request IDs and encoded command bodies remain identical across at most three
attempts; cancellation and request deadlines bound retry waits.

Workspaces, current-user lookup, filters, reminders, settings, and notifications
use one Sync response type and decoder. Successful bodies are read completely,
like REST responses, and read failures are returned before decoding. This removes
silent 16 KiB/256 KiB truncation. Accepted mutations with optional advisory bodies
retain their separate bounded best-effort handling; malformed Sync responses and
provider command errors are returned without retry.

## Authorization boundary

`internal/authorization` owns scope evidence, metadata validation, the safe authorization report, and permission errors. `internal/config` defines credential records and preserves optional raw metadata, including unsupported records in inactive profiles. `internal/credentials` owns profile persistence, atomic replacement, and recovery across file and native storage. The CLI resolves the credential source first, so an environment token never inherits a profile's scope evidence.

Constructing an API client requires an explicit resolved authorization report, preventing callers from silently dropping profile metadata. Every Todoist resource request passes through `internal/api.Client.dispatch`. Its underlying HTTP client is private; embedding/tests can supply a transport without gaining an unguarded dispatch API. Known GET resource routes and command-free Sync resource reads are classified as reads. All other operations require write capability. Redirects pass through the same policy before the next request is sent; destinations outside the configured API origin/path are unclassified. Sync POSTs are inspected because the same endpoint supports reads and mutations. OAuth exchange uses its separate credential-acquisition transport.

The API guard is authoritative. Agent preflight calls the same authorization policy before dispatching any pending action, and authorization errors abort even under continue mode. Bulk task commands propagate these errors instead of converting them to a successful partial-failure summary. Replay skips perform no mutation and remain available with a read-only credential. Planning and previews report current authorization without putting authority into executable plans.

Unknown credential compatibility is deliberate; see [ADR-0003](adr/0003-preserve-write-capability-for-unknown-credentials.md). The [authorization contract](authorization-design.md) separates metadata absence from invalid metadata and describes the machine output contract. External planners and older CLI binaries are outside the guard's enforcement boundary.

## Credential storage boundary

Profile commands use `credentials.Store.List/Inspect/Delete`; list/current/use
never retrieve native secrets or make API calls. Configuration selection remains
outside the store. A narrow user-default update preserves raw unknown config
fields and excludes merged project/environment values. Removal retains dangling
selection rather than selecting fallback credentials; cleanup uses the existing
disabled-before-delete protocol. See the [profile/OAuth contract](profile-oauth-design.md).

`internal/credentials` owns profile persistence and recovery. CLI workflows use its
Store interface for load/save/delete, metadata inspection, enumeration, migration,
repair and health. A smaller Secrets adapter handles native secret CRUD/probing;
file persistence is injectable for failure tests. Selection uses user configuration
and explicit auth flags outside resource workflows. Secret retrieval is lazy at
the authenticated client boundary, so help/status/local work needs no native access.

Keychain entries use immutable generations. A durable token-free transaction record
identifies staged/obsolete entries; the atomic local profile replacement selects
token and authorization together. Recovery confirms file durability before deleting
an obsolete entry. Native calls serialize process-global interaction policy and
never display dialogs. Build-tagged stubs preserve portable compilation without cgo.
See [ADR-0004](adr/0004-select-credential-storage-explicitly.md).
