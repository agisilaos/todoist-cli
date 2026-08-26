# Treat Replay Recording as Part of Action Success

An applied action requires both a successful Todoist mutation and a durable replay record. Replay-record failure terminates application even under `--on-error=continue`, trading availability for protection against silently repeating successful remote mutations; actions already recorded remain safe to skip on a later rerun.

## Consequences

Every successful Todoist mutation replaces the unbounded replay journal before success is emitted; replay skips, action failures, and duplicate records do not write it. Same-directory replacement prevents partially encoded journal content, while replacement visibility remains subject to the underlying OS and filesystem. An interruption after Todoist accepts a mutation but before recording completes can therefore leave an unrecorded mutation that a rerun duplicates. This favors recoverability over batched I/O, does not promise OS- or storage-power-loss durability, and assumes a single applying process, leaving cross-process locking and any safe retention policy as separate design problems.

Replay persistence remains in the existing CLI apply orchestrator because the journal is CLI-local execution state and the change intentionally leaves agent request planning in `internal/app/agent`; moving the entire apply use case across that boundary is a separate architectural refactor.
