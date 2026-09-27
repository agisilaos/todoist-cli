# Treat Replay Recording as Part of Action Success

An applied action requires both a successful Todoist mutation and a durable replay record. Replay-record failure terminates application even under `--on-error=continue`, trading availability for protection against silently repeating successful remote mutations; actions already recorded remain safe to skip on a later rerun.

## Consequences

Every successful Todoist mutation replaces the unbounded replay journal before success is emitted. For ordinary agent plans, replay skips, action failures, and duplicate records do not write it. Same-directory replacement prevents partially encoded journal content, while replacement visibility remains subject to the underlying OS and filesystem. An interruption after Todoist accepts a mutation but before recording completes can therefore leave an unrecorded mutation that a rerun duplicates. This favors recoverability over batched I/O, does not promise OS- or storage-power-loss durability, and assumes a single applying process, leaving cross-process locking and any safe retention policy as separate design problems.

Replay persistence remains in the existing CLI apply orchestrator because the journal is CLI-local execution state and the change intentionally leaves agent request planning in `internal/app/agent`; moving the entire apply use case across that boundary is a separate architectural refactor.

## Review-plan pending writes

Daily review adds pending-write and task-checkpoint metadata to the same journal.
It records pending evidence before dispatch and clears it after a definite rejected
request; these local writes are not applied-action records. A successful mutation
publishes its replay record and any required post-update checkpoint together before
success is emitted. Interruption or an uncertain failure retains pending evidence
and blocks blind retry, choosing explicit recovery over automatically repeating a
possibly successful mutation. See [daily review](../review-design.md).

This extends the ordinary-plan journal-write policy above without changing the
applied-action success requirement, single-applying-process assumption, retention,
or durability limits. Replay skips still perform no journal writes.
