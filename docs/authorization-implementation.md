# Authorization verification

The [authorization contract](authorization-design.md) defines the behavior;
[ARCHITECTURE.md](ARCHITECTURE.md#authorization-boundary) describes its enforcement.
The domain glossary is in [CONTEXT.md](../CONTEXT.md), with the unknown-credential
compatibility decision in [ADR-0003](adr/0003-preserve-write-capability-for-unknown-credentials.md).

## Regression coverage

| Boundary | Coverage |
| --- | --- |
| OAuth | PKCE and device HTTP exchanges; exact requested scopes; omitted, reduced, broader, and unsupported grants; failed login preserves prior credentials |
| Persistence | Legacy loading without rewrites; atomic replacement; inactive profiles and unknown fields preserved; manual replacement and logout |
| API dispatch | REST and Sync mutations blocked before transport; recognized reads permitted; redirects checked with stable authorization errors |
| Machine output contract | Status, doctor, planner requests, previews, schemas, and human/JSON/quiet-JSON errors; no credential disclosure |
| Agent execution | Pending-action preflight; force/continue cannot override authorization; replay skips; plans contain no authorization snapshot |
| Schedules | Profile/config/endpoint/policy selections retained; cron and launchd preserve dry-run/force flags and metacharacters; cron line breaks rejected; credentials resolved at execution |

After changing these boundaries, follow the repository's
[handoff workflow](../CONTRIBUTING.md#ready-for-handoff): targeted regression tests
during development and `make check` or passing ordinary CI on the finished revision.

OAuth protocol tests use local HTTP servers. Live Todoist device authorization
remains unverified, and token refresh is outside this feature. External planners
are not sandboxed. Replay identity remains scoped to the configuration directory,
not the Todoist account; see the [replay decision](adr/0002-treat-replay-recording-as-part-of-action-success.md).
