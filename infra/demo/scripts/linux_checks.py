#!/usr/bin/env python3
"""Bootstrap offline contract checks inside the owned W0 scope, without host pip."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import urllib.request

PIP_URL = "https://files.pythonhosted.org/packages/f3/6e/1736e5b4ae2b778ef2f81c47d797de9f891d4d8acb047a24ca37a60294dd/pip-26.2.1-py3-none-any.whl"
PIP_SHA = "71138adf1f4ca900cdb7d289c21b7494329f2332b6d85f0e1c42108c0384ed3e"


def main():
    root = Path.home() / "feam-w0-vm"
    if root.is_symlink() or root.stat().st_uid != os.getuid():
        raise SystemExit("unowned W0 directory")
    if json.loads((root / "ownership.json").read_text()) != {"scope": "feam-w0-disposable-v1", "uid": os.getuid()}:
        raise SystemExit("unknown W0 scope")
    source = Path(__file__).resolve().parents[3]
    venv = root / "native-build/venv"
    subprocess.run(["python3", "-m", "venv", "--without-pip", str(venv)], check=True)
    wheel = root / "native-build/pip-26.2.1-py3-none-any.whl"
    if not wheel.exists():
        with urllib.request.urlopen(PIP_URL, timeout=60) as response:
            payload = response.read(4 * 1024**2)
        if hashlib.sha256(payload).hexdigest() != PIP_SHA:
            raise SystemExit("pip wheel checksum mismatch")
        wheel.write_bytes(payload)
    if hashlib.sha256(wheel.read_bytes()).hexdigest() != PIP_SHA:
        raise SystemExit("pip wheel checksum mismatch")
    env = dict(os.environ, PYTHONPATH=str(wheel), PIP_DISABLE_PIP_VERSION_CHECK="1",
               PIP_NO_CACHE_DIR="1", PATH=str(venv / "bin") + os.pathsep + os.environ["PATH"])
    python = str(venv / "bin/python")
    subprocess.run([python, "-m", "pip", "install", "--no-index", str(wheel)], env=env, check=True)
    env.pop("PYTHONPATH")
    subprocess.run([python, "-m", "pip", "install", "-r", str(source / "infra/demo/requirements-dev.lock")],
                   env=env, check=True)
    subprocess.run([python, str(source / "infra/demo/scripts/check.py")], cwd=source, env=env, check=True)


if __name__ == "__main__":
    main()
