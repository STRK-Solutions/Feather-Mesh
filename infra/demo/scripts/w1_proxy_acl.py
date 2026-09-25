#!/usr/bin/env python3
"""Grant the named SSH operator access only to the private W1 frontend socket."""
import json
import os
from pathlib import Path
import pwd
import stat
import subprocess
import time

config=Path('/etc/feam/w1-proxy-operator.json')
assert os.geteuid()==0 and not config.is_symlink() and config.stat().st_uid==0
operator=json.loads(config.read_text())['operator_uid']
assert type(operator) is int and operator>=1000 and operator not in range(2101,2108)
pwd.getpwuid(operator)
root=Path('/run/feam-w1-proxy');sock=root/'http.sock'
assert not root.is_symlink() and root.stat().st_uid==2102
subprocess.run(['setfacl','-m',f'u:{operator}:x',str(root)],check=True)
for attempt in range(200):
    if sock.exists():break
    time.sleep(.05)
else:raise SystemExit('frontend socket did not appear')
assert stat.S_ISSOCK(sock.lstat().st_mode) and sock.stat().st_uid==2102
subprocess.run(['setfacl','-m',f'u:{operator}:rw',str(sock)],check=True)
