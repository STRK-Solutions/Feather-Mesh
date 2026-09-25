#!/usr/bin/env python3
"""Offline W0 checks; never contacts an inventory or executes the VM probe."""
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[3]


def main():
    with tempfile.TemporaryDirectory(prefix="feam-w0-check-") as tmp:
        env = dict(os.environ, ANSIBLE_HOME=str(Path(tmp) / "ansible-home"),
                   ANSIBLE_LOCAL_TEMP=str(Path(tmp) / "ansible-tmp"),
                   XDG_CACHE_HOME=str(Path(tmp) / "cache"))
        commands = [
            [sys.executable, "-m", "unittest", "discover", "-s", "web_demo/tests", "-v"],
            ["bash", "-n", "infra/demo/tests/full_vm_probe.sh"],
            ["bash", "-n", "infra/demo/scripts/native_build.sh"],
            ["ansible-playbook", "--syntax-check", "-i",
             "infra/demo/ansible/inventories/example.yml", "infra/demo/ansible/preflight.yml"],
            ["ansible-lint", "--offline", "infra/demo/ansible/preflight.yml"],
        ]
        for command in commands:
            if shutil.which(command[0]) is None:
                raise SystemExit(f"missing required tool: {command[0]}; install requirements-dev.lock")
            print("+ " + " ".join(command), flush=True)
            subprocess.run(command, cwd=ROOT, env=env, check=True)
        for path in sorted((ROOT / "infra/demo/examples").glob("*.json")):
            subprocess.run([sys.executable, "web_demo/validate.py", path.stem, str(path)],
                           cwd=ROOT, env=env, check=True)
    print("PASS: W0 offline checks. LINUX full-VM and UBUNTU runtime gates are separate.")


if __name__ == "__main__":
    main()
