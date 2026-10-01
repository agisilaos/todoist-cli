#!/usr/bin/env python3
"""Common docs contract plus Todoist's existing command-quoting check."""
import argparse
from pathlib import Path
import shlex
import subprocess
import sys

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--root', default='.')
args = parser.parse_args()
shared = Path(__file__).resolve().parent / 'cli-shared/docs-contract-check.py'
result = subprocess.run([sys.executable, str(shared), '--root', args.root])
if result.returncode:
    raise SystemExit(result.returncode)
for number, line in enumerate((Path(args.root) / 'README.md').read_text().splitlines(), 1):
    if line.strip().startswith('todoist '):
        try:
            shlex.split(line.strip())
        except ValueError as exc:
            print(f'error: README.md:{number}: invalid command example: {exc}', file=sys.stderr)
            raise SystemExit(1)
