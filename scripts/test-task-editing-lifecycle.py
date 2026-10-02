#!/usr/bin/env python3
"""Public task editing lifecycle against an isolated synthetic local service.

Run with --binary /path/to/todoist --log /path/to/transcript.md.
This fixture is not a live Todoist verification. It intentionally models only
this lifecycle and checks request counts independently of CLI exit status.
"""
import argparse
import json
import os
from pathlib import Path
import shlex
import subprocess
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import parse_qs, urlsplit


class Fixture:
    def __init__(self):
        self.tasks = {}
        self.writes = []
        self.reads = []
        self.optional_missing = False
        self.reject_rest = False
        self.uncertain_sync = False

    def handler(self):
        fixture = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def send(self, value, status=200):
                data = json.dumps(value).encode()
                self.send_response(status)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            def do_GET(self):
                path = urlsplit(self.path).path
                query = parse_qs(urlsplit(self.path).query)
                fixture.reads.append(self.path)
                if path == '/projects':
                    return self.send({'results': [{'id': 'project', 'name': 'Inbox', 'inbox_project': True}], 'next_cursor': None})
                if path == '/sections':
                    return self.send({'results': [{'id': 'section', 'name': 'Scratch', 'project_id': 'project'}], 'next_cursor': None})
                if path == '/labels':
                    return self.send({'results': [{'id': 'label', 'name': 'work'}], 'next_cursor': None})
                if path.startswith('/tasks/completed/'):
                    return self.send({'items': [t for t in fixture.tasks.values() if t['checked']], 'next_cursor': None})
                if path.startswith('/tasks/'):
                    task = fixture.tasks.get(path.split('/')[2])
                    if task and task['checked']:
                        task = None
                    return self.send(task if task else {'error': 'not found'}, 200 if task else 404)
                if path == '/tasks':
                    tasks = [t for t in fixture.tasks.values() if not t['checked']]
                    if query.get('parent_id'):
                        tasks = [t for t in tasks if t['parent_id'] == query['parent_id'][0]]
                        index = int(query.get('cursor', ['0'])[0])
                        page = tasks[index:index + 1]
                        cursor = str(index + 1) if index + 1 < len(tasks) else None
                    else:
                        page, cursor = tasks, None
                    return self.send({'results': page, 'next_cursor': cursor})
                self.send({'error': 'unexpected route'}, 404)

            def do_POST(self):
                path = urlsplit(self.path).path
                raw = self.rfile.read(int(self.headers.get('Content-Length', 0)))
                request_id = self.headers.get('X-Request-Id')
                assert request_id, 'write missing request ID'
                if path == '/sync':
                    form = parse_qs(raw.decode())
                    commands = json.loads(form['commands'][0])
                    assert form['resource_types'] == ['[]'] and len(commands) == 1
                    command = commands[0]
                    fixture.writes.append({'path': path, 'command': command, 'request_id': request_id})
                    task = fixture.tasks[command['args']['id']]
                    if command['type'] == 'item_update':
                        for key, value in command['args'].items():
                            if key != 'id':
                                task[key] = value
                    elif command['type'] == 'item_move':
                        task['parent_id'] = None
                        task['section_id'] = command['args'].get('section_id')
                    elif command['type'] == 'item_complete':
                        task['checked'] = True
                        task['completed_at'] = '2026-10-01T09:00:00Z'
                        for descendant in fixture.tasks.values():
                            parent = descendant['parent_id']
                            while parent:
                                if parent == task['id']:
                                    descendant['checked'] = True
                                    break
                                parent = fixture.tasks[parent]['parent_id']
                    else:
                        raise AssertionError(command)
                    status = {} if fixture.uncertain_sync else 'ok'
                    return self.send({'sync_status': {command['uuid']: status}, 'items': [] if fixture.optional_missing else [task]})
                body = json.loads(raw)
                fixture.writes.append({'path': path, 'body': body, 'request_id': request_id})
                if fixture.reject_rest:
                    return self.send({'error': 'rejected'}, 403)
                if path == '/tasks':
                    task_id = 'scratch' + str(len(fixture.tasks) + 1)
                    parent = fixture.tasks.get(body.get('parent_id'))
                    task = {'id': task_id, 'content': body['content'], 'description': body.get('description', ''), 'project_id': parent['project_id'] if parent else body.get('project_id', 'project'), 'section_id': parent['section_id'] if parent else body.get('section_id'), 'parent_id': body.get('parent_id'), 'labels': body.get('labels', []), 'priority': body.get('priority', 1), 'child_order': body.get('order', 0), 'checked': False, 'due': None, 'deadline': None, 'responsible_uid': body.get('assignee_id'), 'added_at': '2026-10-01T08:00:00Z'}
                    if body.get('due_string'):
                        task['due'] = {'date': '2026-10-01', 'timezone': None, 'is_recurring': True, 'string': body['due_string'], 'lang': body.get('due_lang', 'en')}
                    if body.get('deadline_date'):
                        task['deadline'] = {'date': body['deadline_date']}
                    fixture.tasks[task_id] = task
                else:
                    task = fixture.tasks[path.split('/')[2]]
                    for key, value in body.items():
                        if key == 'deadline_date':
                            task['deadline'] = {'date': value} if value else None
                        elif key == 'assignee_id':
                            task['responsible_uid'] = value
                        else:
                            task[key] = value
                self.send({} if fixture.optional_missing else task)
        return Handler


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True)
    parser.add_argument('--log', required=True)
    args = parser.parse_args()
    fixture = Fixture()
    server = HTTPServer(('127.0.0.1', 0), fixture.handler())
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    transcript = ['# Task editing consumer execution', '', 'Synthetic local fixture; isolated configuration; noninteractive execution.', 'Implementation knowledge is disclosed. This is not live Todoist verification.', 'Public route: README editing examples → focused task help → lifecycle → exact-state inspection.', '']
    with tempfile.TemporaryDirectory(prefix='todoist-editing-consumer-') as scratch:
        env = dict(os.environ)
        for key in [key for key in env if key.startswith('TODOIST_')]:
            env.pop(key, None)
        env.update(TODOIST_TOKEN='synthetic-consumer-token', TODOIST_CONFIG=str(Path(scratch) / 'config.json'), TODOIST_BASE_URL=f'http://127.0.0.1:{server.server_port}')

        def run(*words, stdin=None, expected=0, writes=0, machine=True):
            command = [str(Path(args.binary).resolve()), *words]
            if machine:
                command += ['--no-input', '--json']
            before = len(fixture.writes)
            result = subprocess.run(command, input=stdin, text=True, capture_output=True, env=env, cwd=scratch, timeout=10)
            delta = len(fixture.writes) - before
            transcript.extend(['```text', '$ todoist ' + shlex.join(command[1:]), f'exit={result.returncode} writes={delta}', 'stdout:', result.stdout.rstrip(), 'stderr:', result.stderr.rstrip(), '```', ''])
            Path(args.log).write_text('\n'.join(transcript))
            assert result.returncode == expected, (command, result.stdout, result.stderr)
            assert delta == writes, (command, delta, writes)
            if expected == 0:
                assert not result.stderr, result.stderr
            elif machine:
                json.loads(result.stderr)
            return json.loads(result.stdout) if machine and result.stdout else result.stdout

        run('task', 'add', '--help', machine=False)
        run('task', 'view', '--help', machine=False)
        run('task', 'reschedule', '--help', machine=False)
        run('task', 'update', '--help', machine=False)
        run('task', 'move', '--help', machine=False)
        run('task', 'complete', '--help', machine=False)
        parent = run('task', 'add', '--content', 'Consumer parent', '--project', 'id:project', '--section', 'id:section', writes=1)[0]['id']
        recurring = run('task', 'add', '--content', 'Consumer recurring child', '--parent', parent, '--due', 'every day', '--due-lang', 'en', '--description', '-', '--label', 'work', '--deadline', '2026-10-31', '--assignee', 'id:owner', stdin='Exact notes\n', writes=1)[0]['id']
        grandchild = run('task', 'add', '--content', 'Consumer grandchild', '--parent', recurring, writes=1)[0]['id']
        sibling = run('task', 'add', '--content', 'Consumer sibling', '--parent', parent, '--order', '2', writes=1)[0]['id']
        task = run('task', 'view', '--id', recurring, '--task-output-version', '2')
        assert task['description'] == 'Exact notes\n' and task['due']['is_recurring']
        view = run('task', 'view', '--id', parent, '--include-children', '--sort', 'order', '--task-output-version', '2')
        assert view['children_complete'] and [t['id'] for t in view['children']] == [recurring, sibling]
        assert grandchild not in [t['id'] for t in view['children']]
        assert len([q for q in fixture.reads if 'parent_id=' in q]) == 2
        run('task', 'reschedule', '--id', recurring, '--due-date', '2026-10-15', writes=1)
        task = run('task', 'view', '--id', recurring, '--task-output-version', '2')
        assert task['due'] == {'date': '2026-10-15', 'timezone': None, 'is_recurring': True, 'string': 'every day', 'lang': 'en'}
        run('task', 'update', '--id', recurring, '--clear-labels', '--clear-assignee', '--clear-description', '--clear-deadline', writes=1)
        run('task', 'move', '--id', recurring, '--clear-parent', '--clear-section', writes=1)
        task = run('task', 'view', '--id', recurring, '--task-output-version', '2')
        assert task['parent_id'] is None and task['section_id'] is None and task['project_id'] == 'project'
        assert task['description'] == '' and task['labels'] == [] and task['deadline'] is None and task['responsible_uid'] is None
        run('task', 'update', '--id', recurring, '--reference=true', '--order', '0', writes=1)
        task = run('task', 'view', '--id', recurring, '--task-output-version', '2')
        assert task['content'] == '* Consumer recurring child' and task['child_order'] == 0 and task['reference_item']['is_reference']
        run('task', 'update', '--id', recurring, '--reference=false', writes=1)
        run('task', 'complete', '--id', recurring, '--forever', writes=1)
        run('task', 'view', '--id', recurring, expected=4)
        history = run('completed', '--since', '2026-10-01', '--all', '--task-output-version', '2')
        task = next(t for t in history if t['id'] == recurring)
        assert task['checked'] and task['due']['is_recurring'] and fixture.tasks[grandchild]['checked']
        due_task = run('task', 'add', '--content', 'Separate due clearing', '--due', 'every day', writes=1)[0]['id']
        run('task', 'update', '--id', due_task, '--clear-due', writes=1)
        assert run('task', 'view', '--id', due_task, '--task-output-version', '2')['due'] is None
        assert len(fixture.writes) == 12
        run('task', 'update', '--id', due_task, '--clear-due', '--due', 'today', expected=2)
        run('task', 'view', '--id', 'missing', expected=4)
        run('task', 'move', 'Consumer', '--clear-parent', expected=2)
        run('task', 'update', '--id', due_task, '--reference=false')
        run('task', 'reschedule', '--id', due_task, '--due-date', '2026-10-16', expected=2)
        run('task', 'update', '--id', due_task, '--description', 'preview', '--dry-run')
        fixture.optional_missing = True
        ack = run('task', 'update', '--id', due_task, '--description', 'accepted', '--task-output-version', '2', writes=1)
        assert ack == {'id': due_task, 'status': 'accepted', 'operation': 'task_update', 'result_available': False}
        fixture.optional_missing = False
        fixture.reject_rest = True
        partial = run('task', 'update', '--id', due_task, '--clear-due', '--description', 'rejected', expected=1, writes=2)
        assert [step['outcome'] for step in partial['steps']] == ['accepted', 'rejected']
        fixture.reject_rest = False
        fixture.uncertain_sync = True
        run('task', 'complete', '--id', sibling, '--forever', expected=1, writes=1)
        fixture.uncertain_sync = False
        run('task', 'view', '--id', sibling, expected=4)
        history = run('completed', '--since', '2026-10-01', '--all', '--task-output-version', '2')
        assert next(t for t in history if t['id'] == sibling)['checked']
        assert len(fixture.writes) == 16
        transcript.extend([f'PASS: lifecycle 12 writes; relevant recovery 4 writes; total {len(fixture.writes)}.', 'All asserted invalid/no-op/dry-run paths dispatched zero writes.', 'All success stderr streams empty; failure stderr structured and parseable.', ''])
    server.shutdown()
    Path(args.log).write_text('\n'.join(transcript))
    print(f'PASS: public fixture lifecycle and recovery; transcript: {args.log}')


if __name__ == '__main__':
    main()
