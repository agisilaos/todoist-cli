#!/usr/bin/env python3
"""Bounded recovery consumer check: isolated CLI, local API, real PTY input.

This is not live Todoist verification. The author knows the implementation.
"""
import json
import os
from pathlib import Path
import pty
import signal
import subprocess
import tempfile
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ROOT = Path(__file__).resolve().parent.parent
TOKEN = 'synthetic-recovery-token'


class Fixture(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def reply(self, value):
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps(value).encode())

    def do_GET(self):
        with self.server.lock:
            if self.path.startswith('/tasks/filter?'):
                value = {'results': [dict(self.server.task)], 'next_cursor': None}
            elif self.path == '/tasks/fixture-task':
                value = dict(self.server.task)
            elif self.path.startswith('/projects') or self.path.startswith('/sections'):
                value = {'results': [], 'next_cursor': None}
            else:
                self.send_error(404)
                return
        self.reply(value)

    def do_POST(self):
        if self.path != '/tasks/fixture-task':
            self.send_error(404)
            return
        payload = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
        with self.server.lock:
            self.server.task.update(payload)
            self.server.task['updated_at'] = 'after'
            self.server.writes += 1
        # Accept the mutation, but deliberately withhold its response until the
        # client is interrupted. Inspection must observe the accepted change.
        self.server.accepted.set()
        self.server.release.wait(10)
        self.close_connection = True


def record(args, code, stdout, stderr):
    # Inputs and fixture state are synthetic; never inherit real credentials.
    print(json.dumps({'command': 'todoist ' + ' '.join(args), 'exit': code,
                      'stdout': stdout, 'stderr': stderr}))


def run(binary, scratch, env, args, data=None):
    result = subprocess.run([str(binary), *args], cwd=scratch, env=env,
                            input=data, capture_output=True, text=True, timeout=15)
    record(args, result.returncode, result.stdout, result.stderr)
    assert TOKEN not in result.stdout + result.stderr, 'credential leaked'
    return result


def interrupted_review(binary, scratch, env, plan, machine, server):
    args = ['review', '--out', str(plan)] + (['--json'] if machine else [])
    master, slave = pty.openpty()
    with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
        process = subprocess.Popen([str(binary), *args], cwd=scratch, env=env,
                                   stdin=slave, stdout=stdout, stderr=stderr)
        sent = interrupted = False
        try:
            deadline = time.monotonic() + 15
            while time.monotonic() < deadline:
                stderr.seek(0)
                captured = stderr.read()
                if b'Disposition [keep/change/complete/skip]:' in captured and not sent:
                    os.write(master, b'change\ncontent\nReconciled content\ndone\nyes\n')
                    sent = True
                if server.accepted.is_set() and not interrupted:
                    process.send_signal(signal.SIGINT)
                    interrupted = True
                if process.poll() is not None:
                    break
                time.sleep(0.02)
            assert process.poll() is not None, 'review timed out'
            assert sent and interrupted, 'fixture did not reach interruption boundary'
            stdout.seek(0)
            stderr.seek(0)
            out, err = stdout.read().decode(), stderr.read().decode()
            record(args, process.returncode, out, err)
            assert process.returncode == 1, 'interrupted review exit changed'
            assert TOKEN not in out + err, 'credential leaked'
            assert server.writes == 1, 'review duplicated mutation'
            if machine:
                report = json.loads(out)
                assert report['tasks'][0]['actions'][0]['remote_outcome_uncertain']
                assert 'literal reference:' not in err, 'human advice in machine mode'
            else:
                assert 'literal reference: "id:fixture-task"' in err
                assert 'Do not reapply this plan' in err
                assert 'completed history/activity' in err
            return json.loads(plan.read_text())
        finally:
            server.release.set()
            if process.poll() is None:
                process.kill()
            process.wait()
            os.close(master)
            os.close(slave)


def main():
    with tempfile.TemporaryDirectory(prefix='todoist-recovery-') as directory:
        scratch = Path(directory)
        binary = scratch / 'todoist'
        subprocess.run(['go', 'build', '-o', str(binary), './cmd/todoist'], cwd=ROOT, check=True)
        server = ThreadingHTTPServer(('127.0.0.1', 0), Fixture)
        server.lock = threading.Lock()
        threading.Thread(target=server.serve_forever, daemon=True).start()
        try:
            for machine in (False, True):
                config_dir = scratch / ('machine' if machine else 'human')
                config_dir.mkdir()
                (config_dir / 'config.json').write_text('{"credential_store":"file"}')
                env = {k: v for k, v in os.environ.items() if not k.startswith('TODOIST_')}
                env.update(HOME=str(scratch), XDG_CONFIG_HOME=str(scratch), NO_COLOR='1',
                           TODOIST_CONFIG=str(config_dir / 'config.json'), TODOIST_PROFILE='work',
                           TODOIST_BASE_URL=f'http://127.0.0.1:{server.server_port}')
                server.task = {'id': 'fixture-task', 'content': 'Original content',
                               'description': '', 'project_id': 'fixture-project',
                               'labels': [], 'priority': 1, 'updated_at': 'before'}
                server.writes = 0
                server.accepted = threading.Event()
                server.release = threading.Event()
                # Public route: README setup and command help, then the actual failure.
                for args in (['--help'], ['review', '--help'], ['task', 'view', '--help']):
                    result = run(binary, scratch, env, args)
                    assert result.returncode == 0
                flags = ['--json', '--quiet-json'] if machine else []
                result = run(binary, scratch, env, ['today', '--no-input', *flags])
                assert result.returncode == 3 and not result.stdout
                if machine:
                    assert set(json.loads(result.stderr)) == {'error', 'meta'}
                else:
                    assert 'auth login --no-input --token-stdin' in result.stderr
                result = run(binary, scratch, env, ['auth', 'login', '--no-input', '--token-stdin', *flags], TOKEN)
                assert result.returncode == 0
                stored = json.loads((config_dir / 'credentials.json').read_text())
                assert stored['profiles']['work']['token'] == TOKEN
                plan_path = config_dir / 'review-plan.json'
                plan = interrupted_review(binary, scratch, env, plan_path, machine, server)
                journal = config_dir / 'agent_replay.json'
                evidence = journal.read_bytes()
                assert any(c['pending'] for c in json.loads(evidence)['reviews'].values())
                result = run(binary, scratch, env, ['task', 'view', 'id:fixture-task', '--full', '--no-input', *flags])
                assert result.returncode == 0 and 'Reconciled content' in result.stdout
                assert server.writes == 1 and journal.read_bytes() == evidence
                # Deliberate negative safety check, not recommended recovery advice.
                result = run(binary, scratch, env, ['agent', 'apply', '--plan', str(plan_path),
                             '--confirm', plan['confirm_token'], '--no-input', *flags])
                assert result.returncode == 5 and server.writes == 1
                assert journal.read_bytes() == evidence and json.loads(plan_path.read_text()) == plan
                if machine:
                    assert json.loads(result.stdout)['tasks'][0]['actions'][0]['remote_outcome_uncertain']
                    assert set(json.loads(result.stderr)) == {'error', 'meta'}
                result = run(binary, scratch, env, ['review', '--no-input', *flags])
                assert result.returncode == 2 and not result.stdout and server.writes == 1
                print(json.dumps({'mode': 'machine' if machine else 'human', 'mutations': server.writes,
                                  'observed_content': server.task['content'], 'replay_evidence': 'unchanged'}))
        finally:
            server.shutdown()
            server.server_close()
    print('recovery consumer checks passed (local fixture; PTY stdin; no live Todoist verification)')


if __name__ == '__main__':
    main()
