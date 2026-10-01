## install-codex

argv: ["/tmp/todoist-installable-agent-skill-review", "skill", "install", "codex", "--scope", "local", "--path", "/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli", "--no-input", "--json"]
exit: 0

stdout:
{
  "operation": "install",
  "status": "installed",
  "target": "codex",
  "scope": "local",
  "path": "/private/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli",
  "package_digest": "dad5b29dff7cac6d0317d68dcaea5208914ad926f17e9ca94fcb9d3405ce8d7a",
  "version": "dev",
  "files": [
    "SKILL.md",
    "references/authorization.md",
    "references/commands.md",
    "references/machine-calls.md",
    "references/plans.md"
  ],
  "retained": []
}

stderr:


## update-codex

argv: ["/tmp/todoist-installable-agent-skill-review", "skill", "update", "codex", "--scope", "local", "--path", "/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli", "--no-input", "--json"]
exit: 0

stdout:
{
  "operation": "update",
  "status": "unchanged",
  "target": "codex",
  "scope": "local",
  "path": "/private/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli",
  "package_digest": "dad5b29dff7cac6d0317d68dcaea5208914ad926f17e9ca94fcb9d3405ce8d7a",
  "version": "dev",
  "files": [],
  "retained": []
}

stderr:


## modified-update-human

argv: ["/tmp/todoist-installable-agent-skill-review", "skill", "update", "codex", "--scope", "local", "--path", "/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli", "--no-input"]
exit: 5

stdout:

stderr:
error: Owned skill files were changed or removed. Restore them, or use update --backup to preserve changed originals before replacement.
Code: SKILL_MODIFIED
Path: "/private/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli"
Affected file: "SKILL.md"


## modified-update-json

argv: ["/tmp/todoist-installable-agent-skill-review", "skill", "update", "codex", "--scope", "local", "--path", "/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli", "--no-input", "--json"]
exit: 5

stdout:

stderr:
{
  "code": "SKILL_MODIFIED",
  "details": {
    "committed": false,
    "files": [
      "SKILL.md"
    ],
    "path": "/private/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli",
    "recovery_required": false
  },
  "error": "Owned skill files were changed or removed. Restore them, or use update --backup to preserve changed originals before replacement.",
  "meta": {}
}


## modified-uninstall-ndjson

argv: ["/tmp/todoist-installable-agent-skill-review", "skill", "uninstall", "codex", "--scope", "local", "--path", "/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli", "--no-input", "--ndjson"]
exit: 5

stdout:

stderr:
{"code":"SKILL_MODIFIED","details":{"committed":false,"files":["SKILL.md"],"path":"/private/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli","recovery_required":false},"error":"Owned skill files were changed or removed. Restore them, or use uninstall --keep-modified to remove management while retaining changed files.","meta":{}}


## busy-human-concurrent-4

argv: ["/tmp/todoist-installable-agent-skill-review", "skill", "install", "codex", "--scope", "local", "--path", "/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Concurrent recovery project with spaces/.agents/skills/todoist-cli", "--no-input"]
exit: 5

stdout:

stderr:
error: Another skill operation holds the installation lock. Wait for it to finish; after an interruption, verify no operation is running before removing the lock.
Code: SKILL_BUSY
Path: "/private/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Concurrent recovery project with spaces/.agents/.todoist-cli.lock"


## busy-json

argv: ["/tmp/todoist-installable-agent-skill-review", "skill", "update", "codex", "--scope", "local", "--path", "/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Concurrent recovery project with spaces/.agents/skills/todoist-cli", "--no-input", "--json"]
exit: 5

stdout:

stderr:
{
  "code": "SKILL_BUSY",
  "details": {
    "committed": false,
    "files": [],
    "path": "/private/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Concurrent recovery project with spaces/.agents/.todoist-cli.lock",
    "recovery_required": false
  },
  "error": "Another skill operation holds the installation lock. Wait for it to finish; after an interruption, verify no operation is running before removing the lock.",
  "meta": {}
}


## busy-ndjson

argv: ["/tmp/todoist-installable-agent-skill-review", "skill", "update", "codex", "--scope", "local", "--path", "/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Concurrent recovery project with spaces/.agents/skills/todoist-cli", "--no-input", "--ndjson"]
exit: 5

stdout:

stderr:
{"code":"SKILL_BUSY","details":{"committed":false,"files":[],"path":"/private/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Concurrent recovery project with spaces/.agents/.todoist-cli.lock","recovery_required":false},"error":"Another skill operation holds the installation lock. Wait for it to finish; after an interruption, verify no operation is running before removing the lock.","meta":{}}


## backup-update

argv: ["/tmp/todoist-installable-agent-skill-review", "skill", "update", "codex", "--scope", "local", "--path", "/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli", "--backup", "--no-input", "--json"]
exit: 0

stdout:
{
  "operation": "update",
  "status": "updated",
  "target": "codex",
  "scope": "local",
  "path": "/private/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli",
  "package_digest": "dad5b29dff7cac6d0317d68dcaea5208914ad926f17e9ca94fcb9d3405ce8d7a",
  "version": "dev",
  "files": [
    "SKILL.md",
    "references/authorization.md",
    "references/commands.md",
    "references/machine-calls.md",
    "references/plans.md"
  ],
  "retained": [],
  "backup_path": "/private/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/todoist-cli-backup-1509645321"
}

stderr:


## uninstall-codex

argv: ["/tmp/todoist-installable-agent-skill-review", "skill", "uninstall", "codex", "--scope", "local", "--path", "/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli", "--no-input", "--json"]
exit: 0

stdout:
{
  "operation": "uninstall",
  "status": "uninstalled",
  "target": "codex",
  "scope": "local",
  "path": "/private/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli",
  "package_digest": "dad5b29dff7cac6d0317d68dcaea5208914ad926f17e9ca94fcb9d3405ce8d7a",
  "version": "dev",
  "files": [
    "SKILL.md",
    "references/authorization.md",
    "references/commands.md",
    "references/machine-calls.md",
    "references/plans.md"
  ],
  "retained": [
    "unrelated-internal-note.txt"
  ]
}

stderr:


## repeat-uninstall-codex

argv: ["/tmp/todoist-installable-agent-skill-review", "skill", "uninstall", "codex", "--scope", "local", "--path", "/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli", "--no-input", "--ndjson"]
exit: 0

stdout:
{"operation":"uninstall","status":"unchanged","target":"codex","scope":"local","path":"/private/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli","files":[],"retained":["unrelated-internal-note.txt"]}

stderr:


## keep-modified-uninstall-claude

argv: ["/tmp/todoist-installable-agent-skill-review", "skill", "uninstall", "claude-code", "--scope", "local", "--path", "/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.claude/skills/todoist-cli", "--keep-modified", "--no-input", "--json"]
exit: 0

stdout:
{
  "operation": "uninstall",
  "status": "uninstalled",
  "target": "claude-code",
  "scope": "local",
  "path": "/private/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.claude/skills/todoist-cli",
  "package_digest": "dad5b29dff7cac6d0317d68dcaea5208914ad926f17e9ca94fcb9d3405ce8d7a",
  "version": "dev",
  "files": [
    "references/authorization.md",
    "references/commands.md",
    "references/machine-calls.md",
    "references/plans.md"
  ],
  "retained": [
    "SKILL.md",
    "unrelated-internal-note.txt"
  ]
}

stderr:


## retained-reinstall-conflict

argv: ["/tmp/todoist-installable-agent-skill-review", "skill", "install", "claude-code", "--scope", "local", "--path", "/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.claude/skills/todoist-cli", "--no-input", "--json"]
exit: 5

stdout:

stderr:
{
  "code": "SKILL_CONFLICT",
  "details": {
    "committed": false,
    "files": [
      "SKILL.md"
    ],
    "path": "/private/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.claude/skills/todoist-cli/SKILL.md",
    "recovery_required": false
  },
  "error": "An unowned file conflicts with this package. Move it or choose another destination; backup does not replace unowned content.",
  "meta": {}
}


## invalid-relative-path

argv: ["/tmp/todoist-installable-agent-skill-review", "skill", "install", "codex", "--scope", "local", "--path", ".agents/skills/todoist-cli", "--no-input", "--json"]
exit: 2

stdout:

stderr:
{
  "code": "SKILL_PATH_INVALID",
  "details": {
    "committed": false,
    "files": [],
    "path": ".agents/skills/todoist-cli",
    "recovery_required": false
  },
  "error": "Use a full absolute skill destination path.",
  "meta": {}
}


## absent-update

argv: ["/tmp/todoist-installable-agent-skill-review", "skill", "update", "codex", "--scope", "local", "--path", "/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli", "--no-input", "--json"]
exit: 4

stdout:

stderr:
{
  "code": "SKILL_NOT_INSTALLED",
  "details": {
    "committed": false,
    "files": [],
    "path": "/private/var/folders/3g/w0d8dz1d26v_577vm4z0j8jr0000gn/T/todoist skill consumer fq1s0fvo/final rerun cu0qotqx/Workspace with spaces/.agents/skills/todoist-cli",
    "recovery_required": false
  },
  "error": "No managed skill is installed here. Run skill install for this target and path first.",
  "meta": {}
}
