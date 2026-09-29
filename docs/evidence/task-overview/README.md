# Active-task overview screenshots

Captured from actual Todoist CLI processes built from implementation commit
`2ef7e1b72da4031b65aea6ce1915dc7cdd86d590` on macOS. The executable reports
`todoist dev (2ef7e1b) unreleased`.

Each command ran in a pseudo-terminal with the stated `COLUMNS` and PTY size.
Configuration and credentials were isolated; a local HTTP fixture supplied the
same synthetic overdue, due-today recurring, undated Inbox, and future Personal
tasks. No live Todoist account, real token, or mutation was involved.

These PNGs are **headless Chrome CLI screenshots of a read-only transcript
viewer**, not Terminal.app screenshots. The viewer displays the captured stdout
and stderr without editing their text (PTY CRLF is normalized to LF). Command,
stream, width, revision, and exit annotations are capture tooling, not CLI UI.
The browser used a temporary isolated profile. The underlying process output,
request paths, exit status, and fixture UTC date are in [captures.json](captures.json).

| Screenshot | Command | Width |
| --- | --- | --- |
| [All-project overview](overview-40.png) | `todoist task list --all-projects --all` | 40 |
| [All-project overview](overview-80.png) | `todoist task list --all-projects --all` | 80 |
| [Today view](today-120.png) | `todoist today` | 120 |
| [Partial page](pagination-80.png) | `todoist task list --all-projects --limit 2` | 80 |

All commands also used an isolated `--config` path and `--no-input`, exited 0,
and preserved the expected fixture membership and order. Only the partial-page
case emitted stderr: its continuation hint is shown in a separate labeled panel.
The default overview requested projects as a collection and did not request
sections or per-task enrichment.

The images establish presentation of actual PTY output through the transcript
viewer. They do not verify a native terminal application's font behavior or live
Todoist API semantics. The implementation's bounded consumer review, rendering
and command tests, and CI evidence are recorded in PR #16.
