# Active-task overview evidence

Captured from actual Todoist CLI processes built from implementation commit
`2ef7e1b72da4031b65aea6ce1915dc7cdd86d590` on macOS. The executable reports
`todoist dev (2ef7e1b) unreleased`.

Each command ran in a pseudo-terminal with the stated `COLUMNS` and PTY size.
Configuration and credentials were isolated; a local HTTP fixture supplied the
same synthetic overdue, due-today recurring, undated Inbox, and future Personal
tasks. No live Todoist account, real token, or mutation was involved.

Captured stdout and stderr retain the CLI text, with PTY CRLF normalized to LF.
Process output, commands, widths, request paths, exit status, and fixture UTC
date are in [captures.json](captures.json).

| Scenario | Command | Width |
| --- | --- | --- |
| All-project overview | `todoist task list --all-projects --all` | 40 |
| All-project overview | `todoist task list --all-projects --all` | 80 |
| Today view | `todoist today` | 120 |
| Partial page | `todoist task list --all-projects --limit 2` | 80 |

All commands also used an isolated `--config` path and `--no-input`, exited 0,
and preserved the expected fixture membership and order. Only the partial-page
case emitted stderr: its continuation hint is retained alongside stdout in the
captures.
The default overview requested projects as a collection and did not request
sections or per-task enrichment.

The captures establish actual PTY output. They do not verify a native terminal
application's font behavior or live Todoist API semantics. The implementation's
bounded consumer review, rendering and command tests, and CI evidence are
recorded in PR #16.
