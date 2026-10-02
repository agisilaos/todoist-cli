#!/usr/bin/env python3
"""Capture/review/replay contract against a synthetic API, never live Todoist."""
import argparse
from contextlib import contextmanager
import copy
import errno
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import os
from pathlib import Path
import pty
import select
import signal
import subprocess
import tempfile
import threading
import time
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parent.parent
TOKEN = 'synthetic-capture-review-token'
TASK_ID = '123456'
DISPOSITION = 'Disposition [keep/change/complete/skip]:'
CONFIRM = 'Apply this plan? [yes/no]:'


class CheckFailure(Exception):
    pass


def require(condition, message):
    # Explicit checks remain enabled under python -O.
    if not condition:
        raise CheckFailure(message)


class Fixture:
    def __init__(self, fault=None):
        self.fault = fault
        self.tasks = {}
        self.writes = []
        self.errors = []
        self.requests = []

    def snapshot(self):
        return copy.deepcopy(self.tasks), list(self.writes)

    def handler(self):
        fixture = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def reply(self, value, status=200):
                data = json.dumps(value).encode() if status != 204 else b''
                self.send_response(status)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            def dispatch(self):
                self.connection.settimeout(2)
                try:
                    fixture.requests.append((self.command, self.path))
                    require(self.headers.get('Authorization') == 'Bearer ' + TOKEN,
                            'fixture received unexpected authentication')
                    path = urlsplit(self.path).path
                    if self.command == 'GET':
                        if path == '/projects':
                            return self.reply({'results': [{'id': '100', 'name': 'Inbox', 'inbox_project': True}], 'next_cursor': None})
                        if path == '/sections':
                            return self.reply({'results': [], 'next_cursor': None})
                        if path == '/tasks/filter':
                            return self.reply({'results': [t for t in fixture.tasks.values() if not t['checked']], 'next_cursor': None})
                        if path == '/tasks/' + TASK_ID and TASK_ID in fixture.tasks:
                            return self.reply(fixture.tasks[TASK_ID])
                    else:
                        # Count every attempted mutation, including rejected/unknown routes.
                        fixture.writes.append((self.command, path))
                        body = json.loads(self.rfile.read(int(self.headers.get('Content-Length', 0))) or '{}')
                        if self.command == 'POST' and path == '/tasks':
                            require(body.get('content') == 'Capture workflow task', 'unexpected capture payload')
                            if fixture.fault == 'reject-capture':
                                return self.reply({'error': 'deliberate capture rejection'}, 400)
                            require(not fixture.tasks, 'duplicate capture')
                            task = {'id': TASK_ID, 'content': 'Saved workflow task',
                                    'description': '', 'project_id': '100', 'labels': ['fixture'],
                                    'priority': 3, 'due': {'date': '2020-01-01', 'is_recurring': False},
                                    'checked': False, 'updated_at': '2020-01-01T00:00:00Z'}
                            fixture.tasks[TASK_ID] = task
                            returned = dict(task)
                            if fixture.fault == 'wrong-receipt':
                                returned['content'] = 'Wrong returned content'
                            return self.reply(returned)
                        if self.command == 'POST' and path == '/tasks/' + TASK_ID + '/close':
                            require(TASK_ID in fixture.tasks, 'completion before capture')
                            if fixture.fault != 'ignore-completion':
                                fixture.tasks[TASK_ID]['checked'] = True
                            return self.reply({}, 204)
                    raise CheckFailure('unexpected fixture route: ' + self.command + ' ' + self.path)
                except Exception as error:
                    fixture.errors.append(str(error))
                    self.reply({'error': 'fixture failure'}, 500)

            do_GET = do_POST = do_PUT = do_PATCH = do_DELETE = dispatch

        return Handler


@contextmanager
def serve(fixture):
    class BoundedServer(HTTPServer):
        def get_request(self):
            connection, address = super().get_request()
            connection.settimeout(2)
            return connection, address

    server = BoundedServer(('127.0.0.1', 0), fixture.handler())
    thread = threading.Thread(target=server.serve_forever, kwargs={'poll_interval': 0.05})
    thread.start()
    try:
        yield f'http://127.0.0.1:{server.server_port}'
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=3)
        require(not thread.is_alive(), 'fixture thread did not stop')


def run(command, scratch, env, prompts=None, timeout=15):
    """Return exit/stdout/stderr; PTY mode merges output as an actual terminal does."""
    if prompts is None:
        process = subprocess.Popen(command, cwd=scratch, env=env, stdin=subprocess.PIPE,
                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                   text=True, start_new_session=True)
        try:
            out, err = process.communicate('', timeout=timeout)
            return process.returncode, out, err
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            out, err = process.communicate()
            raise CheckFailure('command timeout: ' + repr(command) + '\n' + out + err)
        finally:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGKILL)
            process.wait()
    master, slave = pty.openpty()
    process = None
    captured = bytearray()
    remaining = list(prompts)
    cursor = 0
    try:
        process = subprocess.Popen(command, cwd=scratch, env=env, stdin=slave,
                                   stdout=slave, stderr=slave, start_new_session=True)
        os.close(slave)
        slave = None
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if select.select([master], [], [], 0.05)[0]:
                try:
                    chunk = os.read(master, 65536)
                except OSError as error:
                    if error.errno != errno.EIO:
                        raise
                    break
                if not chunk:
                    break
                captured.extend(chunk)
            if remaining:
                prompt, answer = remaining[0]
                position = captured.find(prompt.encode(), cursor)
                if position >= 0:
                    cursor = position + len(prompt.encode())
                    os.write(master, (answer + '\n').encode())
                    remaining.pop(0)
        try:
            process.wait(timeout=max(0.01, deadline - time.monotonic()))
        except subprocess.TimeoutExpired:
            raise CheckFailure('PTY timeout: ' + repr(command) + '\n' + captured.decode(errors='replace'))
        output = captured.decode(errors='replace').replace('\r\n', '\n')
        require(not remaining, 'missing prompt ' + repr(remaining) + '\n' + output)
        return process.returncode, output, ''
    finally:
        if process is not None:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGKILL)
            process.wait()
        os.close(master)
        if slave is not None:
            os.close(slave)


def exercise(binary, fault=None, verbose=False, diagnostics=True):
    fixture = Fixture(fault)
    transcript = []
    with tempfile.TemporaryDirectory(prefix='todoist-capture-review-') as directory, serve(fixture) as url:
        scratch = Path(directory)
        config = scratch / 'config' / 'config.json'
        config.parent.mkdir()
        config.write_text('{"credential_store":"file"}')
        # Allowlist process inputs: no ambient credentials, proxies, planner, profile,
        # project configuration, display mode, or user config can affect the CLI.
        env = {'PATH': os.defpath, 'HOME': str(scratch), 'XDG_CONFIG_HOME': str(scratch),
               'TMPDIR': str(scratch), 'NO_COLOR': '1', 'TERM': 'dumb', 'TZ': 'UTC',
               'TODOIST_TOKEN': TOKEN, 'TODOIST_CONFIG': str(config), 'TODOIST_BASE_URL': url}

        def cli(args, prompts=None, expected=0):
            code, out, err = run([str(binary), *args], scratch, env, prompts)
            transcript.append({'args': args, 'exit': code, 'stdout': out, 'stderr': err})
            require(not fixture.errors, 'fixture errors: ' + repr(fixture.errors))
            require(TOKEN not in out + err, 'synthetic credential leaked')
            require(code == expected, f'command exit: expected {expected}, got {code}')
            return out, err

        try:
            for args in (['--version'], ['--help'], ['task', 'add', '--help'], ['review', '--help'], ['agent', 'apply', '--help']):
                cli(args)
            out, _ = cli(['task', 'add', '--content', 'Capture workflow task'], [])
            for field in ('Created task', 'Content: Saved workflow task', 'Project: Inbox',
                          'Due: 2020-01-01', 'Recurrence: None', 'Priority: 2',
                          'Labels: "fixture"', 'View: todoist task view id:' + TASK_ID):
                require(field in out, 'capture receipt missing: ' + field)
            require(fixture.writes == [('POST', '/tasks')], 'capture mutation count')
            require(len(fixture.tasks) == 1 and not fixture.tasks[TASK_ID]['checked'], 'capture state')
            before = fixture.snapshot()
            out, _ = cli(['task', 'add', '--content', 'Preview only', '--dry-run'], [])
            require('no task created' in out and 'Created task' not in out, 'dry-run receipt')
            require(fixture.snapshot() == before, 'dry run mutated fixture')
            cli(['review'], [(DISPOSITION, 'complete'), (CONFIRM, 'no')])
            require(fixture.snapshot() == before, 'cancelled review mutated fixture')
            plan_path = scratch / 'confirmed-review.json'
            cli(['review', '--out', str(plan_path)], [(DISPOSITION, 'complete'), (CONFIRM, 'yes')])
            require(fixture.tasks[TASK_ID]['checked'], 'confirmed review did not complete task')
            require(fixture.writes == [('POST', '/tasks'), ('POST', '/tasks/' + TASK_ID + '/close')],
                    'confirmed review mutation count')
            plan_bytes = plan_path.read_bytes()
            plan = json.loads(plan_bytes)
            require(plan['confirm_token'] and len(plan['actions']) == 1
                    and plan['actions'][0]['type'] == 'task_complete'
                    and plan['actions'][0]['task_id'] == TASK_ID, 'saved review plan action')
            reviewed = plan['review']['tasks']
            require(len(reviewed) == 1 and reviewed[0]['id'] == TASK_ID
                    and reviewed[0]['disposition'] == 'complete'
                    and reviewed[0]['snapshot']['checked'] is False, 'saved review set and disposition')
            journal = config.parent / 'agent_replay.json'
            evidence = journal.read_bytes()
            before = fixture.snapshot()
            out, err = cli(['--json', 'agent', 'apply', '--plan', str(plan_path), '--confirm', plan['confirm_token']])
            report = json.loads(out)
            require(report['tasks'][0]['actions'][0]['replayed'] is True, 'replay report')
            require(not err, 'successful machine replay wrote stderr')
            require(fixture.snapshot() == before, 'replay duplicated mutation')
            require(journal.read_bytes() == evidence and plan_path.read_bytes() == plan_bytes, 'replay changed saved evidence')
            request_count = len(fixture.requests)
            for args in (['review'], ['review', '--no-input']):
                out, err = cli(args, expected=2)
                require(not out and 'requires terminal input' in err, 'noninteractive refusal output')
                require(fixture.snapshot() == before, 'noninteractive review mutated fixture')
                require(len(fixture.requests) == request_count, 'noninteractive review accessed API')
            print('PASS capture/review/replay: one task, two mutation requests; replay added zero')
        except Exception:
            if diagnostics:
                print(json.dumps({'transcript': transcript, 'fixture': fixture.snapshot(), 'fixture_errors': fixture.errors, 'requests': fixture.requests}, indent=2))
            raise
        finally:
            if verbose:
                print(json.dumps({'transcript': transcript, 'fixture': fixture.snapshot()}, indent=2))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, help='test this binary; otherwise build in a temporary directory')
    parser.add_argument('--verbose', action='store_true', help='print synthetic command transcripts')
    parser.add_argument('--self-test', action='store_true', help='also prove deliberate fixture faults fail the workflow')
    parser.add_argument('--fault', choices=['reject-capture', 'wrong-receipt', 'ignore-completion'], help=argparse.SUPPRESS)
    args = parser.parse_args()
    with tempfile.TemporaryDirectory(prefix='todoist-workflow-build-') as directory:
        binary = args.binary.resolve() if args.binary else Path(directory) / 'todoist'
        if not args.binary:
            subprocess.run(['go', 'build', '-o', str(binary), './cmd/todoist'], cwd=ROOT, check=True, timeout=180)
        exercise(binary, args.fault, args.verbose)
        if args.self_test:
            for fault, expected in [('reject-capture', 'command exit'),
                                    ('wrong-receipt', 'capture receipt missing'),
                                    ('ignore-completion', 'confirmed review did not complete task')]:
                try:
                    exercise(binary, fault, diagnostics=False)
                except CheckFailure as error:
                    require(expected in str(error), f'{fault}: unexpected failure: {error}')
                    print(f'PASS self-test: detected {fault}: {error}')
                else:
                    raise CheckFailure('fault silently passed: ' + fault)


if __name__ == '__main__':
    try:
        main()
    except (CheckFailure, subprocess.SubprocessError, OSError, KeyError, ValueError) as error:
        raise SystemExit('FAIL capture/review/replay: ' + str(error))
