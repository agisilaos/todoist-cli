# Maintainer sessions

## Kick off a chat

For a requested handoff, write one paste-ready brief: the outcome and scope,
issue or source links, decisions and evidence, how to verify completion, and
whether to stop at findings, implementation, a commit, or a push. Refresh the
issue and look for work already in progress before assigning it. Preparing a
brief does not open a chat.

Run `scripts/session start <name>` once and open the new chat in the worktree it
prints. If the chat already has that worktree, use it. The command fetches
`origin/main` and creates a sibling checkout on a local `fix/<name>` branch.
Use a short lowercase name with hyphens. Never work in the primary checkout on
behalf of a separate session.

Run `scripts/session overlaps <repo-relative-path>...` from the worktree root
before editing and again before landing if other sessions are active. It reports
matching files in other worktrees (committed, staged, unstaged, or untracked)
and paths changed on `origin/main` since this branch diverged. This is a snapshot;
coordinate overlapping edits and check the issue before duplicating work.

Use focused checks while working. Stage owned paths explicitly and leave
unrelated work in place. Each session has its own branch; all worktrees in the
clone share the stash.

## Land

Commit only when authorized. Once a push is authorized, run `scripts/session land`
from the clean session checkout. It takes a turn shared by this clone's worktrees,
fetches and rebases onto `origin/main`, has a separate Codex CLI review the exact
base and candidate commits, then runs `make check` once before a fast-forward push
to `origin/main`. Set `remote.origin.url` to a direct URL or absolute local
path and leave `remote.origin.pushurl` unset. Use `TODOIST_CODEX` to select the
review executable if needed.
Review results and full gate logs are printed as paths under
`~/Library/Logs/todoist-cli/sessions/` on macOS and
`${XDG_STATE_HOME:-~/.local/state}/todoist-cli/sessions/` on Linux.
Set `TODOIST_SESSION_LOG_DIR` to use a different directory outside the checkouts.

- A rebase conflict leaves the session in the rebase so you can resolve it or
  abort it. Inspect the resolution before running `land` again.
- Findings, missing review output, or a failed check stop the push. Read the
  reported log, fix the problem within scope, then land again. Resolve any
  review disagreement before publishing.
- If `main` moves during the attempt, land again to qualify against the new base.
  The turn covers worktrees in one clone. For concurrent pushes from other
  clones, the push requires both a fast-forward and the reviewed base SHA.
- If a rewrite removes the session's recorded base from `main`, inspect the
  history and move only the intended commits to a fresh session.

This is the maintainer's direct landing path. It runs the local gate itself;
it does not install Git hooks or change GitHub branch rules. If the repository
requires a PR, follow that requirement. Report the landed SHA, local gate
results, and any pending push CI. Release tags and publishing still follow the
[release runbook](../../RELEASING.md).

## Close

Record useful decisions and outstanding checks in the issue or relevant
documentation before closing the chat. When cleanup is requested, run
`scripts/session end` from the session checkout. It fetches and checks that the
worktree has no local changes or ignored files and its commits are reachable
from `origin/main` before removing the worktree and branch. Clear Git index
flags that hide edits and move or remove ignored build artifacts before ending.
Use the primary checkout afterwards.
