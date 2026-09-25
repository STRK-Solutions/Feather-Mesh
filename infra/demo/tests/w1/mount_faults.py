#!/usr/bin/env python3
"""Destructive mount fault fixture: only the named disposable W0/W1 QEMU VM."""
import json
import os
from pathlib import Path
import subprocess
import sys

if os.geteuid()!=0 or sys.argv[1:]!=['--confirm-disposable']:
    raise SystemExit('explicit disposable root invocation required')
if subprocess.check_output(['systemd-detect-virt','--vm'],text=True).strip()!='qemu' or Path('/etc/hostname').read_text().strip()!='feam-w0-disposable':
    raise SystemExit('refusing non-disposable host')
root=Path('/home/feam-service-data')
rec=json.loads((root/'w1-storage.json').read_text())
assert rec['scope']=='feam-w1-storage-v1'
parent=subprocess.check_output(['findmnt','-n','-o','UUID','--target','/home'],text=True).strip()
verify=['/usr/local/sbin/feam-w1-storage','verify','--parent-uuid',parent]
subprocess.run(verify,check=True)
unit=r'home-feam\x2dservice\x2ddata-slot\x2d01.mount'
backing=root/'slot-01.ext4'; held=root/'slot-01.ext4.fault-held'
assert not held.exists()
namespace=Path('/proc/sys/kernel/apparmor_restrict_unprivileged_userns').read_text()
subprocess.run(['systemctl','stop',unit],check=True)
assert subprocess.run(['systemctl','is-active','user@2101.service'],capture_output=True).returncode!=0
try:
    assert subprocess.run(verify,capture_output=True).returncode!=0
    attempted=subprocess.run(['runuser','-u','feam-runner','--','touch',str(root/'slot-01'/'underlying-write')],capture_output=True)
    assert attempted.returncode!=0
    assert not (root/'slot-01'/'underlying-write').exists()
    backing.rename(held)
    try:
        assert subprocess.run(['systemctl','start','user@2101.service'],capture_output=True).returncode!=0
        assert not os.path.ismount(root/'slot-01')
    finally:
        held.rename(backing)
    subprocess.run(['systemctl','reset-failed',unit,'user@2101.service'],check=True)
    subprocess.run(['mount','-t','tmpfs','-o','size=1m,nosuid,nodev,noexec','tmpfs',str(root/'slot-01')],check=True)
    try:
        assert subprocess.run(verify,capture_output=True).returncode!=0
    finally:
        subprocess.run(['umount',str(root/'slot-01')],check=True)
finally:
    subprocess.run(['systemctl','start','user@2101.service'],check=True)
subprocess.run(verify,check=True)
assert Path('/proc/sys/kernel/apparmor_restrict_unprivileged_userns').read_text()==namespace
assert json.loads((root/'w1-storage.json').read_text())==rec
print(json.dumps({'mount_loss_stops_runner':True,'missing_mount_denied':True,'wrong_mount_denied':True,'underlying_write_denied':True,'runner_start_without_login':True,'filesystem_inventory_unchanged':True,'namespace_policy_unchanged':True}))
