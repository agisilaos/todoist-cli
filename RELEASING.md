# Releasing

This repo releases via a local script that handles:
- Updating `CHANGELOG.md` from the Unreleased section.
- Tagging and pushing `main` + the release tag.
- Building macOS release artifacts (`dist/`).
- Creating a GitHub release (if `gh` is installed).
- Updating `Formula/todoist-cli.rb` in `agisilaos/homebrew-tap`.

## Create a Release

1. Ensure `main` is up to date and the working tree is clean.
2. Run:

```bash
scripts/release.sh vX.Y.Z
```

That tags `HEAD`, pushes the tag, builds artifacts, and updates the Homebrew tap.

## Notes

- Requires macOS, `go`, `python3`, and `git`. Install `gh` if you want automated GitHub releases.
- The script expects `CHANGELOG.md` to include a `## [Unreleased]` section.
- Set `HOMEBREW_TAP_REPO` or `HOMEBREW_TAP_ORIGIN_URL` to override the tap repo or remote.
