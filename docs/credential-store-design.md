# Credential storage

Status: accepted. The user confirmed the complete design and all three test boundaries. PR #4 landed in main at 2879104; implementation and review use that fixed point.

## Accepted foundations

- A credential store is the persistence boundary for credential profiles. A credential backend is a storage implementation behind that boundary. These are architectural terms; authorization metadata is domain language defined in CONTEXT.md.
- Keep native tokens in the native store and non-secret authorization metadata plus a reference to the matching token in a local profile record. Metadata inspection must not require secret retrieval. Replacement preserves the pairing through the commit and recovery protocol below.
- Implement macOS Keychain first, with portable file storage available on macOS, Linux, and Windows in accordance with ADR-0001.
- Require native storage by default for new profiles. File storage requires explicit selection. Existing file profiles retain their backend until migration. A native-store failure never causes automatic plaintext fallback.
- Preserve configuration-directory plus profile isolation. Configuration files within one directory continue to share credentials. Profile names alone must not identify globally shared native entries.

## Existing authorization contract to preserve

Reuse the versioned authorization representation from docs/authorization-design.md. Missing metadata retains legacy unknown semantics; invalid or unsupported metadata must not become permissive unknown authorization. Environment tokens override stored profiles without inheriting their metadata. Successful replacement selects a new token and its associated metadata together; failed persistence preserves the prior usable record. Preserve unrelated profiles and unknown fields.

## Accepted adapter and selection policy

- Use direct Security.framework access through a cgo adapter. Update macOS release builds to include the adapter. Other platforms and builds without cgo compile with native storage unavailable and explicit file storage available.
- Native credential operations display no OS dialogs, even for interactive callers. Users unlock or adjust Keychain externally and retry. Isolate native interaction control and verify its behavior safely before claiming the contract is met.
- The profile store owns complete-profile load, replacement, deletion, metadata inspection, enumeration, health reporting, and consistency/recovery. Inject a smaller native adapter for secret read/write/delete and limited availability checks. Selection is outside CLI workflows.
- Enumerate profiles from local records, not a scan of the user's Keychain. Help and local commands retrieve no secrets. Metadata inspection and basic diagnostics distinguish unchecked token accessibility from verified accessibility.
- Offer --credential-store=native|file for login and migration, plus a user-config default for new profiles; the built-in default is native. Project .todoist.json cannot affect this choice. Existing profiles load from their recorded backend; changing a default does not move them. Environment tokens bypass native access and retain existing precedence.

## Accepted identity and migration policy

- Scope native entries to the canonical configuration-directory path, resolving symlinks, and credential profile. Symlink aliases share a namespace; distinct directories do not. Use service io.github.agisilaos.todoist-cli.credentials.v1 and account identifiers made from directory/profile hashes plus a random entry generation. Validate a stored reference's namespace against the current canonical directory before native access. This prevents accidental cross-directory use; it is not an access-control boundary against a local actor able to edit state.
- Moved or copied directories require a new login in this version. A dedicated relocation command is deferred. A new login establishes the new namespace without reading or deleting native entries owned by the old namespace.
- Migration is explicit and per profile: auth migrate --credential-store=native. Ordinary reads never migrate profiles.
- Write and read back the destination native entry before atomically changing the local profile record to reference it and removing its plaintext token. Preserve its authorization metadata and unrelated profiles. Use the commit and recovery protocol below.
- Do not retain plaintext compatibility copies. Older binaries cannot use migrated profiles. An explicit reverse migration to file storage is required for that compatibility.

## Profile-store contract and storage format

CLI workflows depend on a platform-neutral profile store rather than credential JSON. The public operations are complete-credential load, save/replace, delete, metadata inspection, profile enumeration, and availability/health probing. Migration and repair use the same store's transaction machinery. Keep the injected native secret adapter limited to read, write, delete, and availability; it neither interprets authorization metadata nor selects a backend.

Keep the existing profiles mapping in credentials.json and reuse the authorization object unchanged. A per-profile versioned storage descriptor records the concrete backend and, for native profiles, the namespace and native-entry reference. Its initial version is 1, independent of authorization metadata version 1. Absence of a descriptor is the legacy file case, not permission to ignore a present invalid or unsupported descriptor. Native records contain no plaintext token. File records retain the existing token representation. Unknown fields on unrelated profiles survive updates; migration preserves the selected profile's authorization data without inferring or weakening it.

Recognized JSON field names retain Go's case-insensitive decoding behavior, including Unicode case folds. Writes use canonical field names and discard their alternate spellings, so migration, replacement, and logout cannot preserve stale credential copies as unknown fields. Profile names and genuinely unknown fields remain unchanged.

The selector native resolves to concrete backend keychain on supported macOS builds; recorded backend identity remains keychain rather than changing meaning across platforms. Metadata inspection and enumeration return no tokens or token-derived fingerprints. Enumeration is a store capability; a new list command is not required by this feature.

## Replacement, migration, commit, and recovery

- Existing-profile login replaces within its recorded backend. A conflicting --credential-store flag is a usage error directing the caller to auth migrate. User-config defaults affect only new profiles. The moved-directory new-login case establishes a new namespace, without acting on old-namespace entries.
- Serialize writers per configuration directory using a bounded wait; report a stable busy error when another writer retains the lock. Hold no credential-file lock while conducting the OAuth/browser flow. Re-read state under the lock before changing it. Reads observe one complete committed file state.
- Before creating native entries, durably record the transaction's identifiers and intended transition, without tokens. Use a fresh native-entry generation, never overwrite the active native secret in place. Read back the destination and verify it in memory before publishing it.
- Atomically replace the local profile record to select the new token reference and its associated authorization metadata together. For file-to-native migration, that same replacement removes the plaintext token. A failed operation before publication preserves the prior active profile and attempts to remove any newly staged native entry. A failed cleanup remains recorded for repair.
- Make persistence durable using the platform's supported file/replace/sync operations. If a failure leaves commit completion uncertain, report recovery required and reconcile recorded transaction identifiers with on-disk state; never claim rollback merely because a post-replacement sync or bookkeeping operation failed.
- After a confirmed switch, the new profile remains active. Failure to delete the old native entry is cleanup pending, with committed=true in safe error details. Do not restore the old credential after a completed switch. Reads may use an unambiguously committed active profile while obsolete-entry cleanup is pending. Ambiguous state blocks authenticated use.
- Ordinary reads never migrate or repair. auth repair explicitly reconciles interrupted transactions and retries cleanup, touching only references recorded for the current namespace. Unresolved transactions must be reconciled before another mutation of the affected state. Recovery must not guess which of two unrelated entries belongs to a profile or scan the user's Keychain for candidates.
- Migration is per profile and requires an explicit target selector, including reverse migration to file. An already-selected target is an idempotent no-op once outstanding recovery is resolved. Preserve invalid authorization evidence as invalid; migration must not turn it into absent legacy metadata.
- Removing a plaintext token means removal from the current credential file and tool-owned staging files, without retaining a compatibility backup. It does not promise forensic erasure from filesystem snapshots, external backups, or storage media.

## Logout, profile switching, and corrupted state

Logout first durably disables the selected local profile and removes its token/authorization data, retaining only references necessary for pending native cleanup. It then deletes native entries. If deletion fails, return cleanup pending; the profile remains disabled and cannot authenticate. Repeated logout or auth repair retries cleanup. A missing native item is already deleted for cleanup purposes. Logout does not revoke a Todoist token remotely or unset an overriding environment token.

Profile switching loads only the selected profile's token and associated evidence. Missing referenced secrets are errors, never invitations to use a stale file credential or another profile. Invalid metadata in an inactive profile does not invalidate a healthy selected profile.

When the surrounding file is readable, login can replace and logout can remove an invalid selected authorization record. A syntactically corrupt whole file, unsupported storage format, or mismatched namespace receives a stable error and requires deliberate recovery; none is automatically rewritten or interpreted as legacy. Repair handles recognized transaction states, not arbitrary corrupt-file reconstruction. Preserve unreadable state for the user instead of destructive recovery.

## Diagnostics and machine-client behavior

auth status remains offline and metadata-only. Configured means an active profile is recorded, not that its secret is accessible or Todoist accepts it. Keep existing credential-source meanings and report backend identity separately. Report token accessibility as unchecked until actually tested; metadata-derived authorization describes the recorded credential and is not proof of current authentication.

Doctor reports backend type and availability, then retains its existing authenticated API probe using prompt-free secret retrieval. If retrieval or authorization validation fails, report the relevant failure and skip the API request. A lightweight availability probe cannot establish that every stored entry is readable. Environment overrides bypass native access, including diagnostics of the active credential. Preserve doctor's checks/summary output and existing exit 1 behavior for failures or strict-mode warnings.

Use typed storage errors with stable symbolic codes and fixed, safe messages. Distinguish missing referenced entries, unavailable backend, access denied, interaction required, corrupt state, unsupported format, busy storage, I/O failure, cleanup pending, and recovery required. Report locked separately only when the OS explicitly establishes it; do not infer it from a generic interaction-required status. Storage/auth operation failures use exit 3, while malformed CLI arguments retain usage exit 2. Render symbolic codes and allowlisted details through the existing JSON/quiet-JSON error contract; retain existing human and NDJSON error conventions. Include operation, backend, and committed state only when established; avoid raw OS error strings, entry identifiers, and raw malformed metadata.

Native operations never show OS dialogs. No TTY detection, --force flag, retry, or machine output mode changes fallback or interaction policy. Login may still conduct its explicitly selected OAuth browser flow; the no-dialog rule concerns native credential storage.

## Secret handling

auth login --print-env remains the sole intentional token-output exception, preserving its human, JSON, and NDJSON export contracts for the newly acquired token without persistence. It does not retrieve or dump a stored native credential. Ordinary success output, errors, verbose logs, doctor reports, and test failure messages reveal no tokens. Keep examples and snapshots secret-free, and use comparisons whose failure messages never print secret values.

Native integration passes secrets directly to Security.framework, never in subprocess arguments. Native errors are classified before formatting. Any diagnostic error path must be safe even when a newly acquired or inactive-profile token differs from the active token; redacting only the active token is insufficient.

## Confirmed test boundaries

Use test-first vertical slices at these public boundaries:

1. CLI invocation: selection/precedence, explicit migration/repair, replacement/logout, status/doctor, environment override, error codes/streams, redaction and help/config contracts. Inject the store and use temporary configurations, fake tokens and local HTTP servers.
2. Profile-store contract: save/load/inspect/list/delete/probe plus migration/recovery behavior, using temporary file state and injected secret/persistence adapters. Cover availability, commit failures before/after publication, verification failure, interruption/restart, failed rollback cleanup, pending logout, concurrency, namespace mismatch, corrupted state, metadata preservation and unrelated profiles. Assert behavior through the store, not private helper call order.
3. Native-adapter contract: opt-in tests against a disposable dedicated keychain and explicit item scope in an isolated process. Never access real user entries or change the user's default keychain/search list. Test save/read/replacement/delete, missing-item classification and prompt-free failure where safely reproducible. Abort or skip with an explicit reason if isolation cannot be established; fakes cover statuses that cannot be induced safely.

Cross-platform build checks cover darwin, linux and windows on amd64 and arm64 with cgo disabled, plus macOS native builds with cgo enabled for both supported release architectures. Verify the native adapter in the runnable host architecture; distinguish compilation from runtime verification on the other architecture.

## Delivery and verification

Update README, docs/SPEC.md, auth help, config and security documentation, architecture notes, output schemas/examples and relevant help snapshots/completions. Keep domain language in CONTEXT.md and architectural choices in ADR-0004; retain ADR-0001's explicit portable fallback. Update macOS release build configuration without publishing a release.

Follow the repository [handoff workflow](../CONTRIBUTING.md#ready-for-handoff). For native-adapter or portability changes, also use the relevant specialized checks documented there. Report each result and any platform/test limitation explicitly. Preserve unrelated working-tree changes. The later explicitly invoked ship-change workflow authorizes commits, push, and PR creation after its gates and independent reviews. Do not merge a PR, publish a release, or test against real credentials.

The user confirmed this consolidated design and the three test boundaries before implementation. PR #4 landed before implementation began. The later ship-change request authorizes the shipping workflow; review is pinned to main at 2879104.

## Related decisions

- docs/adr/0001-keep-core-workflows-cross-platform.md
- docs/adr/0003-preserve-write-capability-for-unknown-credentials.md
- docs/adr/0004-select-credential-storage-explicitly.md (accepted)
