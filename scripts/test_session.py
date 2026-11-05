"""Exercise landing and refusal paths without a network, credentials or a real review."""

import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch


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
            "LC_ALL": "C",
            "XDG_CONFIG_HOME": str(self.home / "config"),
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
if any(name in os.environ for name in ("GIT_EXTERNAL_DIFF", "GIT_ATTR_SOURCE", "SHELLOPTS")):
    sys.exit("review inherited shell or Git output overrides")
Path(args[args.index("--output-last-message") + 1]).write_text(os.environ["SESSION_TEST_REVIEW"])
print(args[-1])
""")
        reviewer.chmod(0o755)
        self.git("init", "--bare", "--initial-branch=main", self.remote, cwd=self.home)
        self.git("clone", self.remote, self.repo, cwd=self.home)
        (self.repo / "Makefile").write_text(
            "check:\n"
            '\t$(MAKE) -f Makefile verify\n'
            "verify:\n"
            '\t@test -z "$$BINARY"\n'
            '\t@test -z "$$GOFLAGS" && test "$$GOENV" = off && test "$$GOWORK" = off\n'
            '\t@test -z "$$BASH_ENV" && test ! -e ignored.go\n'
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
        self.env["BINARY"] = "/path/to/a/different/todoist"
        self.env["GOFLAGS"] = "-skip=TestSpecificRegression"
        self.env["GOENV"] = "/path/to/persisted/go-flags"
        self.env["GOWORK"] = "/path/to/a/different/go.work"
        self.env["BASH_ENV"] = "/path/to/local/bash-configuration"
        self.env["SHELLOPTS"] = "noexec"
        self.env["GIT_ATTR_SOURCE"] = "refs/heads/missing"
        self.env["GIT_EXTERNAL_DIFF"] = "true"
        text = self.session("land", cwd=self.work)
        landed = self.tip()
        self.assertIn(f"Landed {landed}", text)
        self.assertEqual(self.git("rev-parse", "HEAD^", cwd=self.work).strip(), upstream)
        attempt, = self.logs.iterdir()
        self.assertIn(f"from {upstream} to {landed}", (attempt / "review.log").read_text())
        self.assertIn(landed, (attempt / "check.log").read_text())
        self.assertEqual(self.git("rev-parse", "HEAD").strip(), self.base)
        self.git("branch", "--set-upstream-to=origin/main", "fix/example")
        self.git("config", "remote.origin.fetch", "+refs/heads/other:refs/remotes/origin/other")
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
        for override in ({}, {"MAKEFLAGS": "-n"}, {"MAKEFLAGS": "-i"},
                         {"MAKE": "true"}, {"MAKE_COMMAND": "true"}):
            with self.subTest(override=override), patch.dict(self.env, override):
                self.assertIn("make exited", self.session("land", cwd=self.work, success=False))
                self.assertEqual(self.tip(), self.base)

    def test_moving_main_during_gate_stops_push(self):
        self.start()
        upstream = self.make_upstream_change(push=False)
        self.env["SESSION_TEST_MOVE"] = str(self.other)
        self.assertIn("origin/main moved", self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), upstream)

    def intercept_push(self, **settings):
        wrapper = self.home / "git"
        wrapper.write_text("""#!/usr/bin/env python3
import os
import subprocess
import sys
git = os.environ["SESSION_TEST_REAL_GIT"]
args = sys.argv[1:]
if "push" in args and any(arg.endswith(":refs/heads/main") for arg in args):
    if "SESSION_TEST_RETARGET" in os.environ:
        subprocess.run([git, "-C", os.environ["SESSION_TEST_ORIGINAL"],
                        "remote", "set-url", "--push", "origin",
                        os.environ["SESSION_TEST_RETARGET"]], check=True)
    elif "SESSION_TEST_REWRITE" in os.environ:
        url = os.environ["SESSION_TEST_REWRITE"]
        subprocess.run([git, "-C", os.environ["SESSION_TEST_ORIGINAL"],
                        "config", f"url.{url}.insteadOf",
                        os.environ["SESSION_TEST_REMOTE"]], check=True)
    elif "SESSION_TEST_VERIFY_TRANSPORT" in os.environ:
        if os.getcwd() != os.environ["SESSION_TEST_WORKTREE"]:
            sys.exit("transport settings did not keep the source working directory")
        git_dir = next(arg for arg in args if arg.startswith("--git-dir="))
        expected = {
            "http.sslVerify": b"true\\n",
            "remote.origin.proxy": b"\\n",
            "remote.origin.proxyAuthMethod": b"basic\\n",
        }
        for key, value in expected.items():
            kind = ["--bool"] if key.startswith("http.") else []
            result = subprocess.check_output([git, git_dir, "config", *kind, "--get", key])
            if result != value:
                sys.exit(f"transport option {key} was not preserved")
    else:
        subprocess.run([git, "--git-dir", os.environ["SESSION_TEST_REMOTE"],
                        "update-ref", "-d", "refs/heads/main"], check=True)
os.execv(git, [git, *args])
""")
        wrapper.chmod(0o755)
        self.env.update({
            "SESSION_TEST_REAL_GIT": shutil.which("git"),
            "SESSION_TEST_REMOTE": str(self.remote),
            "SESSION_TEST_ORIGINAL": str(self.repo),
            "SESSION_TEST_WORKTREE": str(self.work),
            "PATH": f"{self.home}{os.pathsep}{self.env['PATH']}",
            **settings,
        })

    def test_main_deleted_after_final_check_is_not_recreated(self):
        self.start()
        self.intercept_push()
        self.assertIn("stale info", self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.git("branch", "--list", "main", cwd=self.remote), "")

    def test_push_destination_stays_fixed_after_final_check(self):
        self.start()
        self.git("clone", "--bare", self.remote, self.other)
        self.intercept_push(SESSION_TEST_RETARGET=str(self.other))
        self.session("land", cwd=self.work)
        self.assertEqual(self.tip(), self.git("rev-parse", "HEAD", cwd=self.work).strip())
        self.assertEqual(self.git("rev-parse", "main", cwd=self.other).strip(), self.base)

    def test_late_origin_rewrite_cannot_redirect_the_push(self):
        self.start()
        self.git("clone", "--bare", self.remote, self.other)
        self.intercept_push(SESSION_TEST_REWRITE=str(self.other))
        self.session("land", cwd=self.work)
        self.assertEqual(self.tip(), self.git("rev-parse", "HEAD", cwd=self.work).strip())
        self.assertEqual(self.git("rev-parse", "main", cwd=self.other).strip(), self.base)

    def test_push_preserves_bare_tls_and_remote_proxy_configuration(self):
        self.start()
        with (self.repo / ".git/config").open("a") as config:
            config.write('[http]\n\tsslVerify\n[remote "origin"]\n'
                         '\tproxy = ""\n\tproxyAuthMethod = basic\n')
        self.intercept_push(SESSION_TEST_VERIFY_TRANSPORT="1")
        self.session("land", cwd=self.work)
        self.assertEqual(self.tip(), self.git("rev-parse", "HEAD", cwd=self.work).strip())

    def test_main_rewrite_does_not_restore_dropped_commits(self):
        self.make_upstream_change(push=True)
        self.start()
        self.git("update-ref", "refs/heads/main", self.base, cwd=self.remote)
        self.assertIn("no longer contains the recorded session base",
                      self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), self.base)
        self.assertFalse(self.logs.exists())

    def test_rebase_reapplies_patch_that_main_landed_and_reverted(self):
        self.start()
        self.make_upstream_change(push=False)
        self.commit_file(self.other, "topic.txt")
        self.git("revert", "--no-edit", "HEAD", cwd=self.other)
        self.git("push", "origin", "main", cwd=self.other)
        self.session("land", cwd=self.work)
        self.assertEqual(self.git("show", "main:topic.txt", cwd=self.remote), "example\n")

    def test_distinct_push_destination_is_refused(self):
        self.start()
        self.git("clone", "--bare", self.remote, self.other)
        self.git("remote", "set-url", "--push", "origin", self.other)
        self.assertIn("same single URL", self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), self.base)
        self.assertEqual(self.git("rev-parse", "main", cwd=self.other).strip(), self.base)
        self.assertFalse(self.logs.exists())

    def test_rewritten_origin_url_is_refused(self):
        self.start()
        self.git("clone", "--bare", self.remote, self.other)
        self.git("config", f"url.{self.other}.insteadOf", self.remote)
        self.assertIn("without pushurl, URL rewrites or custom remote helpers",
                      self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), self.base)
        self.assertEqual(self.git("rev-parse", "main", cwd=self.other).strip(), self.base)

    def test_remote_helper_url_is_refused_before_fetch(self):
        self.start()
        self.git("remote", "set-url", "origin", "ext::false")
        self.assertIn("without remote helper URLs", self.session("land", cwd=self.work, success=False))
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
        self.assertIn("ignored files", self.session("land", cwd=self.work, success=False))
        self.assertIn("ignored files", self.session("end", cwd=self.work, success=False))
        self.assertEqual(scratch.read_text(), "local work")

    def test_cleanup_preserves_edits_hidden_by_index_flags(self):
        self.session("start", "example")
        self.git("update-index", "--assume-unchanged", "Makefile", cwd=self.work)
        local_makefile = self.work / "Makefile"
        local_makefile.write_text("local changes\n")
        self.assertIn("clear assume-unchanged", self.session("end", cwd=self.work, success=False))
        self.assertEqual(local_makefile.read_text(), "local changes\n")

    def test_overlaps_shows_committed_untracked_and_upstream_changes(self):
        self.start()
        (self.work / "scratch").touch()
        self.git("update-index", "--assume-unchanged", "Makefile", cwd=self.work)
        (self.work / "Makefile").write_text("hidden local change\n")
        self.make_upstream_change(push=True)
        output = self.session("overlaps", "topic.txt", "scratch", "upstream.txt", "Makefile")
        self.assertIn(f"{self.work}: topic.txt", output)
        self.assertIn(f"{self.work}: scratch", output)
        self.assertIn(f"{self.work}: Makefile (index flags may hide edits)", output)
        self.assertIn("Changes on origin/main", output)
        self.assertIn("upstream.txt", output)

    def test_overlap_fetch_preserves_unpublished_tag_when_pruning_is_configured(self):
        self.git("tag", "local-only")
        self.git("config", "fetch.prune", "true")
        self.git("config", "fetch.pruneTags", "true")
        fetch_head = self.repo / ".git/FETCH_HEAD"
        fetch_head.write_text("previous fetch\n")
        self.session("overlaps", "Makefile")
        self.assertEqual(self.git("rev-parse", "refs/tags/local-only").strip(), self.base)
        self.assertEqual(fetch_head.read_text(), "previous fetch\n")

    def test_overlaps_reports_orphan_worktree_before_and_after_its_first_commit(self):
        self.session("start", "example")
        self.git("switch", "--orphan", "orphan-history", cwd=self.work)
        (self.work / "orphan.txt").write_text("local work\n")
        self.assertIn(f"{self.work}: orphan.txt", self.session("overlaps", "orphan.txt"))
        self.git("add", "orphan.txt", cwd=self.work)
        self.git("commit", "-m", "Independent history", cwd=self.work)
        self.assertIn(f"{self.work}: orphan.txt", self.session("overlaps", "orphan.txt"))


if __name__ == "__main__":
    unittest.main()
