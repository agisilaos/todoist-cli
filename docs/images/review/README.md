# Daily review fixture results

This summary records a real `todoist review --filter "overdue | today"` process
built at commit `242b37c38592f6877566d3ec347cf531b3e6bdd2`, running in a macOS
pseudo-terminal.

The run used an isolated local Todoist fixture and dummy credentials; no real
account data or live Todoist mutations were involved. The fixture simulated
recurring-task advancement. This is not evidence of live service behavior.

The run exercised the initial disposition prompt, available task edits and move
destinations, confirmation of a due-date update, project move and completion,
and successful application with accounting for all four tasks.

The process exited 0. Fixture state confirmed task 2 moved to project 200 with
due date 2026-10-01, task 3 advanced to 2026-09-28, and tasks 1 and 4 were unchanged.
