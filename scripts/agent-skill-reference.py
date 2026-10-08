#!/usr/bin/env python3
"""Generate help-derived agent references and validate documented examples safely."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
import difflib
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys
import tempfile


@dataclass(frozen=True)
class Flag:
    takes_value: bool


@dataclass(frozen=True)
class HelpPage:
    command: tuple[str, ...]
    text: str


def help_blocks(text: str) -> dict[str, list[str]]:
    """Read declared sections, excluding flag mentions in prose and examples."""
    blocks: dict[str, list[str]] = {}
    section = ""
    for line in text.splitlines():
        if line.startswith("Aliases:"):
            section = ""  # Render alias declarations separately from usage.
        elif line and not line[0].isspace() and line.endswith(":"):
            section = line[:-1]
            blocks.setdefault(section, [])
        elif section:
            blocks[section].append(line)
    return blocks


def flags_from_help(text: str) -> dict[str, Flag]:
    flags: dict[str, Flag] = {}
    for section, lines in help_blocks(text).items():
        if section == "Usage":
            # Some focused pages declare their entire surface in usage only.
            for line in lines:
                for match in re.finditer(r"(--[a-z][a-z0-9-]*)(\s+<[^>]+>)?", line):
                    flags[match[1]] = Flag(bool(match[2]))
            continue
        if not section.lower().endswith("flags"):
            continue
        for line in lines:
            if not line.strip().startswith("-"):
                continue
            # Two spaces separate the declaration from its human description.
            declaration = re.split(r"\s{2,}", line.strip(), maxsplit=1)[0]
            names = re.findall(r"(?<!\w)(--[a-z][a-z0-9-]*|-[a-zA-Z])\b", declaration)
            takes_value = bool(re.search(r"\s+<[^>]+>", declaration))
            for name in names:
                flags[name] = Flag(takes_value)
    return flags


APPLICATION_COMMANDS = (("agent", "apply"), ("agent", "run"), ("agent", "schedule", "print"))


class CommandInventory:
    def __init__(self, pages: list[HelpPage]):
        self.pages = {page.command: page for page in pages}
        if () not in self.pages:
            raise ValueError("help manifest has no root help")
        self.global_flags = flags_from_help(self.pages[()].text)
        self.flags = {page.command: flags_from_help(page.text) for page in pages}
        self.aliases: dict[tuple[str, ...], tuple[str, ...]] = {}
        for page in pages:
            if not page.command:
                continue
            for alias_line in re.findall(r"^Aliases:\s*(.+)$", page.text, re.MULTILINE):
                for alias in alias_line.split():
                    self.aliases[page.command[:-1] + (alias,)] = page.command

    def canonical(self, command: tuple[str, ...]) -> tuple[str, ...] | None:
        if command in self.pages:
            return command
        return self.aliases.get(command)

    def has_children(self, command: tuple[str, ...]) -> bool:
        return any(len(child) > len(command) and child[:len(command)] == command
                   for child in self.pages)

    def validate(self, arguments: list[str], *, application_only: bool = False) -> bool:
        """Validate names and application confirmation without executing examples."""
        command: tuple[str, ...] = ()
        index = 0
        help_command = False
        help_flag = False
        force = False
        confirm = ""
        while index < len(arguments):
            argument = arguments[index]
            if argument in ("|", "||", ";", "&", "&&", ">", ">>", "<", "2>"):
                break
            if argument == "--":
                break
            if argument.startswith("-") and argument != "-":
                name, separator, value = argument.partition("=")
                flag = self.global_flags.get(name) or self.flags.get(command, {}).get(name)
                if flag is None:
                    raise ValueError(f"{' '.join(command) or 'root'} has no documented flag {name}")
                if flag.takes_value and not separator:
                    index += 1
                    if index == len(arguments):
                        raise ValueError(f"{name} requires a value")
                    value = arguments[index]
                if name in ("--force", "-f") and not separator:
                    force = True
                elif name == "--confirm":
                    confirm = value.strip()
                elif name in ("--help", "-h"):
                    help_flag = not separator or value in ("1", "t", "T", "TRUE", "true", "True")
                index += 1
                continue
            if not command and argument == "help":
                help_command = True
                index += 1
                continue
            if self.has_children(command):
                candidate = self.canonical(command + (argument,))
                if candidate is None:
                    raise ValueError(f"unknown command {' '.join(command + (argument,))}")
                command = candidate
                if application_only and not any(path[:len(command)] == command for path in APPLICATION_COMMANDS):
                    return False
            elif help_command:
                raise ValueError(f"unknown help target {' '.join(command + (argument,))}")
            # Remaining operands belong to a leaf and need not be command names.
            index += 1
        if (command in APPLICATION_COMMANDS
                and not (help_command or help_flag or force or confirm)):
            raise ValueError(f"{' '.join(command)} example requires --confirm or --force (including previews)")
        return not application_only or command in APPLICATION_COMMANDS


def manifest_commands(root: Path) -> list[tuple[str, ...]]:
    commands: list[tuple[str, ...]] = []
    for number, line in enumerate((root / "scripts/help-snapshots.txt").read_text().splitlines(), 1):
        if not line.strip() or line.startswith("#"):
            continue
        try:
            _, invocation = line.split("\t", 1)
            arguments = shlex.split(invocation)
        except ValueError as error:
            raise ValueError(f"help-snapshots.txt:{number}: {error}") from error
        if not arguments or arguments[-1] != "--help":
            raise ValueError(f"help-snapshots.txt:{number}: expected a --help invocation")
        command = tuple(arguments[:-1])
        if any(not re.fullmatch(r"[a-z][a-z0-9-]*", part) for part in command):
            raise ValueError(f"help-snapshots.txt:{number}: expected command names before --help")
        if command in commands:
            raise ValueError(f"help-snapshots.txt:{number}: duplicate command {' '.join(command)}")
        commands.append(command)
    return commands


def collect_help(root: Path, executable: Path, work: Path) -> list[HelpPage]:
    environment = {key: value for key, value in os.environ.items()
                   if not key.startswith("TODOIST_")}
    pages: list[HelpPage] = []
    for command in manifest_commands(root):
        invocation = [str(executable), "--config", str(work / "config.json"), *command, "--help"]
        result = subprocess.run(invocation, cwd=work, env=environment, text=True,
                                capture_output=True, timeout=30, check=False)
        if result.returncode or result.stderr:
            raise ValueError(f"help failed for {' '.join(command) or 'root'} "
                             f"(exit {result.returncode}): {result.stderr.strip()}")
        pages.append(HelpPage(command, result.stdout))
    return pages


def render_reference(pages: list[HelpPage]) -> str:
    lines = [
        "# Command reference", "",
        "Generated by `scripts/agent-skill-reference.py --write` from the executable's",
        "authoritative help and `scripts/help-snapshots.txt`. Curated workflows live in",
        "the other references. Read live help when the installed skill and executable",
        "versions differ. Search this file for the relevant command heading.", "",
        "Global flags are declared under the root command; individual commands may",
        "include a smaller reminder. Output support and operational guarantees remain",
        "command-specific; use the curated references and live schemas.", "",
    ]
    for page in sorted(pages, key=lambda page: page.command):
        name = "todoist" + (" " + " ".join(page.command) if page.command else "")
        lines.extend([f"## {name}", "", "```text"])
        for section, block in help_blocks(page.text).items():
            if section == "Usage" or section.lower().endswith("flags"):
                lines.append(section + ":")
                lines.extend(block)
        for aliases in re.findall(r"^Aliases:\s*(.+)$", page.text, re.MULTILINE):
            lines.append("Aliases: " + aliases)
        while lines[-1] == "":
            lines.pop()
        lines.extend(["```", ""])
    return "\n".join(lines)


def example_fragments(text: str, *, include_inline: bool = True):
    language: str | None = None
    pending = ""
    pending_line = 0
    for number, line in enumerate(text.splitlines(), 1):
        if line.lstrip().startswith("```"):
            language = line.lstrip()[3:].strip() if language is None else None
            continue
        if language in ("sh", "bash", "shell", "console"):
            fragment = pending + line.strip()
            first_line = pending_line or number
            if fragment.endswith("\\"):
                pending, pending_line = fragment[:-1] + " ", first_line
                continue
            pending, pending_line = "", 0
            yield first_line, fragment
        elif language is None and include_inline:
            for fragment in re.findall(r"`([^`]+)`", line):
                yield number, fragment
    if pending:
        raise ValueError(f"line {pending_line}: unfinished shell continuation")


def validate_fragments(source: str, fragments, inventory: CommandInventory,
                       *, application_only: bool = False) -> tuple[int, list[str]]:
    errors: list[str] = []
    checked = 0
    for number, fragment in fragments:
        if not re.search(r"\btodoist(?:\s|$)", fragment):
            continue
        try:
            lexer = shlex.shlex(fragment, posix=True, punctuation_chars="|;&<>")
            lexer.whitespace_split = True
            tokens = list(lexer)
            command_start = True
            for index, token in enumerate(tokens):
                if token in ("|", "||", ";", "&", "&&"):
                    command_start = True
                    continue
                if command_start and re.fullmatch(r"[a-zA-Z_][a-zA-Z0-9_]*=.*", token):
                    continue
                if command_start and token == "todoist":
                    if inventory.validate(tokens[index + 1:], application_only=application_only):
                        checked += 1
                command_start = False
        except ValueError as error:
            errors.append(f"{source}:{number}: {error}")
    return checked, errors


def validate_examples(root: Path, inventory: CommandInventory) -> tuple[int, list[str]]:
    errors: list[str] = []
    checked = 0
    bundle = root / "internal/agentskill/bundle"
    for path in sorted(bundle.rglob("*.md")):
        if path.name == "commands.md":
            continue  # Generated declarations have no manually maintained examples.
        count, failures = validate_fragments(str(path.relative_to(root)),
                                            example_fragments(path.read_text()), inventory)
        checked += count
        errors.extend(failures)
    if checked == 0:
        errors.append("agent skill contains no checked command examples")
    return checked, errors


def help_example_fragments(text: str):
    examples = False
    for number, line in enumerate(text.splitlines(), 1):
        if line and not line[0].isspace() and line.endswith(":"):
            examples = line == "Examples:"
        elif examples:
            yield number, line.strip()


def validate_public_examples(root: Path, inventory: CommandInventory,
                            pages: list[HelpPage], agent_examples: str) -> tuple[int, list[str]]:
    sources = []
    for relative in ("README.md", "docs/SPEC.md"):
        path = root / relative
        if path.exists():
            sources.append((relative, example_fragments(path.read_text(), include_inline=False)))
    for page in pages:
        sources.append(("todoist " + " ".join((*page.command, "--help")),
                        help_example_fragments(page.text)))
    sources.append(("todoist agent examples", help_example_fragments(agent_examples)))
    checked = 0
    errors: list[str] = []
    for source, fragments in sources:
        count, failures = validate_fragments(source, fragments, inventory, application_only=True)
        checked += count
        errors.extend(failures)
    return checked, errors


def collect_agent_examples(executable: Path, work: Path) -> str:
    # Only invoke this fixed local informational command, never an extracted example.
    environment = {key: value for key, value in os.environ.items()
                   if not key.startswith("TODOIST_")}
    result = subprocess.run([str(executable), "--config", str(work / "config.json"),
                             "agent", "examples"], cwd=work, env=environment, text=True,
                            capture_output=True, timeout=30, check=False)
    if result.returncode or result.stderr:
        raise ValueError(f"agent examples failed (exit {result.returncode}): {result.stderr.strip()}")
    return result.stdout


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    action = parser.add_mutually_exclusive_group()
    action.add_argument("--write", action="store_true", help="Update generated command references")
    action.add_argument("--check", action="store_true", help="Check drift and examples (default)")
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parent.parent)
    parser.add_argument("--bin", type=Path, help="Use this executable instead of building the CLI")
    args = parser.parse_args()
    root = args.root.resolve()
    try:
        with tempfile.TemporaryDirectory(prefix="todoist-skill-reference-") as scratch:
            work = Path(scratch)
            executable = args.bin.resolve() if args.bin else work / "todoist"
            if args.bin is None:
                subprocess.run(["go", "build", "-o", str(executable), "./cmd/todoist"],
                               cwd=root, check=True)
            pages = collect_help(root, executable, work)
            expected = render_reference(pages)
            destination = root / "internal/agentskill/bundle/references/commands.md"
            if args.write:
                destination.write_text(expected)
            else:
                actual = destination.read_text()
                if actual != expected:
                    diff = difflib.unified_diff(actual.splitlines(), expected.splitlines(),
                                               fromfile="installed reference", tofile="current help",
                                               lineterm="")
                    print("\n".join(diff), file=sys.stderr)
                    print("error: agent command references are stale; run "
                          "python3 scripts/agent-skill-reference.py --write", file=sys.stderr)
                    return 1
            inventory = CommandInventory(pages)
            checked, errors = validate_examples(root, inventory)
            emitted = collect_agent_examples(executable, work) if ("agent", "examples") in inventory.pages else ""
            public_checked, public_errors = validate_public_examples(root, inventory, pages, emitted)
            errors.extend(public_errors)
            if errors:
                for error in errors:
                    print("error: " + error, file=sys.stderr)
                return 1
            print(f"agent skill references {'updated' if args.write else 'are current'}; "
                  f"checked {checked} skill examples and {public_checked} public agent examples")
            return 0
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        print(f"error: agent skill reference check: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
