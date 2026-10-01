# Plans and recovery

## Discover and prepare

Inspect the configured planner and the executable's plan contracts:

```sh
todoist agent status --no-input --json
todoist agent planner --no-input --json
todoist schema --name plan --json
todoist schema --name planner_request --json
todoist agent plan --help
```

An external planner is a trusted program executed through `/bin/sh -c`. It
receives a JSON request on stdin and must emit plan JSON on stdout. It inherits
the process environment, which can include `TODOIST_TOKEN` and other secrets;
its stderr may appear in a failure diagnostic. Use a trusted command and a
deliberately scoped environment. The CLI's read-only guard and dry run do not
sandbox the planner's filesystem, subprocesses, or independent network activity.

Planner context includes projects, sections, labels, at most 50 matching active
tasks, and optional completed tasks. It is not a complete account snapshot and
does not prove absence beyond that cap. Context filters affect supplied context,
not authorization. Inspect intended IDs and scope with complete list commands
when the requested workflow depends on all tasks.

For a configured, trusted planner, create a plan in a private workspace and inspect
the saved JSON rather than approving only its summary:

```sh
todoist agent plan "Move the selected overdue tasks" --out plan.json --no-input --json
```

Planning writes the last plan beside the configuration and also writes `--out`
when requested. The JSON preview wraps the executable plan under `plan`; the saved
file is the plan itself. Keep the exact file reviewed for subsequent application.

## Confirm, preview, and apply

Review every action's type, target IDs, destinations, and fields against the user's
authorized intent. Current agent action kinds all mutate Todoist. Policy checks
can restrict plans but are not substitutes for authorization or inspection.

`confirm_token` is an acknowledgement value checked against the loaded plan. It is
not a signature or hash binding the action contents to what the user reviewed.
Preserve the reviewed file and account/config selection; any later edit requires
review again even if its token stays unchanged. Plans and replay records are scoped
to the configuration directory, not bound to a Todoist account, so switching
profiles is not an account-safe replay guarantee.

After inspecting the plan, use its token for a preview. Substitute the token
privately; the environment variable below is an example, not a CLI configuration
setting:

```sh
todoist agent apply --plan plan.json --confirm "$CONFIRM_TOKEN" --dry-run --no-input --json
```

A dry run dispatches no Todoist mutations, but may perform reads and documented
local writes. Planning during `agent run` can still execute the external planner
and save plans. A successful preview proves validation of that preview, not remote
permissions, Todoist's natural-language interpretation, or successful application.

Within the user's existing authorization, apply the same reviewed file:

```sh
todoist agent apply --plan plan.json --confirm "$CONFIRM_TOKEN" --no-input --json
```

The CLI checks all pending actions' authorization before the first mutation.
Authorization and replay-record failures stop application even with
`--on-error continue`. `--force` bypasses confirmation requirements; use it only
when the authorized workflow intentionally accepts that bypass, not as recovery
from ambiguous identity or blocked authorization. Empty plans can validate in a
dry run but do not establish an application.

## Replay and uncertain writes

An applied action requires both accepted remote mutation and stored replay record.
Recorded actions are skipped on rerunning the same plan with the same replay
context. JSON per-action results mark replay skips with `error: "skipped_replay"`;
inspect the complete results and exit status before describing the outcome.
`applied_at` and progress success appear only after the relevant success boundary.

For ordinary agent plans, a request may reach Todoist before its replay record is
written. Replay-record failure reports `remote_succeeded: true` in progress events
at stage `replay_record`; interruption can also leave an accepted mutation
unrecorded. Blind retries can duplicate these writes. Preserve the plan, journal,
and diagnostics, inspect the exact remote IDs through read commands, then reconcile
the outcome before choosing any further mutation. Direct resource writes also do
not gain agent replay protection merely because an agent called them.

Review plans have additional pending-write evidence and checkpoints: uncertain
outcomes block blind retries. Preserve that evidence and manually verify remote
state before beginning a fresh review. `review` is a terminal workflow; it rejects
piped input and `--no-input` before API access. Ordinary plans do not acquire this
review protection.

The replay journal is unbounded and assumes one applying process; do not run
concurrent applications against the same configuration. Replacement depends on
the OS and filesystem and does not promise durability across power loss. A
successful exit or later read cannot reconstruct missing execution evidence or
prove every intended field persisted. Report accepted operations, recorded replay,
observed state, and any remaining uncertainty separately.
