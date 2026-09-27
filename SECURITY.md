# Security Policy

If you discover a security issue, please email hello@agisilaos.com.

Please do not open public issues for security vulnerabilities.

## Credential storage

Official macOS builds store new profile tokens in Keychain using Security.framework.
File storage is an explicit portable alternative, never an automatic fallback from
native failure. Non-secret authorization evidence stays locally readable and is
selected together with its token. Invalid evidence never becomes permissive legacy
authorization. Environment tokens override profiles without inheriting evidence.

Keychain operations disable OS interaction and classify failures before rendering.
Tokens are passed directly to native APIs, never through subprocess arguments.
Interactive token entry disables terminal echo and restores terminal settings when
the read finishes, including read errors. Scripts can use `--token-stdin`.
`auth login --print-env` deliberately exports the newly acquired token; all ordinary
output, errors, logs and doctor reports exclude secrets.

Migration verifies the destination before removing the current plaintext token.
Recovery journals contain no tokens. Writers and explicit repair remove abandoned
regular files under the reserved `.todoist-credentials-stage-*` prefix and older
numeric `.credentials-*` staging names while holding the writer lock. Other files
and symlink targets are preserved. Cleanup errors distinguish completed changes
from operations needing recovery. Logout disables a profile before deleting its
native token. Neither operation remotely revokes a token. No CLI can erase copies
in filesystem snapshots or backups, or protect credentials from an actor with the
same user's access to the local configuration and native store.

Native profile references are scoped to configuration directory and profile.
Directory copies require a new login; they do not automatically share or delete
original entries. Unsupported/corrupt storage is reported rather than overwritten.
Preserve corrupt files for deliberate recovery. Use `auth repair` only for recognized
interrupted operations and pending cleanup.

## Isolated native tests

Ordinary CLI tests inject native adapters and use temporary configurations. The
optional native test uses a dedicated temporary keychain and checks that the
system's default keychain and search list remain unchanged:

```bash
TODOIST_TEST_KEYCHAIN=1 go test -tags credentialintegration ./internal/credentials -run TestIsolatedKeychain -count=1 -timeout 30s
```

It never uses real credential entries, alters default/search-list settings, or
intentionally invalidates the test keychain before a write. Some native APIs can
fall back to the default keychain if a supplied keychain becomes invalid, so the
private keychain remains alive until testing finishes. Unavailable-store and other
unsafe-to-induce failures use injected fakes. A native API change that prevents
isolation must fail the test rather than use the real store.

## Review recovery data

Review plan files and replay checkpoints contain private task fields, including
content and descriptions, but no tokens. Newly saved plans and replay journals
use mode 0600 in directories created with mode 0700. Explicit plan paths are never
overwritten. Recovery files are retained until deliberately removed by the user.
Deleting replay evidence can permit duplicate writes; do not remove it while a
plan is still needed. Only one applying process is supported.

Use the current binary, unchanged review plans, and the same configuration,
account, and API endpoint for retries. Older binaries do not enforce review
preconditions. Review metadata is not a permission grant or an account binding;
current credential authorization still governs every mutation. An uncertain
remote write blocks blind retry and requires inspection before a fresh review.
