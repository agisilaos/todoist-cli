#!/usr/bin/env python3
"""Verify secret input and terminal restoration against an isolated CLI build."""
import json
import os
from pathlib import Path
import pty
import select
import subprocess
import tempfile
import termios
import time

ROOT = Path(__file__).resolve().parent.parent


def check_login(binary, scratch, name, token, input_bytes):
    config = scratch / name / 'config.json'
    config.parent.mkdir()
    env = {k: v for k, v in os.environ.items() if not k.startswith('TODOIST_')}
    env.update(HOME=str(scratch), XDG_CONFIG_HOME=str(scratch), NO_COLOR='1')
    master, slave = pty.openpty()
    initial = termios.tcgetattr(slave)
    process = subprocess.Popen(
        [str(binary), '--config', str(config), 'auth', 'login', '--credential-store=file'],
        stdin=slave, stdout=slave, stderr=slave, cwd=scratch, env=env,
    )
    captured = b''
    sent = False
    try:
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline:
            if select.select([master], [], [], 0.05)[0]:
                captured += os.read(master, 65536)
            if b'Todoist API token:' in captured and not sent:
                # Wait for the terminal reader to disable echo before simulating typing.
                # Still send on older builds so the regression fails on visible input.
                for _ in range(20):
                    if not termios.tcgetattr(slave)[3] & termios.ECHO:
                        break
                    time.sleep(0.01)
                os.write(master, input_bytes)
                sent = True
            if process.poll() is not None:
                while select.select([master], [], [], 0)[0]:
                    captured += os.read(master, 65536)
                break
        if process.poll() is None:
            raise AssertionError('login did not finish')
        assert sent, 'login did not prompt'
        restored = termios.tcgetattr(slave)
        # macOS may set its transient pending-input flag when canonical mode resumes.
        initial[3] &= ~getattr(termios, 'PENDIN', 0)
        restored[3] &= ~getattr(termios, 'PENDIN', 0)
        assert restored == initial, 'terminal settings were not restored'
        credential_path = config.parent / 'credentials.json'
        if token:
            assert token not in captured, 'login echoed the secret'
            assert process.returncode == 0, 'login failed'
            credential = json.loads(credential_path.read_text())['profiles']['default']
            assert credential['token'] == token.decode(), 'login did not store the supplied token'
        else:
            assert process.returncode != 0, 'empty terminal input unexpectedly succeeded'
            assert not credential_path.exists(), 'empty input created credentials'
    finally:
        if process.poll() is None:
            process.kill()
        process.wait()
        os.close(master)
        os.close(slave)


with tempfile.TemporaryDirectory(prefix='todoist-terminal-test-') as directory:
    scratch = Path(directory)
    binary = scratch / 'todoist'
    subprocess.run(['go', 'build', '-o', str(binary), './cmd/todoist'],
                   cwd=ROOT, env=dict(os.environ, CGO_ENABLED='0'), check=True)
    check_login(binary, scratch, 'success', b'synthetic-terminal-test-token', b'synthetic-terminal-test-token\r')
    check_login(binary, scratch, 'empty', b'', b'\r')
    check_login(binary, scratch, 'eof', b'', b'\x04')
    check_login(binary, scratch, 'cancel', b'', b'\x03')
print('auth terminal checks passed')
