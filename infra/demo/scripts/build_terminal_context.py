#!/usr/bin/env python3
"""Assemble only reviewed source and hash-verified native binaries for Docker."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import re

ROOT = Path(__file__).resolve().parents[3]
p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--feam', type=Path, required=True)
p.add_argument('--ttyd', type=Path, required=True)
p.add_argument('--model-adapter', type=Path, required=True)
p.add_argument('--artifact-manifest', type=Path, required=True,
               help='reviewed JSON map of feam, ttyd and model-adapter SHA-256 hashes')
p.add_argument('--output', type=Path, required=True)
a = p.parse_args()
expected = json.loads(a.artifact_manifest.read_text())
if not isinstance(expected, dict) or set(expected) != {'feam', 'ttyd', 'model-adapter'}:
    p.error('artifact manifest must contain exactly feam, ttyd and model-adapter')
for name in expected:
    binary = getattr(a, name.replace('-', '_'))
    if binary.is_symlink() or not binary.is_file():
        p.error(name + ' must be a regular non-symlink artifact')
    data = binary.read_bytes()
    if len(data) < 20 or data[:6] != b'\x7fELF\x02\x01' or data[18:20] != b'\x3e\x00':
        p.error(name + ' must be a native Linux amd64 ELF64 artifact')
    if not isinstance(expected[name], str) or not re.fullmatch('[0-9a-f]{64}', expected[name]):
        p.error(name + ' has an invalid reviewed SHA-256')
    if hashlib.sha256(data).hexdigest() != expected[name]:
        p.error(name + ' does not match the approved native binary')
a.output.mkdir(mode=0o700, parents=True, exist_ok=False)
image = ROOT/'infra/demo/images/terminal'
for source in image.iterdir():
    if source.is_file():
        shutil.copy2(source, a.output/source.name)
for name in expected:
    shutil.copy2(getattr(a, name.replace('-', '_')), a.output/name)
(a.output/'binary.sha256').write_text(''.join(f'{v}  {k}\n' for k,v in expected.items()))
for name in ['python_sdk','scripts','mesh_core/tests/data']:
    shutil.copytree(ROOT/'feather-mesh'/name, a.output/'feather-mesh'/name,
                    ignore=shutil.ignore_patterns('__pycache__', '*.pyc', '.DS_Store', '.venv', 'build', 'dist', '*.egg-info'))
print('created reviewed native Linux terminal build context')
