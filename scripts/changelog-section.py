#!/usr/bin/env python3
import runpy
from pathlib import Path
runpy.run_path(str(Path(__file__).resolve().parent / "cli-shared/changelog-section.py"), run_name="__main__")
