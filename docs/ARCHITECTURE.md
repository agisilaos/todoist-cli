# Architecture

This project follows a layered design:

1. CLI adapter (`internal/cli`) parses flags, handles terminal UX, and renders output.
2. App services (`internal/app/*`) hold command use-case rules and payload/query planning.
3. Domain logic (`internal/agent`) holds planner types and validation rules.
4. API adapter (`internal/api`) performs Todoist HTTP calls.

The intent is to keep business/use-case logic in app/domain packages and keep `internal/cli` focused on transport concerns (arguments, prompts, output modes, exit codes).

## Current flow

```text
cmd/todoist
    |
    v
internal/cli (flags, help, UX, output formatting)
    |
    v
internal/app/* + internal/agent (validation, planning, payload/query builders)
    |
    v
internal/api (HTTP client, request/response types)
    |
    v
Todoist API v1
```

## Service coverage

- `internal/app/tasks`: list planning, single-task resolution, move/complete/delete guards, and task mutation payload builders.
- `internal/app/projects`: add/update payload validation and construction.
- `internal/app/filters`: add/update/delete validation and payload construction.
- `internal/app/comments`: comment list/add/update validation and payload construction.
- `internal/app/labels`: label list query planning and add/update payload validation.
- `internal/app/sections`: section list query planning and add/update payload validation.
- `internal/app/agent`: status payload composition and agent action-to-API request planning for `agent apply/run`.
- `internal/agent`: plan/action types, action validation, and summary derivation.
