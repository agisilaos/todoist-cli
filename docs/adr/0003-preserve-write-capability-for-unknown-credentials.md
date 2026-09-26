# Preserve Write Capability for Unknown Credentials

Legacy stored credentials, manually supplied tokens, and environment tokens have no reliable scope evidence; blocking their writes would break existing human workflows and machine clients. They therefore retain unknown authorization mode while the CLI permits attempted writes, choosing compatibility over requiring verified OAuth scope evidence and leaving Todoist to enforce their actual permissions. This compatibility rule applies only when authorization metadata is absent or intentionally unknown: present but invalid, contradictory, or unsupported metadata must not silently become write-capable.

## Consequences

An environment token overrides the selected credential profile without inheriting its authorization information. Exporting an OAuth token and later supplying it through the environment loses the CLI's scope evidence, although it does not change Todoist's actual grant. The machine output contract distinguishes unknown authorization from read-write authorization and explicitly identifies write capability granted by this compatibility rule.
