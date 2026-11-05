"""Exercise landing and refusal paths without a network, credentials or a real review."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SESSION = Path(__file__).with_name("session").resolve()


class SessionTest(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory(prefix="todoist-session-test-")
        self.addCleanup(tmp.cleanup)
        self.home = Path(tmp.name).resolve()
        self.repo = self.home / "test repo"
        self.remote = self.home / "remote.git"
        self.work = self.home / "test repo-example"
        self.other = self.home / "other-clone"
        self.logs = self.home / "logs"
        reviewer = self.home / "reviewer"
        self.env = {
            **{key: value for key, value in os.environ.items() if not key.startswith("GIT_")},
            "GIT_CONFIG_GLOBAL": os.devnull,
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_AUTHOR_NAME": "Session Test",
            "GIT_AUTHOR_EMAIL": "test@example.invalid",
            "GIT_COMMITTER_NAME": "Session Test",
            "GIT_COMMITTER_EMAIL": "test@example.invalid",
            "TODOIST_SESSION_LOG_DIR": str(self.logs),
            "TODOIST_CODEX": str(reviewer),
            "SESSION_TEST_REVIEW": '{"verdict":"clean","findings":[]}',
            "SESSION_TEST_MOVE": "",
            "SESSION_TEST_CHECK": "0",
        }
        reviewer.write_text("""#!/usr/bin/env python3
import os
from pathlib import Path
import sys
args = sys.argv
Path(args[args.index("--output-last-message") + 1]).write_text(os.environ["SESSION_TEST_REVIEW"])
print(args[-1])
""")
        reviewer.chmod(0o755)
        self.git("init", "--bare", "--initial-branch=main", self.remote, cwd=self.home)
        self.git("clone", self.remote, self.repo, cwd=self.home)
        (self.repo / "Makefile").write_text(
            "check:\n"
            '\tgit rev-parse HEAD\n'
            '\t@if [ -n "$$SESSION_TEST_MOVE" ]; then git -C "$$SESSION_TEST_MOVE" push origin main; fi\n'
            '\t@exit "$$SESSION_TEST_CHECK"\n'
        )
        self.git("add", "Makefile")
        self.git("commit", "-m", "Base")
        self.git("push", "origin", "main")
        self.base = self.git("rev-parse", "HEAD").strip()

    def run_command(self, *args, cwd=None, success=True):
        result = subprocess.run(args, cwd=cwd or self.repo, env=self.env, text=True,
                                stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=20)
        self.assertEqual(result.returncode == 0, success, result.stdout)
        return result.stdout

    def git(self, *args, **kwargs):
        return self.run_command("git", *args, **kwargs)

    def session(self, *args, **kwargs):
        return self.run_command(SESSION, *args, **kwargs)

    def tip(self):
        return self.git("rev-parse", "main", cwd=self.remote).strip()

    def commit_file(self, repo, path):
        (repo / path).write_text("example\n")
        self.git("add", path, cwd=repo)
        self.git("commit", "-m", f"Add {path}", cwd=repo)

    def start(self):
        self.session("start", "example")
        self.commit_file(self.work, "topic.txt")

    def make_upstream_change(self, *, push):
        self.git("clone", self.remote, self.other)
        self.commit_file(self.other, "upstream.txt")
        if push:
            self.git("push", "origin", "main", cwd=self.other)
        return self.git("rev-parse", "HEAD", cwd=self.other).strip()

    def test_start_rebase_review_check_push_and_cleanup(self):
        self.start()
        upstream = self.make_upstream_change(push=True)
        text = self.session("land", cwd=self.work)
        landed = self.tip()
        self.assertIn(f"Landed {landed}", text)
        self.assertEqual(self.git("rev-parse", "HEAD^", cwd=self.work).strip(), upstream)
        attempt, = self.logs.iterdir()
        self.assertIn(f"from {upstream} to {landed}", (attempt / "review.log").read_text())
        self.assertIn(landed, (attempt / "check.log").read_text())
        self.assertEqual(self.git("rev-parse", "HEAD").strip(), self.base)
        self.session("end", cwd=self.work)
        self.assertFalse(self.work.exists())
        self.assertEqual(self.git("branch", "--list", "fix/example"), "")

    def test_findings_and_malformed_review_stop_before_the_gate(self):
        self.start()
        for verdict in ('{"verdict":"findings","findings":["Regression in topic.txt:1"]}', "invalid"):
            with self.subTest(verdict=verdict):
                self.env["SESSION_TEST_REVIEW"] = verdict
                self.assertIn("review did not", self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), self.base)
        self.assertFalse(list(self.logs.glob("*/check.log")))

    def test_failed_gate_stops_push(self):
        self.start()
        self.env["SESSION_TEST_CHECK"] = "3"
        for flags in ("", "-n", "-i"):
            with self.subTest(makeflags=flags):
                self.env["MAKEFLAGS"] = flags
                self.assertIn("make exited", self.session("land", cwd=self.work, success=False))
                self.assertEqual(self.tip(), self.base)

    def test_moving_main_during_gate_stops_push(self):
        self.start()
        upstream = self.make_upstream_change(push=False)
        self.env["SESSION_TEST_MOVE"] = str(self.other)
        self.assertIn("origin/main moved", self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), upstream)

    def test_distinct_push_destination_is_refused(self):
        self.start()
        self.git("clone", "--bare", self.remote, self.other)
        self.git("remote", "set-url", "--push", "origin", self.other)
        self.assertIn("same single URL", self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), self.base)
        self.assertEqual(self.git("rev-parse", "main", cwd=self.other).strip(), self.base)
        self.assertFalse(self.logs.exists())

    def test_dirty_session_and_unlanded_cleanup_are_refused(self):
        self.start()
        self.assertIn("not on origin/main", self.session("end", cwd=self.work, success=False))
        (self.work / "scratch").touch()
        self.assertIn("uncommitted", self.session("land", cwd=self.work, success=False))
        self.assertTrue(self.work.exists())
        self.assertEqual(self.tip(), self.base)
        self.assertFalse(self.logs.exists())

    def test_cleanup_preserves_ignored_files(self):
        self.session("start", "example")
        (self.repo / ".git/info/exclude").write_text("scratch\n")
        scratch = self.work / "scratch"
        scratch.write_text("local work")
        self.assertIn("ignored files", self.session("end", cwd=self.work, success=False))
        self.assertEqual(scratch.read_text(), "local work")

    def test_overlaps_shows_committed_untracked_and_upstream_changes(self):
        self.start()
        (self.work / "scratch").touch()
        self.make_upstream_change(push=True)
        output = self.session("overlaps", "topic.txt", "scratch", "upstream.txt")
        self.assertIn(f"{self.work}: topic.txt", output)
        self.assertIn(f"{self.work}: scratch", output)
        self.assertIn("Changes on origin/main", output)
        self.assertIn("upstream.txt", output)


if __name__ == "__main__":
    unittest.main()
