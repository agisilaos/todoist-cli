# Read-only OAuth and authorization metadata

Status: accepted. The design-tree interview and concrete contracts were confirmed before test-first implementation.

## Accepted authorization model

Authentication, authorization scopes, authorization mode, credential source, credential origin, and write capability are separate concepts defined in `../CONTEXT.md`. Authorization is part of the machine output contract.

- OAuth login accepts `--read-only` with either `--oauth` or `--oauth-device`. Without that flag, OAuth requests read-write access as before. Using the flag without an OAuth flow is a usage error; it does not constrain an opaque manually supplied token.
- Read-only requests use exactly `data:read`. Read-write requests use exactly `data:read_write,data:delete,project:delete`.
- An explicit token-response scope determines effective scopes. An omitted scope uses the requested scopes according to OAuth's successful-exchange semantics. An explicit empty or unusable scope is not an omitted scope.
- A read-only request returning mutation permissions fails without replacing the previous credential. A read-write request returning only read access stores read-only authorization and reports the reduction. Unsupported or unusable grants fail instead of becoming permissive unknown credentials.
- Enforcement is binary. Read-only blocks all Todoist mutations. Read-write permits attempted mutations; Todoist enforces individual granted permissions. Requested scopes and effective scopes remain distinguishable.
- Legacy token-only credentials, manual tokens, and environment tokens have unknown authorization and may attempt writes for compatibility. Their mode remains unknown. There is no configuration switch for this policy.
- Present but invalid, contradictory, or unsupported authorization metadata blocks authenticated operations. Absence of metadata is the legacy compatibility case.

## Accepted credential lifecycle

The later accepted [profile/OAuth contract](profile-oauth-design.md) adds explicit
profile selection/removal and limits OAuth exchanges to supported legacy token
lifecycles. Current under an environment override reports external authorization
without inspecting stored evidence; target-profile and candidate-grant errors
never attach another credential's evidence.

Credential selection retains existing precedence. The environment token overrides the selected profile and does not inherit its metadata. Selecting a different profile resolves its token and metadata together.

Successful login replaces one profile's token and metadata together; failed login or failed persistence preserves the previous record. Other profiles and unrelated local changes are preserved. Manual replacement removes any previous OAuth evidence. Logout removes the selected stored credential and its metadata without remotely revoking the token or unsetting an environment variable.

Loading a legacy record does not rewrite the file or invent origin or scopes. `--print-env` remains an explicit token export without storage: later environment use has unknown local authorization, even though Todoist retains the token's actual grant.

Token refresh remains outside this feature. Persisted authorization information does not establish current token validity or extend its lifetime. Todoist device authorization support remains unverified: the existing configurable client flow will gain scope selection and protocol tests, without claiming verified live provider support.

## Accepted enforcement and command behavior

All Todoist resource requests converge on one guarded API dispatch boundary. Explicitly recognized reads remain available. Other operations require write capability. Sync reads use POST, so Sync commands must be distinguished from resource reads. OAuth credential exchange remains separate and usable when the existing profile is read-only.

The full command and alias classification is in `authorization-command-inventory.md`. Local credential/configuration/completion changes, inspection, plan construction, and schedule generation remain available. Notification read-state changes, account settings, goals, and invitation responses are Todoist mutations.

A read-only profile may construct a review plan or agent plan. Dry runs may read Todoist and perform documented local effects, but dispatch no Todoist mutations. Existing validation, agent policy, and confirmation requirements still apply. An external planner is an independently executing trusted program; the CLI does not sandbox it or prevent its own network activity.

Before applying an agent plan, all pending actions are checked before the first mutation. Authorization denial fails the application even with `--force` or `--on-error=continue`; it creates no replay records and does not mark the plan applied. All 20 current agent action kinds mutate Todoist. Mixed-action plans currently mix mutation kinds; no read/local action types will be introduced.

An all-replayed application may succeed with zero newly applied actions. Empty ordinary agent plans cannot be applied; empty previews remain valid. Keep/skip-only review plans may be applied as successful no-ops. Replay and last-plan storage remain scoped to the configuration directory, not the account. Cross-profile application checks the current credential; account-bound plans and replay migration remain separate work. An applied action still requires a successful Todoist mutation and durable replay recording, as specified in ADR-0002.

Schedules carry the resolved profile and configuration path, plus relevant explicit endpoint and agent-policy selections. They resolve credentials and authorization at execution time, without embedding tokens or saved write permission. Environment precedence still applies at execution. Printing a schedule does not install or execute it.

## Storage contract

Keep the existing `profiles` mapping. File-backed profiles retain the `token` field; native profiles omit it and use a `storage` descriptor referencing the native token, as specified in the [credential storage contract](credential-store-design.md). Both backends preserve the optional per-profile `authorization` object:

```json
{
  "version": 1,
  "mode": "read-only",
  "origin": "oauth-pkce",
  "requested_scopes": ["data:read"],
  "effective_scopes": ["data:read"],
  "scope_evidence": "token-response"
}
```

This example is only the authorization object, not a credential export. No token is included in diagnostic examples.

- `version`: authorization metadata format version, initially `1`; no new top-level credential-file version.
- `mode`: `read-only`, `read-write`, or `unknown`.
- `origin`: `oauth-pkce`, `oauth-device`, `manual`, or `unknown`. Legacy and environment origins are unknown; they are never retrospectively called OAuth or manual.
- `requested_scopes` and `effective_scopes`: arrays for known OAuth information, `null` for unknown information. An empty known grant is not equivalent to unknown scopes.
- `scope_evidence`: `token-response`, `oauth-request`, or `none`. `oauth-request` specifically means an omitted scope in a successful exchange, not an uncompleted authorization request.

New manual records persist unknown mode, manual origin, null scope sets, and no scope evidence. Legacy records remain unchanged on disk. Credential source and effective write capability are computed per invocation and never stored as credential facts. No token fragments, fingerprints, refresh tokens, or timestamps are added to this metadata.

Required metadata fields and their consistency are validated. Additional unrecognized fields within a supported version are tolerated and preserved when other records are saved. Explicit null, incomplete, contradictory, or unsupported-version metadata is not treated as an absent legacy object. Invalid metadata in an inactive profile does not poison a valid active profile or an environment override.

For this bounded OAuth feature, the supported read-only grant is `data:read`. A supported read-write grant contains `data:read_write`, optionally accompanied by `data:read`, `task:add`, `data:delete`, or `project:delete`. Redundant implied permissions are retained if returned; they are not invented when absent. Other grant combinations are rejected as unsupported, including write-only grants and optional scopes outside this feature. A read-only login never accepts a write-capable grant.

## Reporting contract

`auth status` keeps its existing `profile`, `configured`, and `source` fields and adds `authorization`. It stays offline. The authorization report includes:

- `metadata_version`: `1` for supported persisted metadata, the observed integer for an unsupported version, or `null` when absent or unreadable.
- `metadata_status`: `valid`, `legacy`, `external`, `missing`, `invalid`, or `unsupported`.
- `mode`, `origin`, `requested_scopes`, `effective_scopes`, and `scope_evidence` as defined above.
- `write_capable`: effective boolean permission to attempt a Todoist mutation.
- `write_capability_reason`: `read-only`, `read-write`, `unknown-compatibility`, `no-credential`, `invalid-metadata`, or `unsupported-metadata`.

With no credential, `configured` is false, mode and scope sets are null, and write capability is false. Invalid or unsupported metadata also yields a null reported mode rather than falsely presenting a legitimate unknown credential; its write capability is false. Known fields from invalid metadata are not echoed as trusted facts.

Human output uses the same distinctions: for example, `read-only; writes blocked`, or `unknown; writes allowed for compatibility; scopes unknown`. It describes a configured credential, not remotely verified authentication.

Doctor includes the same safe authorization report in credentials-check details and a useful human summary. Read-only and unknown are valid configurations and do not alone fail strict diagnostics. Invalid metadata is a diagnostic failure, and the authenticated API probe is skipped. The existing separate read-only API probe tests acceptance, not scope discovery.

`auth status` remains available to inspect invalid metadata and exits nonzero after reporting it. Login/logout can replace/remove an invalid selected authorization object when the credentials file is structurally readable, while preserving other profiles. A syntactically corrupt whole file is reported without destructive automatic recovery.

Add schema entries for authorization reports, auth status, and doctor. Keep the existing output envelopes and stream/quiet-JSON conventions. Existing raw resource JSON and IDs-only streams gain no authorization decorations.

## Agent and login reporting

Add the current safe authorization report to planner requests and apply/run dry-run previews. Previews remain successful even when `write_capable` is false; that field reports permission to apply, not preview validity. Include current authorization in agent status, clearly separate from last-plan information. Authorization in planner requests is advisory: planners may still propose mutations.

Do not persist a write-capability snapshot in the executable plan or let a plan supply authorization. Existing plan version and replay keys remain unchanged. Authorization is resolved again for actual execution.

Successful stored-login output reports the authorization of the credential just stored. If an environment token currently overrides it, report that separately rather than claiming the stored credential is active. Logout similarly reports that an environment token can remain active. Explicit `--print-env` retains its intentionally secret-bearing export contract; ordinary status and diagnostics never expose it.

## Authorization errors

A blocked mutation exits `3` with symbolic code `READ_ONLY` and the exact error string `Todoist mutation blocked: the active credential is read-only.` JSON retains the existing `error` and `meta` fields and adds `code` plus safe details identifying profile, credential source, and authorization. Human remediation explains how to select or obtain a write-capable credential; `--force` is never an override.

Metadata errors also exit `3`:

- `AUTH_METADATA_INVALID`: `Stored authorization metadata is invalid; log in again or remove the affected profile.`
- `AUTH_METADATA_UNSUPPORTED`: `Stored authorization metadata uses an unsupported version; upgrade the CLI or replace the affected credential.`

An unacceptable OAuth grant uses exit `3`, code `OAUTH_SCOPE_INVALID`, and message `OAuth returned an unacceptable scope grant; the stored credential was not changed.` Safe details distinguish a broader-than-requested grant from an unsupported or unusable grant.

Messages never include tokens or raw malformed metadata. Doctor keeps its existing report and diagnostic-failure exit behavior instead of replacing the report with a mutation error.

## Implementation and verification

Implement test-first, beginning with requested OAuth scopes, scope-response handling, credential compatibility/lifecycle, and requests blocked before mutation transport. Cover all REST and Sync mutation families, legitimate Sync reads, agent preflight/replay behavior, schedules, status/doctor reports, and human/JSON/quiet-JSON errors. Tests must show that `--force` and error-continuation settings cannot bypass authorization.

Update README, specification, architecture documentation, schemas, examples, completions, and generated help snapshots as affected. Preserve unrelated work. Follow the repository [handoff workflow](../CONTRIBUTING.md#ready-for-handoff), including affected schema and documentation regression tests. Do not commit or open a pull request.

## Sources and related decisions

- [Todoist OAuth documentation](https://developer.todoist.com/api/v1/#tag/Authorization/OAuth)
- [OAuth token-response scope semantics](https://www.rfc-editor.org/rfc/rfc6749#section-5.1)
- [Doist CLI setup reference](https://github.com/Doist/todoist-cli#setup)
- [Todoist authorization-server metadata](https://api.todoist.com/.well-known/oauth-authorization-server)
- `adr/0003-preserve-write-capability-for-unknown-credentials.md`

## Task editing inventory

Native task due clearing/rescheduling (item_update), hierarchy clearing
(item_move), and permanent completion (item_complete) use the same guarded Sync
mutation boundary. REST task fields/reference/order updates and existing agent
task actions use guarded writes. Expanded views and sorting are reads. Known
already-satisfied reference/hierarchy no-ops may succeed with read-only
credentials because they dispatch zero writes. Dry runs retain read access and
report authorization without mutating. Sequential edits check authorization
before their first write; batches fully preflight and check before dispatch.
