#!/usr/bin/env python3
"""Assemble only reviewed source and hash-verified native binaries for Docker."""
import argparse
import hashlib
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--feam', type=Path, required=True)
p.add_argument('--ttyd', type=Path, required=True)
p.add_argument('--output', type=Path, required=True)
a = p.parse_args()
expected = {'feam': '6d21243cdac07ddf60fc92aae1b79e6e39e80c16444afca444ae2a9824db50c3',
            'ttyd': '8a217c968aba172e0dbf3f34447218dc015bc4d5e59bf51db2f2cd12b7be4f55'}
for name in expected:
    if hashlib.sha256(getattr(a,name).read_bytes()).hexdigest() != expected[name]:
        p.error(name + ' does not match the approved native binary')
a.output.mkdir(mode=0o700, parents=True, exist_ok=False)
image = ROOT/'infra/demo/images/terminal'
for source in image.iterdir():
    if source.is_file():
        shutil.copy2(source, a.output/source.name)
for name in expected:
    shutil.copy2(getattr(a,name), a.output/name)
(a.output/'binary.sha256').write_text(''.join(f'{v}  {k}\n' for k,v in expected.items()))
for name in ['python_sdk','scripts','mesh_core/tests/data']:
    shutil.copytree(ROOT/'feather-mesh'/name, a.output/'feather-mesh'/name,
                    ignore=shutil.ignore_patterns('__pycache__', '*.pyc', '.DS_Store', '.venv', 'build', 'dist', '*.egg-info'))
print('created reviewed native Linux terminal build context')
