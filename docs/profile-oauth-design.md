# Credential profiles and OAuth onboarding

Status: accepted by the user on 2026-09-30, before implementation. Base: main
`1ea11d5`. This feature preserves the already-merged manual-token validation.

## Profile selection and inspection

`profile list`, `profile current`, `profile use NAME`, and `profile remove NAME`
are discoverable top-level commands. Creation remains `--profile NAME auth login`.
Profiles represent credentials/grants; they neither prove distinct Todoist
accounts nor deduplicate grants for one account. Account identity stays unknown.

Selection precedence is flag, environment, project default, user default, then
literal `default`. Config selection is `--config`, `TODOIST_CONFIG`, then the
XDG/home path. Project config is still loaded from the current directory with an
explicit user config. Credentials remain scoped to that user config's directory.

List/current call the existing store's metadata APIs, never Load, Probe, native
enumeration, or the Todoist API. Accessibility remains `unchecked`. List includes
disabled cleanup rows and safe errors for malformed metadata; a row error does
not hide healthy rows, and the command returns exit 3 after reporting all rows.
Whole-file corruption returns a storage error without rewriting state.

Current separates `selected_profile`/`selection_source` from the effective
credential `source`/`authorization`. An environment token reports external unknown
authorization and `profile_active=false`, without stored-profile inspection.
Without the override, current reports selected-profile metadata; missing or
disabled profiles return a report and exit 4. Invalid/unsupported metadata fails
closed, retaining null mode and no permissive unknown classification.

JSON and NDJSON emit one complete report/acknowledgement. Published schemas are
`profile_list`, `profile_current`, `profile_use`, and `profile_remove`. Existing
auth status fields, raw resources, IDs-only eligibility, and error streams stay
compatible. Errors about a target profile attach only that profile's evidence;
OAuth errors never borrow the credential being replaced's evidence.

## Persistence and removal

Use requires an existing, enabled profile with valid or absent legacy metadata.
It checks no native accessibility and makes no API request. Persist only
`default_profile` in the resolved user config, preserving unknown fields and
avoiding project/environment values from merged configuration. Serialize and
atomically replace the local config. Failed or uncertain durability is an error,
never a successful acknowledgement or an unsupported rollback claim.

Use retains flag/environment/project precedence and reports a saved default
shadowed by those selections, plus any environment-token override. Project config
is never mutated. Selecting read-only or unknown-compatible credentials is valid.

Remove requires an explicit name, is idempotent when absent, and reuses Store.Delete.
Defaults and invocation selections are retained even when dangling. No fallback
credential is selected. Invalid authorization evidence can be removed; structurally
corrupt/unsupported storage is preserved for deliberate recovery.

Native removal durably disables the local credential before native deletion.
Cleanup failure leaves it disabled, returns `CREDENTIAL_CLEANUP_PENDING` with
`committed=true`, and supports repeated remove or selected-profile auth repair.
Only recorded references in the current namespace are cleaned. This is not atomic
cross-store deletion, remote revocation, or forensic erasure. The existing storage
[transaction/recovery contract](credential-store-design.md) remains authoritative.
See [ADR-0005](adr/0005-retain-selection-after-profile-removal.md).

## OAuth behavior

Manual login remains the default and its candidate validation is preserved.
Explicit `--oauth` keeps flag/environment client-ID overrides, read-only scope,
no-browser behavior, and explicit export. The callback listener starts before
browser launch; state validation precedes callback success or denial. Repeated
callbacks cannot block handlers. Context cancellation reaches waits, exchanges,
and storage. Cancellation before publishing the selected record preserves the
previous credential; staged journal/native work is repaired or reported as recovery.
Once selected-record publication starts, a completed change or explicit recovery
state follows the existing transaction contract, without claiming rollback. Provider bodies, arbitrary callback text, and secret-bearing errors
are excluded from diagnostics.

Authorization requests include response_type=code, state and S256 PKCE; exchanges
include grant_type=authorization_code, redirect and verifier. A client secret is
never embedded. Configurable mock-provider device flow remains available only with
an explicit device endpoint. Default Todoist discovery does not advertise that flow.

Token refresh remains outside this feature. Reject refresh-bearing token responses
and any finite expiry other than Todoist's documented legacy `315360000` value
before save/export; omitted expiry remains legacy-compatible. Invalid lifecycle
metadata is rejected. The stable code is `OAUTH_LIFECYCLE_UNSUPPORTED`, exit 3.
Rejection preserves the previous credential. See
[ADR-0006](adr/0006-reject-unsupported-oauth-token-lifecycles.md).

## OAuth prerequisites

Read-only official discovery on 2026-09-30 returned authorization_code/refresh_token,
S256, public-client authentication `none`, a registration endpoint and support for
HTTPS Client ID Metadata Documents. It advertised no device authorization endpoint.
This verifies public provider metadata, not a completed user authorization/exchange.

No maintainer-owned client ID, metadata URL, or accepted redirect was found in
tracked project configuration. No application was registered or authenticated.
For a future default, the maintainer must provide either a genuine public-client
registration, or an HTTPS URL they control serving a public JSON document with
matching `client_id`, accepted `redirect_uris`, and `token_endpoint_auth_method: none`.
Confidential App Console clients cannot be assumed usable without a secret.

Todoist docs allow localhost redirects for testing. Literal 127.0.0.1 and any
production loopback policy remain unverified for this project; configure and
verify the exact callback. Live verification then requires human Todoist approval
using scratch credential storage, followed by checks of exchange, effective scopes,
token lifetime and credential preservation. New apps issue one-hour grants, so
durable refresh support is also required before advertising general new-client
onboarding. No mock or discovery test resolves these dependencies.

Primary sources:

- [Todoist OAuth](https://developer.todoist.com/api/v1/#tag/Authorization/OAuth)
- [Dynamic registration](https://developer.todoist.com/api/v1/#tag/Authorization/Dynamic-Client-Registration)
- [Client metadata documents](https://developer.todoist.com/api/v1/#tag/Authorization/OAuth-Client-ID-Metadata-Document)
- [Refresh lifecycle](https://developer.todoist.com/api/v1/#tag/Authorization/Refreshing-access-tokens)
- [Live public discovery](https://api.todoist.com/.well-known/oauth-authorization-server)

## Verification boundaries

Unit/contract tests use temporary configurations, fake native adapters and local
HTTP providers. The consumer workflow uses two scratch profiles and isolated
configuration, records its initial public route and implementation-knowledge limit,
then fixes and reruns scoped friction. No existing user credentials are removed.
Live Todoist exchanges and real Keychain behavior are separately bounded external
verification; mock tests and a passing local gate do not establish either.
