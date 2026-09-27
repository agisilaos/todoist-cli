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

## Accounting

The `review_report` schema describes final output. Every selected task appears,
including tasks not reached before cancellation. Outcomes are kept, skipped,
proposed, applied, failed, partially_applied, or unattempted. `phase: applied` means
application was attempted, not that every task succeeded. Action outcomes identify
replayed successes, errors, and uncertain remote results. Counts cover task outcomes;
actions remain separately inspectable. Proposed dispositions are never applied results.
