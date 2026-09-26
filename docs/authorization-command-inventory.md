# Authorization command inventory

This inventory classifies the command surface covered by the confirmed authorization design. It describes effects; it does not grant permission to execute them.

Read-only means remote reads without Todoist mutations. Local-only commands may change local files or open a browser. Planning produces proposed changes. A dry run dispatches no Todoist mutation but may perform reads and documented local effects. OAuth credential acquisition is a separate authorization exchange, outside the Todoist resource-mutation gate.

| Commands, including aliases | Classification and qualifications |
| --- | --- |
| Bare invocation, `help [command]`, command help flags, `--version`, `help examples` | Local-only |
| `auth status`, `auth logout` | Local-only; logout removes a stored profile and does not revoke the remote token |
| `auth login`, `auth login --token-stdin` | Local credential replacement; OAuth variants also perform a remote authorization exchange |
| `task list/ls`, `task view/show` | Read-only |
| `task add/update/move/complete/reopen/delete/rm/del` | Remotely mutating, including bulk operations; dry-run previews supported |
| Top-level `add`, `inbox add` | Remotely mutating; dry-run previews supported |
| Bare `inbox`, `today`, `upcoming`, `completed` | Read-only |
| `project list/ls`, `project view/show`, `project collaborators` | Read-only |
| `project browse` | Read-only plus local browser opening; dry run suppresses browser opening |
| `project add/create/update/move/archive/unarchive/delete/rm/del` | Remotely mutating; dry-run previews supported |
| `filter list/ls`, `filter show` | Read-only |
| `filter add/update/delete/rm/del` | Remotely mutating; dry-run previews supported |
| `workspace list/ls` | Read-only through a Sync POST |
| `section list/ls`, `label list/ls`, `comment list/ls`, `reminder list/ls` | Read-only |
| `section`, `label`, `comment`, `reminder`: each `add/update/delete/rm/del` | Remotely mutating; dry-run previews supported |
| `notification list/ls`, `notification view` | Read-only |
| `notification accept/reject/read/unread`, including `read --all` | Remotely mutating; dry-run previews supported. `read` changes remote notification state |
| `activity`, bare `stats` | Read-only |
| `stats goals`, `stats vacation` | Remotely mutating; dry-run previews supported |
| `settings view` | Read-only through a Sync POST |
| `settings themes` | Local-only static catalog |
| `settings update` | Remotely mutating; dry-run previews supported |
| `view <Todoist URL>` | Read-only routing to existing resource and list commands |
| `agent plan` | Planning: remote reads, external planner execution, local plan persistence |
| `agent apply`, `agent run` | Remotely mutating when pending actions execute; dry-run previews supported |
| `agent status`, `agent examples` | Local-only |
| `agent planner`, top-level `planner` | Local-only; setting a planner changes local configuration |
| `agent schedule print` | Local-only output of cron/plist instructions for future `agent run`; installs and executes nothing |
| `completion bash/zsh/fish/powershell`, `completion install/uninstall [bash/zsh/fish/powershell]` | Local-only; installation and removal change local files |
| `doctor [--strict]` | Local inspection plus a read-only API probe |
| `schema [--name ...]` | Local-only |

The command dispatchers contain no current `review` command. The glossary's review plan remains a domain concept distinct from the current agent-plan file format.

## Agent actions and effects

All 20 currently accepted agent action kinds are Todoist mutations:

- `task_add`, `task_update`, `task_move`, `task_complete`, `task_reopen`, `task_delete`
- `project_add`, `project_update`, `project_archive`, `project_unarchive`, `project_delete`
- `section_add`, `section_update`, `section_delete`
- `label_add`, `label_update`, `label_delete`
- `comment_add`, `comment_update`, `comment_delete`

A current mixed-action plan contains different mutation kinds. Read/local agent actions are not currently supported. A replay skip performs no new mutation and is not a newly applied action.

`agent plan` saves its output and last-plan state. `agent run --out` can save a plan before a dry run or application. Apply/run dry runs do not record replay entries. Existing validation, agent policy, and confirmation requirements also apply to dry runs.

External planners execute independently as shell subprocesses and inherit the environment. The CLI's mutation guard does not sandbox them.

## Enforcement findings

REST traffic and Sync traffic converge on the guarded dispatch boundary in `internal/api/authorization.go`. Sync resource reads use POST, as do command-bearing Sync mutations. HTTP verb alone is insufficient to distinguish these operations.

Sources: `internal/cli/dispatch.go`, the individual command dispatchers in `internal/cli`, `internal/agent/validation.go`, `internal/app/agent/prepare.go`, `internal/cli/agent_apply.go`, `internal/cli/agent_schedule.go`, and `internal/api`.
