# Todoist CLI Roadmap

Status snapshot (2026-02-12):

- Shipped:
  - Quick add via Todoist quick endpoint with natural language parsing
  - NDJSON across task/project/section/label/comment list outputs
  - Task update by reference (id or text reference)
  - Output schema command for JSON/NDJSON contracts

Planned/considered next:

- [x] 1. OAuth device flow auth for headless/CI/agent environments
- [x] 2. Workspace and collaboration command surface (workspace list + collaborators)
- [x] 3. Agent progress JSONL stream for run/apply observability
- [x] 4. Stronger agent safety rails (policy checks and scoped permissions)
- [x] 5. Idempotency + replay safety for plan/apply reruns
- [x] 6. Better reference-resolution UX (interactive disambiguation + machine details)
- [x] 7. Bulk task operations with filter + dry-run/confirm flows
