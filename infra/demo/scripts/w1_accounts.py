#!/usr/bin/env python3
"""Reject unexpected existing FEAM identities, service scope or sub-ID overlaps."""
import grp
import os
from pathlib import Path
import pwd

root = Path('/home/feam-service-data')
if root.is_symlink() or root.exists() and root.stat().st_uid != 0:
    raise SystemExit('unexpected service root')
if root.exists() and ((root/'.feam-w1-owner').is_symlink() or not (root/'.feam-w1-owner').is_file() or (root/'.feam-w1-owner').read_text() != 'feam-w1-storage-v1\n'):
    raise SystemExit('existing service root has no matching owner marker')
for index, name in enumerate(('runner', 'gateway', 'controller', 'broker', 'collector', 'pipeline', 'reconciler'), 2101):
    expected = 'feam-' + name
    try:
        if grp.getgrnam(expected).gr_gid != index:
            raise SystemExit('existing service group differs')
    except KeyError:
        pass
    try:
        if grp.getgrgid(index).gr_name != expected:
            raise SystemExit('service GID collision')
    except KeyError:
        pass
    try:
        entry = pwd.getpwnam(expected)
    except KeyError:
        entry = None
    if entry and (entry.pw_uid != index or entry.pw_dir != str(root/(name+'-home')) or entry.pw_shell != '/usr/sbin/nologin'):
        raise SystemExit('existing FEAM account differs from reviewed mapping')
    try:
        if pwd.getpwuid(index).pw_name != expected:
            raise SystemExit('service UID collision')
    except KeyError:
        pass
for filename in ('/etc/subuid', '/etc/subgid'):
    for line in Path(filename).read_text().splitlines():
        if not line.strip():
            continue
        name, start, count = line.split(':')
        start, count = int(start), int(count)
        if name == 'feam-runner':
            if (start,count) != (200000,65536):
                raise SystemExit('runner subordinate range differs')
        elif start < 265536 and start+count > 200000:
            raise SystemExit('subordinate ID overlap')
print('account and subordinate mappings available or identical')
