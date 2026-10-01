#!/usr/bin/env python3
"""Regression checks for help generation and strict, side-effect-free examples."""

import importlib.util
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SPEC = importlib.util.spec_from_file_location("agent_skill_reference", Path(__file__).with_name("agent-skill-reference.py"))
REFERENCE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = REFERENCE
SPEC.loader.exec_module(REFERENCE)


ROOT_HELP = """Usage:
  todoist [global flags] <command>

Global flags:
  -h, --help            Show help
  --json                JSON output
  --no-input            Disable prompts
  --profile <name>      Credential profile
  --config <path>       Configuration path
  --version             Version
"""
LIST_HELP = """Usage:
  todoist task list [flags]

Aliases: ls

Flags:
  --all                 Fetch every page
  --project <ref>       Select project

Notes:
  --invented is prose, not a declared flag.
"""


def inventory():
    return REFERENCE.CommandInventory([
        REFERENCE.HelpPage((), ROOT_HELP),
        REFERENCE.HelpPage(("task",), "Usage:\n  todoist task list\n"),
        REFERENCE.HelpPage(("task", "list"), LIST_HELP),
        REFERENCE.HelpPage(("task", "view"), "Usage:\n  todoist task view <ref>\n"),
        REFERENCE.HelpPage(("agent",), "Usage:\n  todoist agent plan\n"),
        REFERENCE.HelpPage(("agent", "plan"), "Usage:\n  todoist agent plan <instruction>\n\nFlags:\n  --out <file>      Plan destination\n"),
    ])


class InventoryTests(unittest.TestCase):
    def test_aliases_and_global_flags_before_command(self):
        inventory().validate(["--profile", "reader", "--json", "task", "ls", "--all", "--no-input"])

    def test_flag_shaped_global_and_local_values_are_literal(self):
        inventory().validate(["--profile", "--unknown", "task", "list", "--project", "--not-a-flag"])
        inventory().validate(["--config=/tmp/config with spaces.json", "task", "view", "id:1"])

    def test_unknown_root_and_group_children_fail(self):
        for arguments in (["invented"], ["task", "invented"], ["agent", "apply"]):
            with self.subTest(arguments=arguments), self.assertRaisesRegex(ValueError, "unknown command"):
                inventory().validate(arguments)

    def test_wrong_command_flag_and_mentioned_flag_fail(self):
        for arguments in (["task", "view", "--all"], ["task", "list", "--invented"]):
            with self.subTest(arguments=arguments), self.assertRaisesRegex(ValueError, "no documented flag"):
                inventory().validate(arguments)

    def test_missing_option_value_fails(self):
        with self.assertRaisesRegex(ValueError, "requires a value"):
            inventory().validate(["task", "list", "--project"])

    def test_usage_only_declarations_are_supported(self):
        pages = list(inventory().pages.values())
        pages.append(REFERENCE.HelpPage(("schema",), "Usage:\n  todoist schema [--name <schema>] [--json]\n"))
        REFERENCE.CommandInventory(pages).validate(["schema", "--name", "task_list", "--json"])

    def test_operands_and_double_dash_do_not_become_commands(self):
        inventory().validate(["task", "view", "id:1"])
        inventory().validate(["agent", "plan", "instruction with spaces", "--out", "private plan.json"])
        inventory().validate(["task", "view", "--", "--literal-reference"])

    def test_help_targets_are_strict(self):
        inventory().validate(["help", "task", "ls"])
        with self.assertRaisesRegex(ValueError, "unknown help target"):
            inventory().validate(["help", "task", "list", "invented"])

    def test_render_uses_help_declarations_and_not_curated_notes(self):
        pages = list(inventory().pages.values())
        expected = REFERENCE.render_reference(pages)
        self.assertIn("--all", expected)
        self.assertIn("Aliases: ls", expected)
        self.assertNotIn("--invented", expected)
        changed = [REFERENCE.HelpPage(page.command, page.text.replace("--all ", "--every-page "))
                   for page in pages]
        self.assertNotEqual(expected, REFERENCE.render_reference(changed))


class ExampleTests(unittest.TestCase):
    def test_bundle_examples_detect_commands_flags_and_quoting_without_execution(self):
        with tempfile.TemporaryDirectory(prefix="skill examples ") as scratch:
            root = Path(scratch)
            bundle = root / "internal/agentskill/bundle/references"
            bundle.mkdir(parents=True)
            (bundle / "calls.md").write_text("""Use `todoist task ls --all --json`.
```sh
todoist --profile '--value' task list --project 'Project with spaces'
todoist agent plan 'task list --invented is text' --out 'private plan.json'
todoist agent plan todoist --out 'private plan.json'
todoist task invented
todoist task view id:1 --all
todoist task list --project 'unterminated
```
""")
            # Text declarations are generated, not shell examples.
            (bundle / "commands.md").write_text("```text\ntodoist made-up\n```\n")
            count, errors = REFERENCE.validate_examples(root, inventory())
            self.assertEqual(count, 4)
            self.assertEqual(len(errors), 3)
            self.assertIn("unknown command task invented", errors[0])
            self.assertIn("no documented flag --all", errors[1])
            self.assertIn("No closing quotation", errors[2])

    def test_continued_commands_and_pipe_are_checked(self):
        shell = "```sh\ntodoist task list " + "\\\n" + "  --all --json | cat\n```\n"
        fragments = list(REFERENCE.example_fragments(shell))
        self.assertEqual(fragments, [(2, "todoist task list  --all --json | cat")])
        inventory().validate(["task", "list", "--all", "--json", "|", "cat"])

    def test_missing_examples_is_a_failure(self):
        with tempfile.TemporaryDirectory() as scratch:
            count, errors = REFERENCE.validate_examples(Path(scratch), inventory())
            self.assertEqual(count, 0)
            self.assertIn("no checked command examples", errors[0])

    def test_manifest_requires_informational_help_commands(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = Path(scratch)
            (root / "scripts").mkdir()
            manifest = root / "scripts/help-snapshots.txt"
            manifest.write_text("root.txt\t--help\nlist.txt\ttask list --help\n")
            self.assertEqual(REFERENCE.manifest_commands(root), [(), ("task", "list")])
            manifest.write_text("bad.txt\ttask delete --yes\n")
            with self.assertRaisesRegex(ValueError, "expected a --help"):
                REFERENCE.manifest_commands(root)
            manifest.write_text("bad.txt\ttask delete -- --help\n")
            with self.assertRaisesRegex(ValueError, "expected command names"):
                REFERENCE.manifest_commands(root)

    def test_live_generation_drift_and_example_errors(self):
        with tempfile.TemporaryDirectory(prefix="skill live help ") as scratch:
            root = Path(scratch)
            (root / "scripts").mkdir()
            (root / "scripts/help-snapshots.txt").write_text("root.txt\t--help\ntask.txt\ttask --help\nlist.txt\ttask list --help\n")
            bundle = root / "internal/agentskill/bundle"
            (bundle / "references").mkdir(parents=True)
            examples = bundle / "SKILL.md"
            examples.write_text("```sh\ntodoist task ls --all --json\n```\n")
            executable = root / "fake todoist"
            helps = {"": ROOT_HELP, "task": "Usage:\n  todoist task list\n", "task list": LIST_HELP}

            def write_executable(pages):
                executable.write_text("#!/usr/bin/env python3\nimport os, sys\n"
                                      "assert not any(key.startswith('TODOIST_') for key in os.environ)\n"
                                      "assert sys.argv[1] == '--config' and sys.argv[-1] == '--help'\n"
                                      f"pages = {pages!r}\n"
                                      "print(pages[' '.join(sys.argv[3:-1])], end='')\n")
                executable.chmod(0o755)

            write_executable(helps)
            command = [sys.executable, str(Path(REFERENCE.__file__)), "--root", str(root), "--bin", str(executable)]
            environment = {**os.environ, "TODOIST_TOKEN": "synthetic-secret"}
            written = subprocess.run([*command, "--write"], text=True, capture_output=True, env=environment)
            self.assertEqual(written.returncode, 0, written.stderr)
            destination = bundle / "references/commands.md"
            original = destination.read_bytes()
            checked = subprocess.run(command, text=True, capture_output=True, env=environment)
            self.assertEqual(checked.returncode, 0, checked.stderr)
            self.assertEqual(destination.read_bytes(), original)
            write_executable({**helps, "task list": LIST_HELP.replace("Fetch every page", "Fetch all supported pages")})
            stale = subprocess.run(command, text=True, capture_output=True, env=environment)
            self.assertEqual(stale.returncode, 1)
            self.assertIn("references are stale", stale.stderr)
            self.assertEqual(destination.read_bytes(), original)
            write_executable(helps)
            examples.write_text("```sh\ntodoist task invented --json\n```\n")
            invalid = subprocess.run(command, text=True, capture_output=True, env=environment)
            self.assertEqual(invalid.returncode, 1)
            self.assertIn("unknown command task invented", invalid.stderr)
            self.assertNotIn("synthetic-secret", written.stdout + written.stderr + checked.stdout + checked.stderr)


if __name__ == "__main__":
    unittest.main()
