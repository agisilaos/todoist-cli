# Daily review

Status: agreed design, implemented in this working tree. This does not imply a release.

`review` is a forward-only human workflow over the existing review set,
disposition, review plan, and applied-action concepts. It uses the ordinary
agent action format and application engine. It does not invoke an external
planner. The only interactive persistence is the finished plan, not a session.

## Selection and decisions

The default Todoist filter is `overdue | today`, across projects. Explicit filters
are sent literally; invalid queries fail without falling back to text search.
Every page is fetched before prompting, repeated cursors fail, and duplicate IDs
are kept once. Selected snapshots are frozen and sorted by due instant, then
higher API priority, then ID. All-day and floating times are ordered in the
terminal's local timezone; fixed times retain their offsets. Undated tasks sort
last. Pagination is not a remotely atomic snapshot and can miss concurrent changes.

Each task shows progress, full content/description, project and section IDs with
names when available, parent, labels, priority, assignee ID, due/recurrence/timezone,
deadline, and duration. Comments and history are not fetched. JSON-escaped text
prevents task content from being interpreted as terminal control sequences.

Keep means intentionally unchanged; skip means deferred. Neither emits an action.
Change offers content, description, labels (comma separated), priority (`p1`–`p4`
or API 1–4), due string/date/datetime/language, duration (`N minute` or `N day`),
deadline, assignee, and movement. Edits produce one `task_update`, then optionally
one `task_move`. Project/section/parent are alternative move destinations; selecting
another replaces the previous move. Project/section names must resolve uniquely before the final preview; explicit
`id:<id>` references remain available. Unknown or ambiguous names repeat the edit
prompt. References resolve before the final preview.
There is no new field-clearing contract. Empty values leave the field unchanged.
Completion is a separate disposition. It advances a recurring occurrence; ordinary
completion also completes subtasks, as the existing task completion endpoint does.

Dates use `YYYY-MM-DD`; explicit datetimes require RFC3339 with an offset.
Natural-language due values are sent verbatim, and the preview does not pretend
to know their eventual interpretation. Changing a recurring due value may replace
recurrence. Date-only, floating local, and timezone-qualified values remain distinct.

## Confirmation, output, and authorization

The preview lists original task fields and each proposed action. Only `yes` at the
final prompt applies. Empty/invalid input repeats the prompt. `--force` is rejected.
`--dry-run` and read-only credentials stop at preview; both can use `--out`.
Policies apply to review actions just as they apply to ordinary agent plans.

`--no-input` and non-TTY stdin fail with usage exit 2 before API reads. Structured
output does not disable interactive input: `--json`/`--ndjson` produce a single
final report on stdout; prompts and previews use stderr. `--plain` uses the human
report with tab-separated task rows; it is not a new resource-list format.
`--ids-only` remains unsupported. Empty selection is success with zero tasks.

Explicit cancel/no and Ctrl-C before application exit successfully; unexpected EOF
is failure. No remote mutation occurs before final confirmation. Cancellation does
not automatically save an unfinished session. An explicit `--out` saves the
finished preview even if confirmation is subsequently declined. During application,
Ctrl-C cancels HTTP work and stops further dispatch; an in-flight write can remain
uncertain. Known outcomes are reported, with nonzero status on application failure.

## Plan and application contract

An optional `review` member extends version-1 agent plans. Its own version is 1;
it stores filter, task snapshots, dispositions, and zero-based action indices.
Every action must belong to exactly one task. Existing plans without this member
keep their contracts. Keep/skip-only review plans can be applied as successful
no-ops, unlike ordinary empty agent plans. `agent apply` and `agent run` both honor
review metadata and return the review report for actual application. Dry-run agent
output retains its existing plan-preview envelope. Review plans require fail-fast
application; `--on-error=continue` is rejected for application.

A finished plan is written before the first mutation under
`<config-directory>/review-plans/<confirmation-token>.json`, unless `--out` supplies
a path. `agent run --out` also refuses to overwrite review-plan exports. New files use mode 0600 and never overwrite an existing path. Reserved credential,
configuration, replay, policy, and last-plan paths cannot be used as review output. Failure to
save blocks mutation. No automatic cleanup removes recovery evidence. Plans contain
private task data, not credentials. Use the same configuration, account, and API
endpoint when retrying. Do not edit recovery plans or apply them with older binaries,
which do not understand review metadata. This is not account-bound authorization.

Before dispatch, every pending task is re-read and compared with its original or
checkpointed reviewed fields. Changes or missing tasks stop application; no force
override exists. Each pending action rechecks its task immediately before dispatch.
This cannot eliminate a race between a read and the remote mutation.

A review checkpoint lives in an optional `reviews` map in the existing replay
journal. Before dispatch it is marked pending and persisted. After a successful
update followed by a move, the mutation response supplies the post-update snapshot;
a later independent read is not adopted as an approved state. The checkpoint and
action replay marker are published together before reporting an applied action.
A missing mutation-response snapshot or persistence failure leaves pending evidence
and terminates application. The remaining move cannot be retried blindly.

Definite 4xx rejections, except 408, clear pending evidence and can be retried with
the unchanged plan. Transport errors, 408, server failures, and interruption retain
uncertain evidence. Inspect Todoist and start a fresh review after reconciling an
uncertain outcome; the CLI offers no override that blindly repeats it. Recorded
successes are skipped on replay, and a partial task's remaining action compares
against its post-update checkpoint. Preflight failure still reports prior successes.

ADR-0002 continues to govern success, persistence limitations, unbounded journal
retention, and the single-applying-process assumption. No transactional application,
atomic remote conflict check, account-scoped replay, or exactly-once write guarantee
is introduced. Native credentials and cross-platform expansion are outside scope.

## Persistence sequence for maintainers

The CLI apply loop owns replay skipping, dispatch events, error policy, and success
reporting. Ordinary task actions persist pending evidence before dispatch and bind
mutation, failure cleanup, and replay recording to the prepared action. Ordinary
non-task actions retain mutation and replay recording without pending evidence.
Review preparation returns a `preparedAction` only after revalidating the task and
persisting pending evidence. Its operations hold one action's checkpoint and
mutation response; the replay store has no mutable current-action index or response.

| Boundary | Persisted evidence and next step |
| --- | --- |
| Replay key already present | Skip before preparation; no mutation or journal write. |
| Review revalidation or pending save fails | Stop before mutation execution; no applied-action result. |
| Review preparation succeeds | Pending evidence exists before request construction and dispatch. Interruption can therefore block retry even if no request was sent. |
| Final request result is a definite rejection | Clear pending evidence in a journal replacement. If replacement fails, retain pending evidence and stop. |
| Final request result is uncertain | Retain pending evidence and stop; do not redispatch from this plan. |
| Mutation succeeds | Record the replay key and clear pending evidence in the same replacement; review actions also include any required response checkpoint. |
| Required response snapshot is missing or recording fails | Stop without reporting an applied action. Review pending evidence remains. |
| Recording succeeds | Report the applied action. Interruption before reporting is safe to replay because the record is already installed. |

`fileReplayStore.updateJournal` owns candidate construction, persistence, and
in-memory publication for both ordinary and review writes. It copies the journal
maps; callers replace checkpoint values and treat snapshot maps as immutable.
Only successful persistence installs the candidate in memory. The file writer,
JSON fields, key derivation, corruption handling, and durability limits are unchanged.
Task writes dispatch once without automatic retries or redirects under
[ADR-0009](adr/0009-dispatch-task-writes-once-and-retain-pending-evidence.md).
Prerequisite reads and eligible non-task requests retain bounded retries;
classification above applies to the result returned to the apply loop.

## Accounting

The `review_report` schema describes final output. Every selected task appears,
including tasks not reached before cancellation. Outcomes are kept, skipped,
proposed, applied, failed, partially_applied, or unattempted. `phase: applied` means
application was attempted, not that every task succeeded. Action outcomes identify
replayed successes, errors, and uncertain remote results. Counts cover task outcomes;
actions remain separately inspectable. Proposed dispositions are never applied results.

## Human recovery guidance

The human report uses existing `remote_outcome_uncertain` evidence to recommend
read-only exact-ID inspection. Keep the saved plan and replay journal, compare
observed fields against intended changes and reported applied actions, and check
Todoist completed history/activity when completion cannot be established by an
active task lookup. Reconcile manually before a fresh review; unresolved outcomes
are a reason to stop, not to resubmit. The CLI has no automatic reconciliation or
pending-evidence override. A hard kill cannot print a final diagnostic; preserve
the saved plan and inspect remotely, and consult this workflow before further
application. See [errors and recovery](error-recovery.md).

These rendering additions leave explicit machine modes, the report schema,
persistence sequencing, and ADR-0002 unchanged.
