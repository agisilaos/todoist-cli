# Task editing consumer review

Date: 2026-10-01. Source: `d7ef83a` plus the review corrections committed with
this report. Binary: a temporary `go build ./cmd/todoist` executable reporting
`todoist dev (local) unreleased`; docs: the matching README and focused help.
Environment: macOS arm64, Go 1.27.1, Python 3, noninteractive execution, isolated
configuration, synthetic credentials, and a local HTTP fixture. Ambient
`TODOIST_*` values were removed and commands ran from a temporary scratch directory.
The reviewer already knew the implementation; this limits claims of independent
discovery. This is fixture verification, **not live Todoist verification**.

## Public route and initial attempt

Read the README task/editing guidance, then `task add/view/reschedule/update/move/complete
--help`. Used exact IDs returned by creation and JSON with `--no-input`; selected
v2 explicitly for inspection. The initial transcript was saved before source
inspection or corrections.

The main lifecycle completed with exactly 12 writes. The first recovery attempt
then exposed an incorrect fixture expectation: ambiguous text was expected to
return exit 5. Actual execution correctly returned the established usage exit 2,
empty stdout, structured `ambiguous_match` details on stderr, and zero writes:

```text
$ todoist task move Consumer --clear-parent --no-input --json
exit=2 writes=0
matches: Consumer parent, Consumer sibling
```

The fixture assertion was corrected to the public contract. No CLI exit behavior
changed. Initial full transcript: `/tmp/todoist-task-editing-consumer-initial.md`.

## Findings and corrections

- **Both audiences, documentation judgment:** the earlier README command and
  presentation inventories still advertised only `due|priority` sorting for
  active tasks and omitted rescheduling, expansion, and permanent completion.
  Updated those inventories to agree with the editing examples and focused help.
- **Agent consumers, documentation judgment:** task-view help unconditionally
  described an ordinary task object/schema after introducing expanded views;
  README also promised JSON without envelopes. Qualified ordinary resources and
  named the separate expanded/fallback contracts. Task-data fidelity now lists
  rescheduling in its coverage and returned-resource declarations.
- **Review harness:** corrected the ambiguity exit assertion above. This was
  observed execution friction in the harness, not a product defect.

No additional lifecycle or relevant recovery defect was observed. Interactive
usability was not tested. The documentation findings are judgments from the
public route, not claims about first-time user behavior.

## Corrected execution and state checks

| Operation | State verified | Writes |
| --- | --- | ---: |
| Create parent, recurring child, grandchild, sibling | Exact stdin description, parent links, recurrence | 4 |
| Inspect and expand parent with v2/order sorting | Two direct-child pages, both children, no grandchild | 0 |
| Reschedule recurring child | New date, original recurrence/expression/language/time character | 1 |
| Clear non-due fields | Empty description/labels; null deadline/assignee | 1 |
| Clear parent and section | Current-project root | 1 |
| Enable reference/order zero, then disable reference | Single prefix added/removed; explicit zero preserved | 2 |
| Permanently complete recurring child | Completed history retains recurrence; descendant completed | 1 |
| Create separate due-clearing task and clear due | Null due on the separate task | 2 |
| Relevant recovery | Accepted fallback, partial rejection, uncertain acknowledgement | 4 |

The corrected lifecycle passed: **12 lifecycle writes + 4 recovery writes = 16**.
Invalid setter/clearer input, ambiguity, missing-task reads, unsupported time
conversion, established no-ops, and dry runs dispatched zero writes.

Accepted update with optional task data unavailable returned exit 0 and:

```json
{"id":"scratch5","status":"accepted","operation":"task_update","result_available":false}
```

Combined due clearing and a rejected remaining edit returned exit 1, a partial
report on stdout with `accepted` then `rejected` steps and their request IDs, and
a structured error on stderr. It dispatched exactly two writes and no rollback.
An uncertain permanent-completion acknowledgement returned exit 1 and empty
stdout after one dispatch. Read-only completed-history inspection established
the fixture's resulting state without repeating the mutation. All successful
stderr streams were empty; failure stderr was parseable JSON.

Reproduce with a temporary binary and transcript:

```bash
go build -o /tmp/todoist-task-editing ./cmd/todoist
python3 scripts/test-task-editing-lifecycle.py \
  --binary /tmp/todoist-task-editing --log /tmp/todoist-task-editing-consumer.md
```

Corrected transcript: `/tmp/todoist-task-editing-consumer-corrected.md`. These
scratch paths are local evidence; the runnable fixture and this bounded report
are retained in the repository. This review covers only this lifecycle and its
recovery paths, not all commands or live service/DST behavior. Unit/API fixture
tests separately cover timezones, DST, pagination failures, batch accounting,
read-only authorization, malformed acknowledgements, and compatible output modes.
