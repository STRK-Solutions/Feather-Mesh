#!/usr/bin/env python3
"""Reconcile a retained linger file after logind has forgotten the QEMU runner."""
import json
import os
from pathlib import Path
import subprocess
import sys
import time

if os.geteuid() != 0 or sys.argv[1:] != ['--confirm-disposable']:
    raise SystemExit('explicit disposable root invocation required')
if (subprocess.check_output(['systemd-detect-virt', '--vm'], text=True).strip() != 'qemu'
        or Path('/etc/hostname').read_text().strip() != 'feam-w0-disposable'):
    raise SystemExit('refusing non-disposable host')
root=Path('/home/feam-service-data')
state=(root/'w1-terminal-state.json').read_bytes()
inventory=(root/'w1-storage.json').read_bytes()
if subprocess.run(['loginctl','show-user','feam-runner'],capture_output=True).returncode == 0:
    subprocess.run(['loginctl', 'terminate-user', 'feam-runner'], check=True)
try:
    for _ in range(30):
        forgotten=subprocess.run(['loginctl','show-user','feam-runner'],capture_output=True).returncode != 0
        if forgotten: break
        time.sleep(1)
    assert forgotten
    assert Path('/var/lib/systemd/linger/feam-runner').is_file()
finally:
    subprocess.run(['loginctl','enable-linger','feam-runner'],check=True)
    subprocess.run(['systemctl','start','user@2101.service'],check=True)
    ready=False
    for _ in range(60):
        result=subprocess.run(['runuser','-u','feam-runner','--','env',
            'XDG_RUNTIME_DIR=/run/user/2101','DOCKER_HOST=unix:///run/user/2101/feam-docker.sock',
            'docker','info'],capture_output=True)
        if result.returncode==0:
            ready=True
            break
        time.sleep(1)
    assert ready
    subprocess.run(['/usr/local/sbin/feam-w1-terminal','start'],check=True)
    subprocess.run(['systemctl','start','feam-w1-private-terminal.service'],check=True)
live=subprocess.check_output(['loginctl','show-user','feam-runner','--property=Linger','--property=Sessions'],text=True)
assert 'Linger=yes' in live.splitlines() and 'Sessions=' in live.splitlines()
assert (root/'w1-terminal-state.json').read_bytes()==state
assert (root/'w1-storage.json').read_bytes()==inventory
print(json.dumps({'retained_file_with_missing_registration':True,'enable_linger_repairs_registration':True,
    'runner_ready_without_login':True,'terminal_generation_and_storage_preserved':True}))
