# Contributing

Thanks for your interest in contributing.

## Quick Start

1. Fork and clone the repo.
2. Install Go 1.22+.
3. Run tests:
   ```bash
   go test ./...
   ```
4. Format code:
   ```bash
   gofmt -w $(rg --files -g '*.go')
   ```
5. Check documentation (requires Python 3):
   ```bash
   make docs-check
   ```
6. Run coverage guardrails:
   ```bash
   make coverage-check
   ```

## Guidelines

- Keep changes focused and add tests when possible.
- Follow the CLI UX conventions in `README.md`.
- When changing commands, flags, output, or configuration, update the affected README/spec sections and regenerate help with `scripts/update-help.sh`. See [documentation maintenance](docs/README.md#keeping-docs-in-sync).
- Open a discussion before large or breaking changes.

## Submitting a PR

- Describe the problem and the solution.
- Include steps to validate.
- Ensure CI passes.

## Releases

See `RELEASING.md` for the release script and checklist.
