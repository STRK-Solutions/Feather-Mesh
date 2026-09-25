#!/usr/bin/env python3
"""Create a private, hash-verified owner bootstrap bundle from reviewed W1 files."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import tarfile
import tempfile

ROOT=Path(__file__).resolve().parents[3]

def digest(path):
    with path.open('rb') as source:return hashlib.file_digest(source,'sha256').hexdigest()

p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--image',type=Path,required=True)
p.add_argument('--proxy',type=Path,required=True)
p.add_argument('--operator-json',type=Path,required=True)
p.add_argument('--output',type=Path,required=True)
a=p.parse_args()
pins=json.loads((ROOT/'docs/evaluations/web-demo/w1-artifacts.json').read_text())
if digest(a.image)!=pins['image_archive_sha256'] or digest(a.proxy)!=pins['native_proxy_sha256']:
    p.error('native artifact does not match the tested W1 pins')
config=json.loads(a.operator_json.read_text())
required={'feam_parent_uuid','feam_operator_uid','feam_image_id','feam_proxy_sha256','ansible_bin','operator_output_dir'}
if set(config)!=required or config['feam_image_id']!=pins['rootless_image_id'] or config['feam_proxy_sha256']!=pins['native_proxy_sha256']:
    p.error('operator inputs must bind the exact tested artifacts')
if a.output.exists():p.error('retain old bundles; choose a new output')
with tempfile.TemporaryDirectory(prefix='feam-w1-bundle-') as temporary:
    source=Path(temporary)/'source';source.mkdir(mode=0o700)
    paths=[ROOT/'infra/demo/ansible/host.yml',ROOT/'infra/demo/ansible/deploy-w1.yml']
    paths+=list((ROOT/'infra/demo/ansible/roles').rglob('*.yml'))
    paths+=list((ROOT/'infra/demo/ansible/roles').rglob('*.j2'))
    paths+=list((ROOT/'infra/demo/scripts').glob('w1_*.py'))
    paths+=[ROOT/'infra/demo/scripts/owner_bootstrap_w1.sh']
    paths+=list((ROOT/'infra/demo/tests/w1').glob('*.py'))
    paths+=list((ROOT/'infra/demo/tests/w1').glob('*.sh'))
    for original in paths:
        if original.is_symlink() or not original.is_file():p.error('source must be a regular file')
        target=source/original.relative_to(ROOT);target.parent.mkdir(parents=True,exist_ok=True)
        shutil.copy2(original,target)
    for original,name in [(a.image,'terminal-image.tar'),(a.proxy,'private-terminal'),(a.operator_json,'operator.json')]:
        shutil.copyfile(original,source/name)
    manifest=source/'artifacts.sha256'
    manifest.write_text(''.join(f'{digest(path)}  {path.relative_to(source)}\n' for path in sorted(source.rglob('*')) if path.is_file()))
    with tarfile.open(a.output,'x') as bundle:bundle.add(source,arcname='source')
a.output.chmod(0o600)
print('bundle_sha256='+digest(a.output))
print('bundle_bytes='+str(a.output.stat().st_size))
