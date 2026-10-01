# Complete task editing

Status: accepted on 2026-10-01; implementation and ship-change review authorized.
Base: freshly fetched `origin/main`, `7dbe10ee9513e70858d862830c742cba13a07b61`.
Worktree branch: `fix/complete-task-editing`.

## Scope and command vocabulary

Deliver all three phases: mutation semantics and recovery; opt-in child expansion and sorting; documentation, contracts, tests, public consumer review, scoped corrections, and rerun.

```text
task update <ref> --clear-due --clear-deadline --clear-labels
                  --clear-assignee --clear-description
task move <ref> --clear-parent --clear-section
task reschedule <ref> (--due-date DATE | --due-datetime RFC3339 |
                       --due-local-datetime YYYY-MM-DDTHH:MM:SS)
task complete <ref> --forever
task add/update ... --reference[=true|false] --order N
task view <ref> --include-children
```

Preserve ordinary due editing in task update and ordinary recurring completion in task complete. Project/section administration, new agent-plan action kinds, and platform expansion remain outside scope.

Preserve the legacy default task output, frozen opt-in v2 projection and allowlist, null/presence semantics, numeric machine priorities, IDs-only eligibility, and human-only `--full`.

## Input presence and conflicts

- Omission means unchanged. Empty description, explicit `reference=false`, and `order=0` are edits. Other empty setters are usage errors; use clear flags where supported.
- Before API reads, reject setter/clearer pairs, multiple due setters, conflicting target selectors, duplicate single-valued natural tokens, and natural tokens overlapping explicitly set or cleared fields. Natural input fills only unspecified fields; repeated labels remain supported.
- `--clear-due` conflicts with due-language editing. Ordinary due-language editing remains available.
- A project accompanying a move section scopes section resolution rather than becoming another move destination.
- `--description -` reads exact text, including whitespace and a trailing newline. Empty input clears description. `--content -` uses existing add trimming behavior for add and update. Dual content/description stdin and description setter/clearer pairs fail before API access.
- Reference controls add/remove exactly one leading `* `, retain already-matching titles, and reject repeated prefixes or a blank remaining title. Ordinary content edits without reference controls retain supplied text. Reference-only edits read the current exact task.
- Order accepts the verified signed int32 range, including zero. REST create uses `order`; REST update uses `child_order`.

## Mutation operations and combined edits

Use documented request shapes. REST update clears assignee/deadline with null, labels with an empty array, and description with an empty string. Sync `item_update` clears due with `due:null`; a full due object reschedules. Sync `item_complete` permanently completes a task and its subtasks. Ordinary REST close advances recurrence.

Due clearing alone dispatches one Sync write. Fully preflight combined edits, then clear due through Sync and submit all remaining fields through one REST update: at most two writes. Stop if the first step is rejected or uncertain. If it is accepted and the second step is rejected or uncertain, emit the separate per-step partial-result report and exit nonzero with read-only reconciliation guidance. Never roll back or retry automatically. Full success preserves a valid final returned task or uses the accepted-write fallback when optional resource data is unavailable. See ADR-0011.

## Hierarchy and established no-ops

| Requested clear | Established destination |
| --- | --- |
| Parent | Root in the current effective section, or current project when sectionless |
| Section | Remain in the current project; an inherited section requires explicit parent detachment |
| Parent and section | Current-project root |

Clear flags can accompany one another but not project/section/parent destination setters. Walk exact parent links to a known root to establish effective project/section; refuse missing links, cycles, mismatched identities, or contradictory placement. A child's null section alone does not establish effective sectionlessness. Sync `item_move` to a section or project explicitly establishes the promised root destination.

A proven already-satisfied hierarchy clear or already-matching reference-only edit dispatches zero writes and emits `{id, status:"unchanged", operation}`. A proven sectionless child may clear section as a no-op while retaining its parent. Batches count these as accepted with `dispatched:false`, separately from actual mutations. Proven no-ops remain available to read-only credentials.

## Rescheduling evidence and time handling

- Require primary returned `due.date`, explicit recurrence, and established timezone character: date/floating with explicit null timezone, or fixed time with a valid named timezone. Recurring tasks also require returned expression and language. Nonrecurring updates omit a stale expression.
- Present compatibility `datetime` must agree by wall time or instant. A calendar date plus datetime must agree with that datetime's local calendar date, using the returned timezone for a fixed instant. Missing primary date or malformed/conflicting compatibility data refuses rescheduling. Compatibility `datetime` is not a current request field.
- Date targets preserve a timed task's floating wall clock or fixed-zone local clock. Local datetime targets apply to existing floating tasks. RFC3339 targets select an exact instant for existing fixed-zone tasks while retaining their timezone. Date-only tasks take date targets; never silently convert time character.
- Preserve established recurrence in the native full due object. Refuse insufficient/contradictory evidence, unknown zones, unsupported conversions, and date replacements producing DST gaps/folds. RFC3339 can disambiguate a fixed-zone fold; floating time needs no DST conversion.

## Collections and sorting

Opt-in expanded views fetch every page of direct active children. Ordinary task view makes no child request. Validate collection/cursor presence, retain request parameters, detect cursor cycles, and buffer expansion before output. Failed expansion emits the existing error with empty stdout. Pagination exhaustion establishes completeness without a concurrent snapshot guarantee.

Filtered move/complete supports hierarchy clearing and permanent completion; reschedule remains single-target. Fetch and preflight the complete selection before confirmation/writes, including strict pagination, usable exact identities, duplicate checks, and ancestry traversal outside the selection. Reject ancestor/descendant overlap. Preflight failure dispatches nothing. Retain `--yes`/`--force` and zero-mutation dry runs. Continue definite runtime rejections, stop on uncertainty, and report accepted/rejected/uncertain/unattempted targets. Incomplete batches exit nonzero.

Explicit sorting applies to active, filtered, preset and completed lists, Today, Inbox, Upcoming, Completed, filter show, and expanded children:

```text
--sort due|deadline|priority|added|updated|completed|content|order|none
--sort-order asc|desc
```

| Key | Comparison | Default direction |
| --- | --- | --- |
| due | Calendar date then local clock; date-only before timed on the same date | asc |
| deadline | Calendar date | asc |
| priority | Numeric API priority | desc |
| added, updated, completed | RFC3339 instant | desc |
| content | Case-sensitive ordinal text | asc |
| order | Exact numeric `child_order`, without mixing `order_key` | asc |
| none | Fetched order | No direction permitted |

Fixed due timestamps use a valid returned named timezone; without one, sorting uses the timestamp's own offset. Contradictory/malformed keys are unavailable. Missing keys stay last in either direction; opaque ID breaks ties ascending independently of direction. Sort the fetched selection; `--all` permits sorting across every fetched page. Direction without an explicit sort is a usage error; an unexpanded view rejects sorting flags. Omitted flags preserve all defaults, including Upcoming's existing ordering.

## Acceptance, dispatch, and recovery

Task writes dispatch once without redirects or automatic retries, including existing agent task actions. Transport failures, HTTP 408/5xx, redirects, and missing/malformed required acknowledgement remain uncertain. Unrelated resource retries remain unchanged.

Exact Sync command UUID to `"ok"` is the only Sync acceptance. A correctly typed recognizable documented error object establishes rejection; malformed, contradictory, arbitrary, or empty acknowledgements remain uncertain. Parse required acknowledgement independently of optional task resources. Optional data unavailability cannot reverse established acceptance.

Ordinary agent task actions persist pending evidence before dispatch, clear it on definite rejection, and publish the applied record while clearing pending evidence on acceptance. Uncertainty stops application regardless of continue mode and blocks blind rerun pending manual reconciliation. Check pending action keys before any application. Executable plan/action schemas remain unchanged; the existing local replay journal extends explicitly. Preserve review pending/checkpoints and the accepted-mutation-plus-replay-record success requirement. See ADR-0002 and ADR-0009.

## Output contracts

Keep ordinary v1/v2 task resources unchanged. Publish distinct schemas and command-result unions for:

- Expanded JSON and single-record NDJSON: `{task, children, children_complete:true}`. Nested resources use the selected task version.
- Accepted add/update/reschedule with unavailable optional task data: `{status:"accepted", operation, result_available:false, id?}`. Creation omits unknown ID. Preserve valid returned resources in their established representation.
- Established no-op: `{id, status:"unchanged", operation}`.
- Partial combined edits: per-step outcomes/request IDs, with nonzero exit and read-only reconciliation guidance.
- Batches: complete target accounting and actual dispatch counts, including no-ops and unattempted targets.

Human/plain output distinguishes acceptance from unavailable resulting state and directs exact-ID inspection when possible. Never fabricate a returned task from requested or pre-action fields. See ADR-0008 and ADR-0010.

## Verification and consumer review

Cover exact requests and dispatch counts for presence/null/empty/false/zero, conflicts, hierarchy/ancestry, recurrence/timezone/DST, pagination, sorting, reference titles, read-only credentials, rejected/uncertain writes, malformed acknowledgements, partial edits, compatible output, and agent recovery. Update README, focused help, completion, snapshots, schemas, SPEC, task-data fidelity, and bundled/installable agent references. Regenerate command reference with the repository tool. Run focused tests, `go test ./...`, and `make check`.

Use cli-consumer-review for the public scratch lifecycle with isolated configuration, synthetic credentials, and a local fixture: README/help; nested recurring creation/inspection; child expansion/v2; rescheduling; non-due clearing/hierarchy detachment; reference/order controls; permanent completion. Clear due on a separate task. Disclose implementation knowledge, record initial friction before fixes, verify resulting state/mutation counts and relevant noninteractive recovery/stdout/stderr, correct feature-specific findings, and rerun. Fixture and dry-run verification are not live Todoist verification.

Preserve primary-checkout release work and unrelated changes. The user subsequently authorized ship-change and PR creation on 2026-10-01. Merge and release remain outside scope. PR evidence screenshots stay outside Git repositories under the user's memory preference.

## Implementation structure

Reuse returned task facts rather than introducing another task model. Share low-level due parsing, while keeping preservation and sorting policies separate. Share strict task-page validation for the new completeness guarantees and one invocation-local exact-ID cache for ancestry reads. Extend existing prepared-action/replay persistence rather than adding another apply loop or journal. Use small shared acknowledgement/outcome types and compose new schemas from the frozen task-resource definitions. Avoid unrelated resource or rendering refactors.

## Verified API evidence

Verified against current official API v1 documentation on 2026-10-01; this is documentary evidence, not live Todoist verification.

- REST [create](https://developer.todoist.com/api/v1/#operation/create_task_api_v1_tasks_post) and [update](https://developer.todoist.com/api/v1/#operation/update_task_api_v1_tasks__task_id__post): order field names/ranges and explicit non-due clear values. Update due setters are strings; a null REST setter is undocumented.
- [Sync task update](https://developer.todoist.com/api/v1/#tag/Sync/Tasks/Update-a-task) and [due-object construction](https://developer.todoist.com/api/v1/#tag/Due-dates/Create-or-update-due-dates): due clearing/full-object editing. Sync update does not document `child_order`; numeric reordering is deprecated. Combined edits use supported sequential writes.
- [Due forms](https://developer.todoist.com/api/v1/#tag/Due-dates): calendar, floating and fixed-zone distinctions. The API does not define DST gap/fold conversion policy; refusals are deliberate CLI policy.
- [Permanent completion](https://developer.todoist.com/api/v1/#tag/Sync/Tasks/Complete-task) and [move](https://developer.todoist.com/api/v1/#tag/Sync/Tasks/Move-a-task): native completion and explicit root destinations.
- [Acknowledgements](https://developer.todoist.com/api/v1/#tag/Sync/Overview/Response-Error): exact command success evidence independent of optional resources.
- [Task selection](https://developer.todoist.com/api/v1/#tag/Tasks/operation/get_tasks_api_v1_tasks_get) and [pagination](https://developer.todoist.com/api/v1/#tag/Pagination): active-parent selection, maximum page size 200, and concurrent pagination limits.
- [Doist CLI comparison](https://github.com/Doist/todoist-cli/blob/b0f9932c912dd74cdea6168c8b22a74ed04e94b7/src/lib/api/core.ts): Sync update/completion usage. It remains comparative evidence, not our wire specification.

Current code omits explicit empty edits, resolves update targets before validation, retries/redirects task writes, lacks ordinary-plan pending protection and full filtered preflight, ignores sort in some list branches, and treats missing page facts as empty/exhausted. These are the concrete implementation gaps. Baseline `go test ./...` passes on the fetched base before implementation.
