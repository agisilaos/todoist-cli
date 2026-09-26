#!/usr/bin/env python3
"""Enforce shared documentation contract for CLI repositories."""

from __future__ import annotations

import argparse
import re
import shlex
import sys
from pathlib import Path
from urllib.parse import unquote, urlsplit

REQUIRED_HEADINGS = ["Install", "Usage", "Release", "Docs"]
REQUIRED_RELEASE_LINES = [
    "make changelog-context VERSION=vX.Y.Z",
    "make release-check VERSION=vX.Y.Z",
    "make release-dry-run VERSION=vX.Y.Z",
    "make release VERSION=vX.Y.Z",
    "scripts/changelog-context.sh",
    "scripts/release-check.sh",
    "scripts/release.sh",
    "RELEASING.md",
]
FORBIDDEN_PATTERNS = [
    re.compile(r"^## \[Unreleased\]", flags=re.MULTILINE),
]


def has_h2_heading(text: str, heading: str) -> bool:
    pattern = re.compile(rf"^##\s+{re.escape(heading)}\s*$", flags=re.MULTILINE)
    return bool(pattern.search(text))


def iter_docs_markdown(root: Path) -> list[Path]:
    files = sorted(root.glob("*.md"))

    docs_dir = root / "docs"
    if docs_dir.exists():
        files.extend(sorted(docs_dir.rglob("*.md")))

    return files


def local_link_errors(root: Path, path: Path, text: str) -> list[str]:
    errors = []
    in_fence = False
    for number, line in enumerate(text.splitlines(), start=1):
        if line.lstrip().startswith("```"):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        # Inline Markdown links; external URLs are deliberately not fetched in CI.
        for target in re.findall(r"\[[^\]]*\]\(([^\s)]+)\)", line):
            link = urlsplit(target.strip("<>"))
            if link.scheme or link.netloc or not link.path:
                continue
            destination = path.parent / unquote(link.path)
            if not destination.exists():
                errors.append(f"{path.relative_to(root)}:{number}: missing link target: {target}")
    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description="Validate CLI docs contract")
    parser.add_argument("--root", default=".", help="Repository root (default: current directory)")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    readme = root / "README.md"

    if not readme.exists():
        print("error: README.md not found", file=sys.stderr)
        return 1

    readme_text = readme.read_text(encoding="utf-8")

    errors: list[str] = []

    for name in ("CHANGELOG.md", "RELEASING.md"):
        if not (root / name).is_file():
            errors.append(f"{name} not found")

    for heading in REQUIRED_HEADINGS:
        if not has_h2_heading(readme_text, heading):
            errors.append(f"README.md missing required heading: ## {heading}")

    headings = re.findall(r"^##\s+(.+?)\s*$", readme_text, flags=re.MULTILINE)
    actual = [heading for heading in headings if heading in REQUIRED_HEADINGS]
    if actual != REQUIRED_HEADINGS:
        errors.append("README.md headings must appear once in order: " + ", ".join(REQUIRED_HEADINGS))

    for line in REQUIRED_RELEASE_LINES:
        if line not in readme_text:
            errors.append(f"README.md missing release reference: {line}")

    # Keep the existing shell quoting check; tokenization never executes examples.
    for number, line in enumerate(readme_text.splitlines(), start=1):
        if line.strip().startswith("todoist "):
            try:
                shlex.split(line)
            except ValueError as exc:
                errors.append(f"README.md:{number}: invalid command example: {exc}")

    for path in iter_docs_markdown(root):
        text = path.read_text(encoding="utf-8")
        rel = path.relative_to(root)
        errors.extend(local_link_errors(root, path, text))
        for pattern in FORBIDDEN_PATTERNS:
            if pattern.search(text):
                errors.append(f"{rel} contains forbidden pattern: {pattern.pattern}")

    if errors:
        for err in errors:
            print(f"error: {err}", file=sys.stderr)
        return 1

    print("docs contract check passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
