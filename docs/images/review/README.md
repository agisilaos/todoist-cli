# Daily review screenshots

Captured from a real `todoist review --filter "overdue | today"` process built at
commit `242b37c38592f6877566d3ec347cf531b3e6bdd2`, running in a macOS pseudo-terminal.
The browser view displays the live, unedited terminal output and forwards input
to the process. Its frame and input box are capture tooling, not CLI features.

The run used an isolated local Todoist fixture and dummy credentials; no real
account data or live Todoist mutations were involved. The fixture simulated
recurring-task advancement. This is not evidence of live service behavior.

- `01-disposition.png`: initial task and disposition prompt.
- `02-edit-fields.png`: available task edits and move destinations.
- `03-confirmation.png`: proposed due-date update, project move, and completion.
- `04-result.png`: successful application and accounting for all four tasks.

The process exited 0. Fixture state confirmed task 2 moved to project 200 with
due date 2026-10-01, task 3 advanced to 2026-09-28, and tasks 1 and 4 were unchanged.
