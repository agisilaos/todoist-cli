# Retain selection after credential profile removal

Removing a credential profile retains any default selection pointing to its name, so the next stored invocation reports a missing credential until explicit selection or login. Clearing the default would permit fallback to another credential, trading convenient automatic recovery for unintended account or grant activation; deletion therefore preserves disabled-before-delete and cleanup-pending recovery without claiming atomic cross-store deletion.
