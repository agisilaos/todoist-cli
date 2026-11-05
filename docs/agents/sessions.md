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
from the clean session checkout. Move or remove ignored files first so a rebase
cannot overwrite them. The command takes a turn shared by this clone's worktrees,
fetches and rebases onto `origin/main`, and creates a disposable checkout of the
candidate. The Codex review and `make check` each get a fresh checkout of that
commit. Fetch and push use a separate Git directory and the same snapshot of
the user's transport settings. Configure one direct `remote.origin.url` or absolute local
path. The workflow supports Git's standard transports and requires a source
checkout without pre-push or pre-rebase hooks. Automatic rebase uses built-in
merge drivers and repository attributes. Stop background writers in the session
before rebasing. Use `TODOIST_CODEX` to select the review executable if needed.
Review results and full gate logs are printed as paths under
`~/Library/Logs/todoist-cli/sessions/` on macOS and
`${XDG_STATE_HOME:-~/.local/state}/todoist-cli/sessions/` on Linux.
Set `TODOIST_SESSION_LOG_DIR` to use a different directory outside the checkouts.

- A rebase conflict or newly empty commit leaves the session in the rebase.
  Inspect the cause, then resolve the stop or abort before running `land` again.
- Findings, missing review output, or a failed check stop the push. Read the
  reported log, fix the problem within scope, then land again. Resolve any
  review disagreement before publishing.
- If `main` moves during the attempt, land again to qualify against the new base.
  The turn covers worktrees in one clone. For concurrent pushes from other
  clones, the push requires both a fast-forward and the reviewed base SHA.
- If Git cannot confirm the push result, retrying checks whether `main` contains
  the candidate. If it does not, verify the commit was never published before
  clearing the pending publication record named by the command.
- If a rewrite removes the session's recorded base from `main`, inspect the
  history and move only the intended commits to a fresh session. Use `land` for
  upstream rebases so it can record the base it used.

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
from `origin/main` before unregistering the worktree and removing its branch.
It saves a Git bundle of the session tip and moves the checkout files and Git
metadata to a private sibling directory on the same volume. The command prints
the recovery path. Delete that directory once you no longer need it. If cleanup
stops before unregistering the
worktree, run `git worktree move <archive>/worktree <original-session-path>` from
the primary checkout. Inspect the restored files and staged changes before
retrying. Clear Git index flags that hide edits and move or remove ignored build
artifacts before ending. Use the primary checkout afterwards.
