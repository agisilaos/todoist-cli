# Todoist CLI

A terminal companion for Todoist that gives people fast interactive workflows over a stable contract for scripts and agents.

## Language

**Terminal companion**:
A human-first interface for Todoist workflows where the terminal offers a meaningful advantage, while leaving visual planning to Todoist's graphical applications.
_Avoid_: Terminal clone, Todoist replacement

**Machine client**:
A script, agent, or other non-interactive caller that depends on explicit inputs and stable output contracts.
_Avoid_: Bot, automation user

**Machine output contract**:
The documented structured output and error behavior that machine clients can rely on across compatible releases.
_Avoid_: JSON mode, agent output

**Authentication**:
Establishing that Todoist accepts a credential. Authentication alone does not establish which permissions the credential has.
_Avoid_: Authorization, write access

**Authorization scope**:
A permission associated with a credential by Todoist.
_Avoid_: Authentication scope, CLI permission

**Requested scopes**:
The set of Todoist permissions the CLI asks for during an OAuth authorization flow. Requested scopes are distinct from the permissions established by the completed exchange.
_Avoid_: Granted scopes, effective scopes

**Effective scopes**:
The Todoist permissions established by a successful OAuth exchange, using the returned scopes or the requested scopes when the response omits them under OAuth's scope rules. Effective scopes cannot be established merely from an externally supplied opaque token.
_Avoid_: Requested scopes, write capability

**Scope evidence**:
The basis for the CLI's knowledge of effective scopes: an explicit OAuth token response or the scope rules of a successful OAuth exchange. Independently supplied opaque tokens have no such evidence.
_Avoid_: Token validity, credential source

**Authorization mode**:
The CLI's evidence-based classification of a credential as read-only, read-write, or unknown. Unknown remains unknown even when the CLI's authorization policy permits writes.
_Avoid_: Authentication mode, credential source

**Credential source**:
Where the current invocation obtained its credential, such as an environment variable or a stored profile.
_Avoid_: Credential origin, login method

**Credential origin**:
How a stored credential was acquired, such as PKCE, device authorization, or manual entry.
_Avoid_: Credential source, authorization mode

**Credential profile**:
A named stored credential and its associated authorization information, selected together for an invocation. A token supplied through the environment overrides that selection without inheriting the profile's authorization information.
_Avoid_: Todoist account, authorization mode

**Write capability**:
Whether the CLI's authorization policy permits the current invocation to attempt a Todoist mutation. Todoist still determines whether the request succeeds.
_Avoid_: Granted scope, guaranteed write access

**Todoist mutation**:
A change to remotely stored Todoist data, including notification read state and account settings. Changes to local credentials, configuration, or plan files are not Todoist mutations.
_Avoid_: Any write, local change

**Dry run**:
A preview of proposed operations during which the CLI dispatches no Todoist mutations. Necessary reads and the command's documented local effects may still occur; external planners remain independently executing programs.
_Avoid_: Applied action, sandboxed execution

**Review set**:
The explicit collection of tasks selected at the start of a review and accounted for in its final summary.
_Avoid_: Today view, batch

**Disposition**:
The deliberate outcome assigned to a task during a review: kept, changed, completed, or skipped.
_Avoid_: Action, status

**Applied action**:
An agent-plan action whose Todoist mutation succeeded and whose replay record was stored. Until both occur, the CLI does not report the action as successful.
_Avoid_: Completed action, successful request

**Replay record**:
Durable evidence that a specific action from a confirmed agent plan has already changed Todoist, preventing the same action from being applied again.
_Avoid_: Journal entry, applied marker
