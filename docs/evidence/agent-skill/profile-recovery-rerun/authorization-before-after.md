# Delivered removed-profile cleanup recovery

Before (original installed artifact; parent independently supplied the finding):

```text
Logout affects the stored profile. It neither revokes Todoist's token nor removes
an overriding environment token. `profile remove NAME` requires an explicit name
and is idempotent when absent; it retains defaults/selections and never activates
a fallback credential. Native cleanup failure leaves the profile disabled and
returns `CREDENTIAL_CLEANUP_PENDING` with `committed=true`. Repeat removal or use
the selected profile's `auth repair`, preserving the configuration selection.
Removal does not revoke the remote grant.

```

After (actual new isolated installation):

```text
Logout affects the stored profile. It neither revokes Todoist's token nor removes
an overriding environment token. `profile remove NAME` requires an explicit name
and is idempotent when absent; it retains defaults/selections and never activates
a fallback credential. Native cleanup failure leaves the profile disabled and
returns `CREDENTIAL_CLEANUP_PENDING` with `committed=true`. Repeat
`profile remove NAME` or run `todoist --profile NAME auth repair`, preserving the
same `--config` when supplied; the error's `repair_command` names that profile
and configuration. Removal can target a profile different from the current
selection, so plain `auth repair` may repair the wrong profile.
Removal does not revoke the remote grant.

```
