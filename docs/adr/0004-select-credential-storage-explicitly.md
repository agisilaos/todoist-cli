---
status: accepted
---

# Require explicit plaintext credential storage

Use native storage by default for new credential profiles, with macOS Keychain as the first native backend and explicitly selected file storage preserving the portable workflows required by ADR-0001. Existing file profiles retain their backend until migration; selecting native storage must never silently fall back to plaintext after a failure, trading extra setup on unsupported or headless systems for predictable secret handling. Keep non-secret authorization metadata readable separately from native tokens while preserving their association, and preserve configuration-directory plus profile isolation.

The consolidated contract and test boundaries in ../credential-store-design.md were confirmed before implementation.

The accepted adapter uses direct Security.framework access through cgo, trading unchanged macOS release builds for native error and interaction control; builds without that adapter retain the explicit portable file option. Profile migration is explicit, verifies the destination before removing the plaintext token, and deliberately gives up automatic downgrade compatibility rather than leaving a plaintext copy. Native entries preserve canonical configuration-directory and profile isolation, so moving a configuration directory requires deliberate recovery rather than silently sharing the original directory's credentials.

Keep a completed credential switch active when obsolete-entry deletion fails, and report cleanup pending rather than pretending the switch rolled back. Logout durably disables the profile before native deletion, so an inaccessible Keychain cannot leave that profile active; explicit repair retries recorded cleanup. These choices prioritize deterministic credential selection over claiming cross-store atomicity that the file and Keychain cannot provide.
