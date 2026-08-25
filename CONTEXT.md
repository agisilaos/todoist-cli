# Todoist CLI

A terminal companion for Todoist that gives people fast interactive workflows over a stable contract for scripts and agents.

## Language

**Terminal companion**:
A human-first interface for Todoist workflows where the terminal offers a meaningful advantage, while leaving visual planning to Todoist's graphical applications.
_Avoid_: Terminal clone, Todoist replacement

**Machine client**:
A script, agent, or other non-interactive caller that depends on explicit inputs and stable output contracts.
_Avoid_: Bot, automation user

**Machine output contract**:
The documented structured output and error behavior that machine clients can rely on across compatible releases.
_Avoid_: JSON mode, agent output

**Review set**:
The explicit collection of tasks selected at the start of a review and accounted for in its final summary.
_Avoid_: Today view, batch

**Disposition**:
The deliberate outcome assigned to a task during a review: kept, changed, completed, or skipped.
_Avoid_: Action, status

**Applied action**:
An agent-plan action whose Todoist mutation succeeded and whose replay record was stored. Until both occur, the CLI does not report the action as successful.
_Avoid_: Completed action, successful request

**Replay record**:
Durable evidence that a specific action from a confirmed agent plan has already changed Todoist, preventing the same action from being applied again.
_Avoid_: Journal entry, applied marker
