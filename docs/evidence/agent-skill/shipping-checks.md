# Shipment checks — 2026-10-01

The feature commit is `ac2a59e`, based on main
`1ea11d5f35da4faf3ce221b034146264d9724b5e`. These checks cover the finished feature
tree, including the recovery repair below. A subsequent documentation commit
records evidence without changing the executable or bundled package.

## Recovery repair found during worktree review

Independent review found that a successful `update --backup` followed by failure
removing the installation lock lost the backup path in its error. The CLI returns
an error instead of the result in this case, so a human or machine client could
not locate the preserved customization from its diagnostic.

The reviewer reproduced the original defect using a temporary Go test overlay,
without editing repository files. Its captured failure was:

```text
--- FAIL: TestReviewBackupPathSurvivesLockCleanupFailure (0.01s)
    failure_test.go:266: backup undiscoverable in error:
    result.BackupPath="…/.agents/todoist-cli-backup-1753637044"
    error.BackupPath="" committed=true
FAIL github.com/agisilaos/todoist-cli/internal/skillinstall
exit: 1
```

The deferred cleanup now copies the result's backup path to the returned typed
error after lock release. The durable regression covers both committed and
rolled-back updates and reads the reported backup to verify exact customization
bytes. The reviewer confirmed the repair:

```text
go test ./internal/skillinstall -run '^TestBackupPathSurvivesLockCleanupFailure$' -count=1
ok github.com/agisilaos/todoist-cli/internal/skillinstall 0.392s
```

No additional actionable correctness or de-slop finding remained in the bounded
worktree review. That review included tracked and untracked feature files,
curated workflows, generated references, and consumer evidence. It does not
independently prove native loading, model behavior, crash recovery, or execution
on other operating systems.

## Fresh consumer attempt

A new reviewer, without reading source, internal tests, or prior review artifacts,
completed 36 public-route invocations. README's opening link led to the Agent
skills section, inventory, focused help, explicit Codex local installation,
inspection, update, conflict recovery, and uninstall. Claude Code global placement
was also checked in a scratch home with spaces. There was no initial friction
requiring guessing and no actionable finding.

The complete [consumer report](shipping-consumer/review.md) states versions,
knowledge limits, resulting-state checks, and untested behavior. Its
[transcript](shipping-consumer/consumer-transcript.txt) preserves commands,
stdout/stderr, exits, and filesystem observations. These calls did not use real
credentials, Todoist, external planners, or model inference. Existing local/global
native loading evidence remains applicable because the main entrypoint did not
change; the new cleanup repair is verified by failure injection, not by claiming
the consumer produced that filesystem failure.

## Validation

The post-repair feature tree passed:

- `go test ./...`.
- `make check`: formatting, vet, full coverage tests, module metadata,
  documentation/help/reference checks, and terminal authentication fixtures.
- `go test ./internal/skillinstall -race -count=1 -cover`: 88.6% coverage.
- `git diff --check`; no newly added deferred-work comments.
- PowerShell's completion smoke script on macOS, using an isolated portable
  PowerShell 7.6.6 runtime downloaded from the official release and checked against
  its published SHA-256 digest. The exact repository script was invoked with
  `pwsh -NoProfile -File scripts/test-powershell-completion.ps1` and exited 0.

PowerShell was unavailable during the original September 30 review; the October 1
check closes that local verification limit. It does not substitute for Linux CI.
No release matrix, live Todoist, real-model compliance, cloud distribution,
enterprise precedence, or power-loss verification is claimed.

## Integration with current main

Before publishing the conflict resolution, the candidate tree combined feature
head `8322db3` with main `b12585b696d7019915e2182685af80b3610d9989`. Main had gained
credential-profile commands and stricter OAuth onboarding. The resolution keeps
both command families, regenerates help and bundled references, and updates
curated profile/OAuth guidance to those merged contracts. The ownership ADR is
now 0007, preserving main's existing 0005 and 0006 decisions.

That candidate passed `go test ./...`, `make check` (including 30 curated-example
checks), PowerShell completion smoke, Linux/Windows amd64 cross-compilation, and
`go test ./internal/skillinstall -race -count=1 -cover` (89.1%). Its `SKILL.md`
main body still matches the native-probed package byte-for-byte. References have
changed; the earlier probes continue to establish entrypoint loading only.

A new reviewer completed 27 public-route invocations against this candidate,
without implementation, test, or earlier-evidence inspection. Codex local
installation, update, uninstall, edited-file recovery, exact backup preservation,
and JSON/NDJSON behavior passed; Claude Code global placement also passed. The
installed `profile list`, `profile current`, and `auth login` references matched
live help. No actionable
findings or initial public-path friction remained. [The bounded report and text
captures](current-main-consumer/README.md) preserve the pending-merge version,
reviewer harness corrections, state checks, and verification limits.

## Profile cleanup guidance repair

Final Spec review found that the bundled guidance suggested the selected
profile's `auth repair` after `profile remove NAME`. Removal retains selection
and can target another profile, so that wording could direct recovery to the
wrong credential. Guidance now uses `todoist --profile NAME auth repair`, keeps
the same explicit `--config`, and points to the error's `repair_command`.
The existing profile contract regression verifies that repair names the removed
profile and selected configuration; this repair changes instructions, not CLI
behavior. The earlier consumer attempt did not execute credential cleanup and
therefore did not establish this recovery behavior.

A [10-command lifecycle rerun](profile-recovery-rerun/README.md) confirmed the
corrected instructions were actually installed, relative references resolved,
repeated operations were deterministic, machine output parsed, and unrelated
files were retained. The reviewer reused prior workflow knowledge and did not
execute native credential cleanup. The corrected bundle also passed
`go test ./...`, `make check` (31 curated examples), and the two existing
named-profile/configuration recovery regressions.
