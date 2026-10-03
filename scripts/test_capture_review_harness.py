#!/usr/bin/env python3
"""Failure-path tests for the workflow runner itself (no Go build required)."""
import importlib.util
import os
from pathlib import Path
import sys
import tempfile
import time
import unittest
from urllib.request import urlopen

sys.dont_write_bytecode = True

spec = importlib.util.spec_from_file_location('workflow', Path(__file__).with_name('test-capture-review.py'))
workflow = importlib.util.module_from_spec(spec)
spec.loader.exec_module(workflow)


class HarnessTests(unittest.TestCase):
    def test_pty_timeout_reaps_child(self):
        with tempfile.TemporaryDirectory() as directory:
            pidfile = Path(directory) / 'pid'
            script = 'import os,time; from pathlib import Path; Path("pid").write_text(str(os.getpid())); print("started", flush=True); time.sleep(60)'
            started = time.monotonic()
            with self.assertRaisesRegex(workflow.CheckFailure, 'PTY timeout:.*') as caught:
                workflow.run([sys.executable, '-c', script], directory, os.environ, [], timeout=1)
            self.assertIn('started', str(caught.exception))
            self.assertLess(time.monotonic() - started, 5)
            with self.assertRaises(ProcessLookupError):
                os.kill(int(pidfile.read_text()), 0)

    def test_noninteractive_timeout_has_output_and_reaps_child(self):
        with tempfile.TemporaryDirectory() as directory:
            script = 'import os,time; from pathlib import Path; Path("pid").write_text(str(os.getpid())); print("waiting", flush=True); time.sleep(60)'
            with self.assertRaisesRegex(workflow.CheckFailure, 'command timeout') as caught:
                workflow.run([sys.executable, '-c', script], directory, os.environ, timeout=1)
            self.assertIn('waiting', str(caught.exception))
            with self.assertRaises(ProcessLookupError):
                os.kill(int((Path(directory) / 'pid').read_text()), 0)

    def test_missing_prompt_fails(self):
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(workflow.CheckFailure, 'missing prompt'):
                workflow.run([sys.executable, '-c', 'print("different prompt")'], directory,
                             os.environ, [('expected prompt', 'yes')], timeout=2)

    def test_fixture_closes_on_failure(self):
        fixture = workflow.Fixture()
        with self.assertRaisesRegex(RuntimeError, 'deliberate'):
            with workflow.serve(fixture) as url:
                raise RuntimeError('deliberate')
        with self.assertRaises(OSError):
            urlopen(url, timeout=1)


if __name__ == '__main__':
    unittest.main()
