"""Regression cases for the documentation gate; no network or CLI mutations."""

import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

CHECKER = Path(__file__).with_name("docs-contract-check.py")


class DocsContractTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.readme = self.root / "README.md"
        self.readme.write_text(
            "## Install\n## Usage\n## Release\n## Docs\n"
            "make changelog-context VERSION=vX.Y.Z\n"
            "make release-check VERSION=vX.Y.Z\n"
            "make release-dry-run VERSION=vX.Y.Z\n"
            "make release VERSION=vX.Y.Z\n"
            "scripts/changelog-context.sh scripts/release-check.sh scripts/release.sh RELEASING.md\n"
        )
        (self.root / "RELEASING.md").write_text("# Releasing\n")
        (self.root / "CHANGELOG.md").write_text("## [v1.0.0] - 2026-01-01\n- Existing release.\n")

    def check(self):
        return subprocess.run(
            [sys.executable, str(CHECKER), "--root", str(self.root)],
            capture_output=True, text=True, check=False,
        )

    def test_valid_contract(self):
        self.assertEqual(self.check().returncode, 0)

    def test_heading_order(self):
        self.readme.write_text(self.readme.read_text().replace(
            "## Install\n## Usage", "## Usage\n## Install"
        ))
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("once in order", result.stderr)

    def test_unreleased_in_changelog(self):
        (self.root / "CHANGELOG.md").write_text("## [Unreleased]\n")
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Unreleased", result.stderr)

    def test_missing_changelog(self):
        (self.root / "CHANGELOG.md").unlink()
        self.assertNotEqual(self.check().returncode, 0)

    def test_broken_command_quoting(self):
        self.readme.write_text(self.readme.read_text() + 'todoist add "unfinished\n')
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("invalid command example", result.stderr)

    def test_local_links_across_root_and_nested_docs(self):
        docs = self.root / "docs"
        docs.mkdir()
        (docs / "README.md").write_text("[release](../RELEASING.md#release)\n")
        (self.root / "CONTRIBUTING.md").write_text("[docs](docs/README.md)\n")
        self.assertEqual(self.check().returncode, 0)
        (docs / "README.md").write_text("[missing](missing.md)\n")
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("docs/README.md:1: missing link target", result.stderr)

    def test_external_links_and_fenced_examples_are_not_checked(self):
        (self.root / "CONTRIBUTING.md").write_text(
            "[external](https://example.invalid/docs)\n"
            "```md\n[example](missing.md)\n```\n"
        )
        self.assertEqual(self.check().returncode, 0)


if __name__ == "__main__":
    unittest.main()
