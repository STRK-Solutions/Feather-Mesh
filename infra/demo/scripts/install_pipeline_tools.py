#!/usr/bin/env python3
"""Install a reviewed converter and hash-locked wheels in a versioned root scope."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import sys

ROOT = Path('/opt/feam/pipeline-releases')
LINKS = {Path('/opt/feam/pipeline'): 'venv', Path('/opt/feam/importers/convert.py'): 'convert.py'}


def checked_source(path, digest):
    if not re.fullmatch('[0-9a-f]{64}', digest):
        raise ValueError('invalid reviewed digest')
    if path.is_symlink() or not path.is_file() or path.stat().st_size > 2 * 1024**2:
        raise ValueError('bounded regular source required')
    content = path.read_bytes()
    if hashlib.sha256(content).hexdigest() != digest:
        raise ValueError('source differs from reviewed digest')
    return content


def private_root(path):
    for parent in reversed([path, *path.parents]):
        if parent.is_symlink():
            raise ValueError('symlink installation ancestor')
        if parent.exists() and (not parent.is_dir() or parent.stat().st_uid != 0 or parent.stat().st_mode & 0o022):
            raise ValueError('unsafe installation ancestor')
    path.mkdir(parents=True, exist_ok=True, mode=0o755)


def archive_incomplete_venv(destination, sources):
    """Preserve the exact failed ensurepip attempt before a deliberate retry."""
    archive = destination.with_name(destination.name + '.incomplete-ensurepip')
    if archive.exists() or archive.is_symlink():
        raise ValueError('incomplete installation archive already exists')
    if destination.is_symlink() or not destination.is_dir() or destination.stat().st_uid != 0 or destination.stat().st_mode & 0o022:
        raise ValueError('unsafe incomplete installation')
    if (destination / 'installed.json').exists() or (destination / 'installed.json').is_symlink():
        raise ValueError('installation receipt exists')
    if set(p.name for p in destination.iterdir()) != {*sources, 'venv'}:
        raise ValueError('unexpected incomplete installation contents')
    for name, content in sources.items():
        path = destination / name
        if path.is_symlink() or not path.is_file() or path.stat().st_uid != 0 or path.read_bytes() != content:
            raise ValueError('incomplete installation source changed')
    venv = destination / 'venv'
    if venv.is_symlink() or not venv.is_dir() or venv.stat().st_uid != 0 or not (venv / 'pyvenv.cfg').is_file() or (venv / 'bin/pip').exists():
        raise ValueError('not the recorded missing-ensurepip attempt')
    destination.rename(archive)
    fd = os.open(ROOT, os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)
    return archive


def install(lock, lock_sha, converter, converter_sha, reconcile_missing_ensurepip=False):
    if os.geteuid() != 0 or sys.platform != 'linux' or sys.version_info[:2] != (3, 12) or platform.machine() != 'x86_64':
        raise ValueError('native Ubuntu Python 3.12 amd64 operator root required')
    sources = {'requirements.lock': checked_source(lock, lock_sha), 'convert.py': checked_source(converter, converter_sha)}
    identity = {'lock_sha256': lock_sha, 'converter_sha256': converter_sha, 'python': '3.12', 'machine': 'x86_64'}
    encoded = json.dumps(identity, sort_keys=True, separators=(',', ':')).encode()
    release_sha = hashlib.sha256(encoded).hexdigest()
    private_root(ROOT)
    destination = ROOT / release_sha
    # Reject foreign paths before downloads, or any change to current links.
    for link, suffix in LINKS.items():
        private_root(link.parent)
        if link.is_symlink():
            target = Path(os.readlink(link))
            if not target.is_absolute() or target.parent.parent != ROOT or target.name != suffix:
                raise ValueError('unowned current installation link')
            previous = target.parent / 'installed.json'
            if previous.is_symlink() or not previous.is_file() or previous.stat().st_uid != 0:
                raise ValueError('unrecorded previous installation')
        elif link.exists():
            raise ValueError('refuse existing non-link installation')
    receipt = destination / 'installed.json'
    changed = False
    archived = None
    if reconcile_missing_ensurepip:
        if not destination.exists() or destination.is_symlink():
            raise ValueError('recorded incomplete installation unavailable')
        archived = archive_incomplete_venv(destination, sources)
    if destination.exists() or destination.is_symlink():
        if destination.is_symlink() or destination.stat().st_uid != 0 or destination.stat().st_mode & 0o022 or not receipt.is_file() or receipt.is_symlink() or receipt.stat().st_uid != 0 or receipt.read_bytes() != encoded:
            raise ValueError('incomplete or different installation; preserve for explicit reconciliation')
        for name, content in sources.items():
            if (destination / name).is_symlink() or (destination / name).read_bytes() != content:
                raise ValueError('installed source changed')
    else:
        changed = True
        destination.mkdir(mode=0o755)
        for name, content in sources.items():
            (destination / name).write_bytes(content)
            (destination / name).chmod(0o444)
        subprocess.run([sys.executable, '-m', 'venv', str(destination / 'venv')], check=True, timeout=60)
        python = destination / 'venv/bin/python'
        subprocess.run([str(python), '-m', 'pip', '--isolated', 'install', '--index-url', 'https://pypi.org/simple', '--disable-pip-version-check', '--no-cache-dir', '--only-binary=:all:', '--require-hashes', '-r', str(destination / 'requirements.lock')], check=True, timeout=600)
        subprocess.run([str(python), '-c', 'import polars, rasterio, numpy, jsonschema; print("native pipeline readers available")'], check=True, timeout=30)
        with receipt.open('xb') as output:
            output.write(encoded)
            output.flush()
            os.fsync(output.fileno())
        receipt.chmod(0o444)
        fd = os.open(destination, os.O_DIRECTORY)
        try:
            os.fsync(fd)
        finally:
            os.close(fd)
    for link, suffix in LINKS.items():
        target = destination / suffix
        if link.is_symlink() and Path(os.readlink(link)) == target:
            continue
        pending = link.with_name(link.name + '.new')
        pending.symlink_to(target)
        os.replace(pending, link)
        changed = True
        fd = os.open(link.parent, os.O_DIRECTORY)
        try:
            os.fsync(fd)
        finally:
            os.close(fd)
    print(json.dumps({'protocol': 'feam.pipeline-tools.v1', 'release_sha256': release_sha, 'changed': changed, 'archived_incomplete': str(archived) if archived else None, **identity}))


if __name__ == '__main__':
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--lock', type=Path, required=True)
    p.add_argument('--lock-sha256', required=True)
    p.add_argument('--converter', type=Path, required=True)
    p.add_argument('--converter-sha256', required=True)
    p.add_argument('--reconcile-missing-ensurepip', action='store_true',
                   help='preserve the verified partial venv and retry the same reviewed inputs')
    a = p.parse_args()
    install(a.lock, a.lock_sha256, a.converter, a.converter_sha256, a.reconcile_missing_ensurepip)
