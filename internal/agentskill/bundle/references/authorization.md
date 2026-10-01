# Credentials and authorization

## Select and inspect the credential

Use `--profile` or `TODOIST_PROFILE` to select a stored credential profile.
`TODOIST_TOKEN` overrides stored credentials, including the selected profile's
authorization metadata. Check the active source and authorization on the actual
invocation before relying on a stored read-only profile.

```sh
todoist --profile reader auth status --no-input --json
todoist --profile reader doctor --no-input --json
todoist schema --name authorization --json
todoist schema --name auth_status --json
```

Status reads local metadata without retrieving native secrets or testing remote
acceptance. `configured` means a credential is recorded, not that it is accessible
or authenticated. Doctor checks backend health and performs an API probe; a
successful probe establishes acceptance at that time, not scopes or future access.

Authentication, effective scopes, scope evidence, and write capability differ:

- OAuth records scopes from an explicit successful token response or, when
  omitted, the successful exchange's requested scopes. Requested scopes alone
  before an exchange do not establish a grant.
- Manual, environment, and legacy tokens have unknown scope evidence. The CLI
  permits attempted writes for compatibility while still reporting unknown;
  Todoist decides whether those requests succeed.
- A read-only credential blocks every Todoist mutation, including notification
  read state, settings, goals, and invitation responses. Reads, local operations,
  planning, and previews remain available. `--force` cannot bypass authorization.
- Present invalid or unsupported authorization metadata blocks authenticated
  operations. Inspect status and use documented login/logout repair paths rather
  than deleting evidence or treating corruption as unknown authorization.

Plans do not confer write permission. Application resolves the current credential
and authorization again; a read-only preview can succeed while application fails.

## Acquire and protect credentials

Credential acquisition changes local credential state and may involve browser or
human authorization. Choose the flow and profile deliberately. A read-only OAuth
example is:

```sh
todoist --profile reader auth login --oauth --read-only
```

New profiles default to native storage. macOS Keychain is the implemented native
backend; unsupported builds fail instead of silently choosing plaintext. Select
`--credential-store file` only when the user accepts portable plaintext storage.
Existing profiles retain their backend; migration and interrupted storage
transactions use documented `auth migrate` and `auth repair` commands. Live Todoist
device authorization support is unverified, and token refresh is not implemented.

For an already provided manual token, use the documented stdin route in a private
environment. Manual verification proves authentication only and leaves scopes
unknown. Keep tokens out of command arguments, transcripts, planner commands,
public artifacts, and diagnostics. `auth login --print-env` is an explicitly
secret-bearing export; capture it privately only when that export is requested.
Reusing an exported token as `TODOIST_TOKEN` loses the CLI's local scope evidence.

Logout affects the stored profile. It neither revokes Todoist's token nor removes
an overriding environment token. Keep credential files, plan files, progress logs,
replay records, and external backups private; they can contain sensitive account
data even when they contain no token.
