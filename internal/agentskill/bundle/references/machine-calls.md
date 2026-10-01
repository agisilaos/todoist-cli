# Machine calls

## Discover the supported surface

Use root help to discover command families, then focused help for exact operands
and flags. Global flags may appear before or after command names; `--` ends global
flag parsing. Help is available without credentials and before loading broken
configuration. The generated [command reference](commands.md) contains usage and
flag declarations; read the relevant section rather than loading the full file.

```sh
todoist task update --help
todoist schema --json
todoist schema --name task_list --json
todoist schema --name ids_only --json
```

Use supported commands from this executable. This package does not promise the
commands of the separate Doist `td` CLI or features described on unmerged branches.

## Parse outputs and errors

- `--json` emits the command's raw array or object, not a universal result
  envelope. Empty resource lists emit `[]`. A task list is an array; a task view
  is an object. Consult the relevant published schema when one exists.
- `--ndjson` emits one object per line for lists, or one record for supported
  single results. An empty list emits no records.
- `--ids-only` supports the lists named by its wire-format descriptor. It emits
  one opaque ID plus LF per result, with zero stdout bytes for empty results.
  Activity IDs identify events and collaborator IDs identify users. The mode
  conflicts with JSON, NDJSON, and plain output and rejects unsupported commands.
- `--plain` preserves each command's text contract; it is not a universal TSV
  schema. Task lists use TSV, while task detail remains labeled text. Prefer JSON
  when parsing fields.
- JSON errors go to stderr with `error` and `meta`, and classified failures may
  include `code` and `details`. `--quiet-json` compacts JSON errors; by itself it
  does not select JSON output. NDJSON and plain error conventions can remain text.
  Preserve stdout and stderr separately, especially with progress enabled.
- Exit codes are 0 success, 1 generic failure, 2 usage, 3 authentication or
  authorization, 4 not found, and 5 conflict. Partial application may return useful
  stdout plus a nonzero exit. Inspect per-action errors and replay skips too.

Resource machine payloads do not add the human view's enrichment or prove that
missing fields are false or empty. Numeric task priority remains the API scale
(4 highest); `p1` through `p4` inputs use Todoist's display scale (p1 highest).

## Select scope, pages, and identity

`task list` defaults to one page of active Inbox tasks. `--all-projects` changes
scope; `--all` fetches every page. `inbox`, `today`, and `upcoming` fetch all pages
of their selections. Other families have their own supported cursor, offset,
limit, and all-page flags; check focused help before assuming support.

JSON task lists are arrays without a cursor envelope. For complete task collection
use `--all`, or follow the command's continuation notice on stderr when doing
page-by-page retrieval. IDs-only continuation notices also stay on stderr and can
occur even when an individual page is empty.

```sh
todoist task list --all-projects --all --no-input --json
todoist task view id:123456 --no-input --json
todoist task complete --id 123456 --no-input --json
```

The example ID is illustrative; substitute an ID discovered from the intended
account and selection. IDs are opaque strings. Prefer explicit `id:` references
or a command's `--id` flag over title matching, and select explicit destination
IDs when supported. Fuzzy name matching is opt-in and does not authorize guessing
between multiple candidates.

`--no-input` suppresses prompts; it does not supply missing inputs or confirmation.
An ambiguous task reference still fails with exit 2 and empty stdout. The JSON
ambiguity error's `details.matches` contains titles without candidate IDs, so
recover by listing the intended scope with JSON, checking context and full IDs,
then inspecting the chosen task and deliberately retrying with its exact ID.
