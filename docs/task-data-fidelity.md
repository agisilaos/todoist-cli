# Task-data contract

`--task-output-version 2` selects faithful task-resource JSON/NDJSON over Todoist
API v1. The default remains legacy version 1. This is a CLI output version, not a
Todoist API version. The [compatibility decision](adr/0008-preserve-legacy-task-output-with-a-faithful-projection.md)
preserves existing clients. `--full` affects human and legacy plain detail;
JSON/NDJSON payloads are unchanged.

```sh
todoist task view id:task-id --no-input --json --task-output-version 2
todoist task list --all-projects --all --no-input --ndjson --task-output-version=2
todoist schema --name task_item_v2
todoist schema --name task_list_v2
```

## Selection and coverage

The selector accepts exactly `1` or `2`, before or after the command. There is no
config or environment override. Explicit selection requires `--json` or `--ndjson`.
Valid help, including `--help=true` and `-h=true`, remains available without
configuration or credentials; invalid values or unsupported uses return usage
exit 2 before side effects.

Supported commands are `task list`/`ls`, `task view`/`show`, `task add`,
`task update`, `task reschedule`, `add`, `inbox add`, bare `inbox`, `today`, `upcoming`, `completed`,
and `filter show`. Task/filter/label entity URLs and Inbox/Today/Upcoming/Completed
page URLs through `view` use the same representation. Acknowledgements such as
move/complete/reopen/delete, dry runs, and agent/review contracts reject selection.

Ordinary JSON views remain objects; lists and returned add/update/reschedule resources remain arrays. Expanded views and accepted-write fallbacks use the separate contracts below.
NDJSON emits the same object once per line. Pagination and diagnostics go only
to stderr; `--all`, selection, order, priorities, and IDs-only behavior are unchanged.
`--full` never changes either machine representation. Numeric machine priority
retains API numbering, with 4 highest; human overview/detail retain Todoist P1 highest.

## Presence and values

Every supported API fact is optional. An absent field is omitted and remains
unknown. Explicit `null` remains null, including defensively returned null where
upstream normally promises a value. False, zero, empty strings, empty arrays, and
empty objects remain exact. Typed values do not imply that missing sibling facts
were returned. Unknown properties are excluded; v2's allowlist and semantics are
fixed, and changing them requires a new output version.

New integer facts accept whole numeric values such as `1.0` or `6e1` under JSON
Schema's integer semantics. Their returned numeric representation is preserved
without floating-point rounding. Existing strict decoding of legacy fields such
as `priority` is unchanged.

Human detail distinguishes `Not returned`, known absence (such as `No deadline`
or `Unassigned`), and unknown explicit null for fields where null does not
establish absence. New malformed optional facts show `Unavailable (invalid
returned type)`; v2 omits only those facts and writes nonfatal field/type
diagnostics to stderr. Valid siblings survive. Existing strict field decoding
is retained, and legacy streams receive no new diagnostics. No response value
is reconstructed from mutation input or previous responses.

## Supported fields

The names below are returned wire names. IDs remain opaque strings. Each field
and nested member also permits null under the CLI's defensive fidelity policy.

| Type | Fields |
| --- | --- |
| String | `user_id`, `id`, `project_id`, `section_id`, `parent_id`, `added_by_uid`, `assigned_by_uid`, `responsible_uid`, `added_at`, `completed_at`, `completed_by_uid`, `updated_at`, `order_key`, `content`, `description` |
| Integer | `priority`, `child_order`, `note_count`, `day_order`, `completed_count`, `postponed_count` |
| Boolean | `is_collapsed`, `checked`, `is_deleted`, advisory `is_uncompletable` |
| Array of strings | `labels` |
| Object | `due`, `deadline`, `duration` |

Nested fields:

- `due`: strings `date`, `string`, `lang`, `timezone`, compatibility `datetime`;
  boolean `is_recurring`.
- `deadline`: strings `date`, `lang`.
- `duration`: integer `amount`; string `unit`.

`responsible_uid` is the assignee; mutation `assignee_id` is a different request
name. `child_order` is sibling position, including zero. `order_key` orders
lexicographically among tasks with the same project, section, and parent; null
means the task has not migrated to that ordering representation. Day order is separate.

Due data retains the original date/time, offset, timezone, language, expression,
and explicit recurrence flag without timezone conversion. Current API v1 `date`
can contain a calendar date, floating datetime, or fixed UTC timestamp;
`datetime` is retained only when actually returned for compatibility. Null due
means no due date; absent due means unknown. Null timezone identifies date-only
or floating due data. No recurrence is inferred from the expression.
Human detail shows distinct timestamp values when both `date` and `datetime`
are returned; a floating value without a returned timezone remains unknown.

Deadline is date-only and separate from due recurrence; its returned language is
output metadata. Duration normally has a positive amount and unit `minute` or
`day`, but output preserves an actually returned integer zero or other typed
value without coercion. `note_count` is deprecated and currently only returns
zero, so it does not establish a live comment count. Completion counters and
timestamps do not establish current completion state; `checked` remains independent.

These definitions were verified against the [official Todoist API v1 documentation](https://developer.todoist.com/api/v1/#tag/Tasks)
on 2026-09-30. Completed endpoints return the same task fields in `items`, while
active/filtered endpoints use `results`; the decoder handles those collections
without changing client pagination.

## Reference items

V2 adds a separate derived property only when returned string content is available:

```json
"reference_item": {"is_reference": true, "source": "content_prefix"}
```

The exact `* ` title prefix establishes this classification, including false
when returned content lacks the prefix. Missing/null content leaves it unknown.
An explicitly returned `is_uncompletable` flag is retained separately as advisory
data: it is absent from the current official response specification. If it
disagrees with title syntax, neither value overrides the other. A reference item
can still be completed through its parent, so this classification is independent
of `checked`. See [official reference-item guidance](https://www.todoist.com/help/todoist/features/create-an-uncompletable-task-in-todoist-QxQosZuF).

Doist's CLI is comparative evidence, not our schema: its [pinned SDK transformation](https://github.com/Doist/todoist-sdk-typescript/blob/19798a373802c3d45cc5f753cc01d68c8780edf4/src/types/tasks/types.ts#L71)
derives reference status from content. We preserve source facts and label that
derivation explicitly instead of treating the SDK property as a wire field.

## Recovering unavailable facts

If a collection response omits information needed for a decision, inspect the
same exact ID with `task view ... --json --task-output-version 2`. A fuller response
can establish the fact; it may still be absent, null, or malformed. Do not interpret
absence as false/zero/unassigned or automatically retry a successful mutation
because its resource response lacks information. Use `--all` or the stderr cursor
notice for incomplete collections. Missing tasks retain exit 4; missing credentials
retain exit 3, with empty stdout on errors.

## Editing and expanded output

[Task editing](task-editing-design.md) reuses returned task facts. Rescheduling
refuses unknown recurrence/time character; hierarchy clearing walks exact ancestry.
The frozen task_item_v2 projection is unchanged. `--include-children` wraps v1/v2
resources in a separately schematized parent/children/children_complete envelope,
with every direct active child page fetched before output. No flag means no child
requests. Failed or contradictory expansion has empty stdout.

Accepted add/update/reschedule writes may lack optional resource data. In that
case `task_write_ack` carries acceptance and result_available:false; it contains
no invented task facts. JSON/NDJSON result unions are task_write_result[_v2] and
task_write_record[_v2]. Two-step edits may produce task_partial_edit with a nonzero
exit; task_unchanged and task_batch are also separate contracts. Inspect before
resubmitting an accepted or uncertain write. Existing --full and IDs-only rules
remain unchanged.
