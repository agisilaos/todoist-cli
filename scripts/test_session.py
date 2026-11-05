"""Exercise landing and refusal paths without a network, credentials or a real review."""

import os
from pathlib import Path
import shutil
import subprocess
import sys
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
            "SESSION_TEST_POISON_PUSH": "",
            "SESSION_TEST_POISON_REVIEW": "",
            "SESSION_TEST_CHECK": "0",
        }
        reviewer.write_text("""#!/usr/bin/env python3
import os
from pathlib import Path
import sys
args = sys.argv
if any(name in os.environ for name in ("GIT_EXTERNAL_DIFF", "GIT_ATTR_SOURCE", "SHELLOPTS",
                                       "PYTHONOPTIMIZE")):
    sys.exit("review inherited shell or Git output overrides")
if any(name.startswith("BASH_FUNC_") or value.startswith("() {")
       for name, value in os.environ.items()):
    sys.exit("review inherited exported shell functions")
if os.environ["SESSION_TEST_POISON_REVIEW"]:
    Path("Makefile").write_text("check:\\n\\t@true\\n")
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
            '\t@if [ -n "$$SESSION_TEST_POISON_PUSH" ]; then git config remote.origin.pushurl "$$SESSION_TEST_POISON_PUSH"; fi\n'
            '\t@if [ -n "$$SESSION_TEST_MOVE" ]; then git -C "$$SESSION_TEST_MOVE" push origin main; fi\n'
            '\t@exit "$$SESSION_TEST_CHECK"\n'
        )
        self.git("add", "Makefile")
        self.git("commit", "-m", "Base")
        self.git("push", "origin", "main")
        self.base = self.git("rev-parse", "HEAD").strip()

    def run_command(self, *args, cwd=None, success=True, input=None):
        result = subprocess.run(args, cwd=cwd or self.repo, env=self.env, text=True, input=input,
                                stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=60)
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
        self.git("commit", "--amend", "-m", "Add topic", "-m", "What changed:", cwd=self.work)
        self.git("config", "rebase.instructionFormat", "%s%n%b")
        upstream = self.make_upstream_change(push=True)
        self.git("config", "core.hooksPath", os.devnull)
        self.env["BINARY"] = "/path/to/a/different/todoist"
        self.env["GOFLAGS"] = "-skip=TestSpecificRegression"
        self.env["GOENV"] = "/path/to/persisted/go-flags"
        self.env["GOWORK"] = "/path/to/a/different/go.work"
        self.env["BASH_ENV"] = "/path/to/local/bash-configuration"
        self.env["SHELLOPTS"] = "noexec"
        self.env["GIT_ATTR_SOURCE"] = "refs/heads/missing"
        self.env["GIT_EXTERNAL_DIFF"] = "true"
        self.env["BASH_FUNC_go%%"] = "() { true; }"
        self.env["PYTHONOPTIMIZE"] = "1"
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
        bundle, = self.home.glob(".test repo-ended-*/session.bundle")
        self.assertEqual(self.git("bundle", "list-heads", bundle), f"{landed} refs/heads/fix/example\n")

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

    def intercept_git(self, **settings):
        wrapper = self.home / "git"
        wrapper.write_text("""#!/usr/bin/env python3
import os
from pathlib import Path
import subprocess
import sys
git = os.environ["SESSION_TEST_REAL_GIT"]
args = sys.argv[1:]
if "worktree" in args and "move" in args and "SESSION_TEST_LATE_IGNORED" in os.environ:
    Path(os.environ["SESSION_TEST_LATE_IGNORED"]).write_text("late local work")
    makefile = Path(os.environ["SESSION_TEST_WORKTREE"]) / "Makefile"
    original = makefile.read_text()
    makefile.write_text(original.replace("\\tgit rev-parse HEAD\\n",
                                         "\\t@echo late staged work\\n"))
    subprocess.run([git, "-C", str(makefile.parent), "add", "Makefile"], check=True)
    makefile.write_text(original)
if "worktree" in args and "move" in args and "SESSION_TEST_LATE_CONFLICT" in os.environ:
    work = os.environ["SESSION_TEST_WORKTREE"]
    blob = subprocess.check_output([git, "-C", work, "rev-parse", "HEAD:Makefile"],
                                   text=True).strip()
    entries = (f"0 {'0' * len(blob)}\\tMakefile\\n100644 {blob} 1\\tMakefile\\n"
               f"100644 {blob} 2\\tMakefile\\n")
    subprocess.run([git, "-C", work, "update-index", "--index-info"],
                   input=entries, text=True, check=True)
if "diff" in args and "--cached" in args and "SESSION_TEST_INTERRUPT_ARCHIVE" in os.environ:
    sys.exit(42)
if "fetch" in args and "SESSION_TEST_VERIFY_TRANSPORT" in os.environ:
    git_dir = next(arg for arg in args if arg.startswith("--git-dir="))
    if not {"--no-auto-maintenance", "--no-write-commit-graph"}.issubset(args):
        sys.exit("fetch maintenance was not disabled")
    for key, expected in {
        "fetch.fsckObjects": b"true\\n",
        "transfer.fsckObjects": b"true\\n",
        "fetch.fsck.missingEmail": b"error\\n",
    }.items():
        if subprocess.check_output([git, git_dir, "config", "--get", key]) != expected:
            sys.exit(f"fetch validation option {key} was not preserved")
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
    elif "SESSION_TEST_SYMLINK" in os.environ:
        link = Path(os.environ["SESSION_TEST_SYMLINK"])
        link.unlink()
        link.symlink_to(os.environ["SESSION_TEST_LINK_TARGET"])
    elif "SESSION_TEST_LOSE_PUSH_RESPONSE" in os.environ:
        subprocess.run([git, *args], check=True)
        sys.exit(42)
    elif "SESSION_TEST_VERIFY_TRANSPORT" in os.environ:
        if os.getcwd() != os.environ["SESSION_TEST_WORKTREE"]:
            sys.exit("transport settings did not keep the source working directory")
        git_dir = next(arg for arg in args if arg.startswith("--git-dir="))
        expected = {
            "http.sslVerify": b"true\\n",
            "remote.origin.proxy": b"\\n",
            "remote.origin.proxyAuthMethod": b"basic\\n",
            "push.gpgSign": b"if-asked\\n",
            "user.signingkey": b"test-signing-key\\n",
            "user.useConfigOnly": b"true\\n",
            "committer.name": b"Fixture signer\\n",
            "committer.email": b"signer@example.invalid\\n",
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
        with patch.dict(self.env):
            self.intercept_git()
            self.assertIn("stale info", self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.git("branch", "--list", "main", cwd=self.remote), "")
        self.git("update-ref", "refs/heads/main", self.base, cwd=self.remote)
        self.session("land", cwd=self.work)
        self.assertEqual(self.tip(), self.git("rev-parse", "HEAD", cwd=self.work).strip())

    def test_definite_server_rejection_can_be_retried(self):
        self.start()
        hook = self.remote / "hooks/pre-receive"
        hook.parent.mkdir(exist_ok=True)
        hook.write_text("#!/bin/sh\nexit 1\n")
        hook.chmod(0o755)
        self.assertIn("[remote rejected]", self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), self.base)
        hook.unlink()
        self.session("land", cwd=self.work)
        self.assertEqual(self.tip(), self.git("rev-parse", "HEAD", cwd=self.work).strip())

    def test_push_destination_stays_fixed_after_final_check(self):
        self.start()
        self.git("clone", "--bare", self.remote, self.other)
        self.intercept_git(SESSION_TEST_RETARGET=str(self.other))
        self.session("land", cwd=self.work)
        self.assertEqual(self.tip(), self.git("rev-parse", "HEAD", cwd=self.work).strip())
        self.assertEqual(self.git("rev-parse", "main", cwd=self.other).strip(), self.base)

    def test_late_origin_rewrite_cannot_redirect_the_push(self):
        self.start()
        self.git("clone", "--bare", self.remote, self.other)
        self.intercept_git(SESSION_TEST_REWRITE=str(self.other))
        self.session("land", cwd=self.work)
        self.assertEqual(self.tip(), self.git("rev-parse", "HEAD", cwd=self.work).strip())
        self.assertEqual(self.git("rev-parse", "main", cwd=self.other).strip(), self.base)

    def test_late_symlink_retarget_cannot_redirect_the_push(self):
        self.start()
        self.git("clone", "--bare", self.remote, self.other)
        link = self.home / "origin-link"
        link.symlink_to(self.remote)
        self.git("remote", "set-url", "origin", link.as_uri())
        self.intercept_git(SESSION_TEST_SYMLINK=str(link), SESSION_TEST_LINK_TARGET=str(self.other))
        self.session("land", cwd=self.work)
        self.assertEqual(self.tip(), self.git("rev-parse", "HEAD", cwd=self.work).strip())
        self.assertEqual(self.git("rev-parse", "main", cwd=self.other).strip(), self.base)

    def test_gate_checkout_config_cannot_redirect_the_push(self):
        self.start()
        self.git("clone", "--bare", self.remote, self.other)
        self.env["SESSION_TEST_POISON_PUSH"] = str(self.other)
        self.session("land", cwd=self.work)
        self.assertEqual(self.tip(), self.git("rev-parse", "HEAD", cwd=self.work).strip())
        self.assertEqual(self.git("rev-parse", "main", cwd=self.other).strip(), self.base)

    def test_review_checkout_changes_cannot_replace_the_gate(self):
        self.start()
        self.env["SESSION_TEST_POISON_REVIEW"] = "1"
        self.env["SESSION_TEST_CHECK"] = "3"
        self.assertIn("make exited", self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), self.base)

    def test_relative_path_cannot_select_make_from_the_candidate(self):
        self.start()
        tools = self.work / "tools"
        tools.mkdir()
        fake_make = tools / "make"
        fake_make.write_text("#!/bin/sh\nexit 0\n")
        fake_make.chmod(0o755)
        self.git("add", "tools/make", cwd=self.work)
        self.git("commit", "-m", "Add fake make", cwd=self.work)
        self.env["PATH"] = f"tools{os.pathsep}{self.env['PATH']}"
        self.env["SESSION_TEST_CHECK"] = "3"
        self.assertIn("make exited", self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), self.base)

    def test_push_preserves_bare_tls_and_remote_proxy_configuration(self):
        self.start()
        with (self.repo / ".git/config").open("a") as config:
            config.write('[http]\n\tsslVerify\n[remote "origin"]\n'
                         '\tproxy = ""\n\tproxyAuthMethod = basic\n'
                         '[push]\n\tgpgSign = if-asked\n[user]\n\tsigningkey = test-signing-key\n'
                         '\tuseConfigOnly = true\n[committer]\n\tname = Fixture signer\n'
                         '\temail = signer@example.invalid\n'
                         '[fetch]\n\tfsckObjects = true\n[transfer]\n\tfsckObjects = true\n'
                         '[fetch "fsck"]\n\tmissingEmail = error\n')
        self.intercept_git(SESSION_TEST_VERIFY_TRANSPORT="1")
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

    def test_main_rewind_cannot_republish_a_previously_landed_tip(self):
        self.start()
        self.session("land", cwd=self.work)
        self.git("update-ref", "refs/heads/main", self.base, cwd=self.remote)
        self.assertIn("no longer contains the recorded session base",
                      self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), self.base)

    def test_main_rewind_after_a_lost_push_response_requires_reconciliation(self):
        self.start()
        head = self.git("rev-parse", "HEAD", cwd=self.work).strip()
        with patch.dict(self.env):
            self.intercept_git(SESSION_TEST_LOSE_PUSH_RESPONSE="1")
            self.assertIn("push exited 42", self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), head)
        self.git("update-ref", "refs/heads/main", self.base, cwd=self.remote)
        self.assertIn("outcome of the previous push", self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), self.base)

    def test_main_rewrite_does_not_restore_a_manually_rebased_fork_point(self):
        self.start()
        self.make_upstream_change(push=True)
        self.git("fetch", "origin", cwd=self.work)
        self.git("rebase", "origin/main", cwd=self.work)
        self.git("update-ref", "refs/heads/main", self.base, cwd=self.remote)
        self.assertIn("no longer contains the session's fork point",
                      self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), self.base)

    def test_fast_forward_uses_recorded_base_without_a_tracking_reflog(self):
        self.start()
        upstream = self.make_upstream_change(push=True)
        self.git("config", "core.logAllRefUpdates", "false")
        reflog = self.repo / ".git/logs/refs/remotes/origin/main"
        if reflog.exists():
            reflog.unlink()
        self.session("land", cwd=self.work)
        self.assertEqual(self.git("rev-parse", "main^", cwd=self.remote).strip(), upstream)

    def test_abort_keeps_the_last_successful_base_without_a_tracking_reflog(self):
        self.start()
        self.make_upstream_change(push=False)
        (self.other / "topic.txt").write_text("conflicting upstream work\n")
        self.git("add", "topic.txt", cwd=self.other)
        self.git("commit", "-m", "Conflicting upstream work", cwd=self.other)
        self.git("push", "origin", "main", cwd=self.other)
        self.git("config", "core.logAllRefUpdates", "false")
        reflog = self.repo / ".git/logs/refs/remotes/origin/main"
        if reflog.exists():
            reflog.unlink()
        self.assertIn("rebase failed", self.session("land", cwd=self.work, success=False))
        self.git("rebase", "--abort", cwd=self.work)
        self.git("revert", "--no-edit", "HEAD", cwd=self.other)
        self.git("push", "origin", "main", cwd=self.other)
        self.session("land", cwd=self.work)
        self.assertEqual(self.git("show", "main:topic.txt", cwd=self.remote), "example\n")

    def test_resolved_base_survives_a_later_aborted_rebase_without_a_reflog(self):
        self.start()
        self.make_upstream_change(push=False)
        self.git("config", "core.logAllRefUpdates", "false")
        reflog = self.repo / ".git/logs/refs/remotes/origin/main"
        if reflog.exists():
            reflog.unlink()
        for content in ("first upstream edit\n", "second upstream edit\n"):
            (self.other / "topic.txt").write_text(content)
            self.git("add", "topic.txt", cwd=self.other)
            self.git("commit", "-m", "Edit topic upstream", cwd=self.other)
            self.git("push", "origin", "main", cwd=self.other)
            self.assertIn("rebase failed", self.session("land", cwd=self.work, success=False))
            if content.startswith("first"):
                (self.work / "topic.txt").write_text("example\n")
                self.git("add", "topic.txt", cwd=self.work)
                self.git("-c", "core.editor=true", "rebase", "--continue", cwd=self.work)
            else:
                self.git("rebase", "--abort", cwd=self.work)
        self.git("update-ref", "refs/heads/main", self.base, cwd=self.remote)
        self.assertIn("no longer contains the recorded session base",
                      self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), self.base)

    def test_landing_refuses_a_custom_merge_driver_before_rebase(self):
        self.start()
        original = self.git("rev-parse", "HEAD", cwd=self.work).strip()
        self.make_upstream_change(push=True)
        self.git("config", "merge.keep-upstream.driver", "true")
        self.assertIn("requires built-in merge drivers",
                      self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.git("rev-parse", "HEAD", cwd=self.work).strip(), original)
        self.assertFalse(self.logs.exists())

    def test_rebase_reapplies_patch_that_main_landed_and_reverted(self):
        self.start()
        self.make_upstream_change(push=False)
        self.commit_file(self.other, "topic.txt")
        self.git("revert", "--no-edit", "HEAD", cwd=self.other)
        self.git("push", "origin", "main", cwd=self.other)
        self.session("land", cwd=self.work)
        self.assertEqual(self.git("show", "main:topic.txt", cwd=self.remote), "example\n")

    def test_rebase_stops_on_a_newly_empty_topic_commit(self):
        self.start()
        original = self.git("rev-parse", "HEAD", cwd=self.work).strip()
        self.make_upstream_change(push=False)
        self.commit_file(self.other, "topic.txt")
        self.git("push", "origin", "main", cwd=self.other)
        self.assertIn("rebase failed", self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.git("rev-parse", "ORIG_HEAD", cwd=self.work).strip(), original)
        self.assertFalse(self.logs.exists())

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

    def test_landing_refuses_to_skip_a_source_pre_push_hook(self):
        self.start()
        hook = self.home / "hooks " / "pre-push"
        hook.parent.mkdir()
        self.git("config", "core.hooksPath", hook.parent)
        hook.write_text("#!/bin/sh\nexit 1\n")
        hook.chmod(0o755)
        self.assertIn("source checkout has a pre-push hook",
                      self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), self.base)

    def test_candidate_requires_a_regular_tracked_makefile(self):
        self.start()
        external = self.home / "skip-make"
        external.write_text("check:\n\t@true\n")
        makefile = self.work / "Makefile"
        makefile.unlink()
        makefile.symlink_to(external)
        self.git("add", "Makefile", cwd=self.work)
        self.git("commit", "-m", "Link to external Makefile", cwd=self.work)
        self.assertIn("regular tracked Makefile", self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), self.base)
        self.assertFalse(self.logs.exists())

    def test_landing_refuses_a_configured_hook_with_file_hooks_disabled(self):
        self.start()
        self.git("config", "core.hooksPath", os.devnull)
        self.git("config", "hook.local-gate.event", "pre-push")
        self.git("config", "hook.local-gate.command", "false")
        self.assertIn("configured pre-push hook", self.session("land", cwd=self.work, success=False))
        self.assertEqual(self.tip(), self.base)

    def test_landing_refuses_a_pre_rebase_hook_before_it_can_write_files(self):
        self.start()
        hook = self.repo / ".git/hooks/pre-rebase"
        hook.parent.mkdir(exist_ok=True)
        hook.write_text("#!/bin/sh\nprintf danger > scratch\n")
        hook.chmod(0o755)
        self.assertIn("source checkout has a pre-rebase hook",
                      self.session("land", cwd=self.work, success=False))
        self.assertFalse((self.work / "scratch").exists())

    def test_dirty_session_and_unlanded_cleanup_are_refused(self):
        self.start()
        self.assertIn("not on origin/main", self.session("end", cwd=self.work, success=False))
        (self.work / "scratch").touch()
        self.assertIn("uncommitted", self.session("land", cwd=self.work, success=False))
        self.assertTrue(self.work.exists())
        self.assertEqual(self.tip(), self.base)
        self.assertFalse(self.logs.exists())

    def test_cleanup_ignores_grafts_that_hide_an_unlanded_commit(self):
        self.start()
        head = self.git("rev-parse", "HEAD", cwd=self.work).strip()
        graft = self.home / "grafts"
        graft.write_text(f"{self.base} {head}\n{head}\n")
        self.env["GIT_GRAFT_FILE"] = str(graft)
        self.assertEqual(self.git("-c", "advice.graftFileDeprecated=false", "rev-list",
                                  f"{self.base}..HEAD", cwd=self.work), "")
        self.assertIn("not on origin/main", self.session("end", cwd=self.work, success=False))
        self.assertTrue(self.work.exists())

    def test_cleanup_rejects_an_alternate_index_that_hides_staged_only_changes(self):
        self.session("start", "example")
        makefile = self.work / "Makefile"
        original = makefile.read_text()
        makefile.write_text("staged local changes\n")
        self.git("add", "Makefile", cwd=self.work)
        makefile.write_text(original)
        with patch.dict(self.env, {"GIT_INDEX_FILE": str(self.home / "alternate-index")}):
            self.git("read-tree", "HEAD", cwd=self.work)
            self.assertEqual(self.git("status", "--porcelain", cwd=self.work), "")
            self.assertIn("GIT_INDEX_FILE", self.session("end", cwd=self.work, success=False))
        self.assertEqual(self.git("show", ":Makefile", cwd=self.work), "staged local changes\n")

    def test_cleanup_retains_ignored_file_created_during_the_move(self):
        self.session("start", "example")
        (self.repo / ".git/info/exclude").write_text("scratch\n")
        self.git("config", "color.ui", "always")
        self.git("config", "diff.noprefix", "true")
        self.git("config", "diff.context", "0")
        self.intercept_git(SESSION_TEST_LATE_IGNORED=str(self.work / "scratch"))
        self.session("end", cwd=self.work)
        archive, = self.home.glob(".test repo-ended-*/worktree")
        self.assertEqual((archive / "scratch").read_text(), "late local work")
        self.assertIn("+\t@echo late staged work", (archive.parent / "staged.patch").read_text())
        self.git("apply", "--check", archive.parent / "staged.patch")
        self.assertTrue((archive.parent / "git-metadata/index").is_file())
        self.assertFalse(self.work.exists())

    def test_interrupted_staged_patch_keeps_the_index_registered(self):
        self.session("start", "example")
        (self.repo / ".git/info/exclude").write_text("scratch\n")
        self.intercept_git(SESSION_TEST_LATE_IGNORED=str(self.work / "scratch"),
                           SESSION_TEST_INTERRUPT_ARCHIVE="1")
        self.session("end", cwd=self.work, success=False)
        archive, = self.home.glob(".test repo-ended-*/worktree")
        self.assertIn(f"worktree {archive}", self.git("worktree", "list", "--porcelain"))
        self.assertIn("\t@echo late staged work", self.git("show", ":Makefile", cwd=archive))

    def test_cleanup_keeps_a_late_unmerged_index_registered(self):
        self.session("start", "example")
        self.intercept_git(SESSION_TEST_LATE_CONFLICT="1")
        self.assertIn("contains unmerged index entries",
                      self.session("end", cwd=self.work, success=False))
        archive, = self.home.glob(".test repo-ended-*/worktree")
        self.assertIn(f"worktree {archive}", self.git("worktree", "list", "--porcelain"))
        self.assertIn("\tMakefile", self.git("ls-files", "--unmerged", cwd=archive))

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
        self.git("config", "core.ignorecase", "true")
        self.env["GIT_LITERAL_PATHSPECS"] = "1"
        output = self.session("overlaps", "TOPIC.TXT/guide.md", "scratch",
                              "UPSTREAM.TXT/guide.md", "makefile")
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

    def test_overlaps_reports_upstream_paths_even_after_revert(self):
        self.start()
        self.make_upstream_change(push=False)
        self.git("revert", "--no-edit", "HEAD", cwd=self.other)
        self.git("push", "origin", "main", cwd=self.other)
        output = self.session("overlaps", "upstream.txt", cwd=self.work)
        self.assertIn("Changes on origin/main", output)
        self.assertIn("upstream.txt", output)

    def test_overlaps_reports_other_sessions_paths_even_after_revert(self):
        self.start()
        self.git("revert", "--no-edit", "HEAD", cwd=self.work)
        self.assertIn(f"{self.work}: topic.txt", self.session("overlaps", "topic.txt"))

    def test_overlaps_includes_unreplayed_commits_in_a_paused_rebase(self):
        self.start()
        self.commit_file(self.work, "later.txt")
        self.make_upstream_change(push=False)
        (self.other / "topic.txt").write_text("upstream conflict\n")
        self.git("add", "topic.txt", cwd=self.other)
        self.git("commit", "-m", "Conflicting topic", cwd=self.other)
        self.git("push", "origin", "main", cwd=self.other)
        self.git("fetch", "origin", cwd=self.work)
        self.git("rebase", "origin/main", cwd=self.work, success=False)
        self.assertIn(f"{self.work}: later.txt", self.session("overlaps", "later.txt"))
        output = self.session("overlaps", "upstream.txt", cwd=self.work)
        self.assertIn("Changes on origin/main", output)
        self.assertIn("upstream.txt", output)

    def test_overlaps_matches_canonically_equivalent_unicode(self):
        self.session("start", "example")
        self.commit_file(self.work, "caf\u00e9.txt")
        self.assertIn(f"{self.work}: caf\u00e9.txt", self.session("overlaps", "cafe\u0301.txt"))

    def test_candidate_rejects_case_collisions_before_checkout(self):
        blob = self.git("rev-parse", "HEAD:Makefile").strip()
        tree = self.git("mktree", input=f"100644 blob {blob}\tAlpha.go\n"
                                        f"100644 blob {blob}\talpha.go\n").strip()
        source = (
            "from pathlib import Path; import runpy, sys; "
            "script = runpy.run_path(sys.argv[1]); "
            "script['require_portable_tree'](sys.argv[2], Path.cwd(), None)"
        )
        self.assertIn("paths collide on macOS",
                      self.run_command(sys.executable, "-c", source, SESSION, tree, success=False))

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
