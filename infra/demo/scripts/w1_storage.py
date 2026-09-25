#!/usr/bin/env python3
"""W1 finite owned ext4 inventory. Root/operator only; never a website API."""
import argparse
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import uuid

SCOPE = 'feam-w1-storage-v1'
ROOT = Path('/home/feam-service-data')


def run(*args):
    return subprocess.check_output(args, text=True).strip()


def regular(path):
    s = path.lstat()
    if not stat.S_ISREG(s.st_mode) or s.st_uid != 0 or s.st_nlink != 1:
        raise ValueError('unowned or aliased backing file')
    return s


def initialized(path):
    # Lazy inode-table zeroing can punch holes through a loop device after
    # mkfs -E nodiscard has returned. Never admit a reservation still subject
    # to that background work. Older owned volumes may finish it in place.
    result = subprocess.run(['dumpe2fs', str(path)], check=False, text=True,
                            capture_output=True, env={**os.environ, 'LC_ALL': 'C'})
    if result.returncode:
        raise ValueError('cannot record completed inode initialization: '+result.stderr.strip())
    groups = [line for line in result.stdout.splitlines() if re.match(r'^Group \d+:', line)]
    if not groups or any('ITABLE_ZEROED' not in line for line in groups):
        raise ValueError('inode table initialization incomplete; retain volume and retry after it finishes')


def reserve(path, size):
    fd = os.open(path, os.O_WRONLY | os.O_NOFOLLOW)
    try:
        os.posix_fallocate(fd, 0, size)
        os.fsync(fd)
    finally:
        os.close(fd)
    if regular(path).st_blocks * 512 < size:
        raise ValueError('backing file reservation remains sparse')


def format_new(path, fs_uuid):
    subprocess.run(['mkfs.ext4', '-q', '-E',
                    'nodiscard,lazy_itable_init=0,lazy_journal_init=0',
                    '-U', fs_uuid, str(path)], check=True)
    initialized(path)
    reserve(path, path.stat().st_size)


def safe_root(expected_uuid):
    if os.geteuid() != 0:
        raise ValueError('operator root required')
    if Path('/home').is_symlink() or ROOT.is_symlink():
        raise ValueError('symlink storage root refused')
    actual = run('findmnt', '-n', '-o', 'UUID', '--target', '/home')
    if not expected_uuid or actual != expected_uuid:
        raise ValueError('parent filesystem UUID mismatch')
    if not ROOT.is_dir() or ROOT.stat().st_uid != 0 or ROOT.stat().st_mode & 0o022:
        raise ValueError('root-owned service directory required')


def load_inventory():
    path = ROOT / 'w1-storage.json'
    if not path.exists():
        return {'scope': SCOPE, 'volumes': {}}
    regular(path)
    data = json.loads(path.read_text())
    if data['scope'] != SCOPE:
        raise ValueError('unknown inventory owner')
    return data


def save(data):
    path = ROOT / 'w1-storage.json'
    temporary = ROOT / 'w1-storage.json.new'
    with temporary.open('x') as output:
        os.chmod(temporary, 0o600)
        json.dump(data, output, indent=2)
        output.flush()
        os.fsync(output.fileno())
    os.replace(temporary, path)
    fd = os.open(ROOT, os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def provision(expected_uuid, runtime_mib, slot_mib):
    safe_root(expected_uuid)
    data = load_inventory()
    changed = False
    specs = {'runtime': runtime_mib, 'slot-01': slot_mib, 'slot-02': slot_mib}
    for name, size_mib in specs.items():
        path = ROOT / (name + '.ext4')
        mount = ROOT / name
        size = size_mib * 1024**2
        if mount.is_symlink():
            raise ValueError('symlink mountpoint refused')
        if name in data['volumes']:
            record = data['volumes'][name]
            s = regular(path)
            if (s.st_dev, s.st_ino, s.st_size) != (record['device'], record['inode'], size):
                raise ValueError('backing identity/size changed; never reformat')
            if run('blkid', '-p', '-s', 'UUID', '-o', 'value', str(path)) != record['uuid']:
                raise ValueError('backing filesystem UUID mismatch')
            if record.get('initialized') is not True:
                # One-time migration of earlier W1 files. Flush their mounted
                # filesystem first; never infer completion from a failed scan.
                # Once recorded, admission reads no live allocation bitmaps.
                if os.path.ismount(mount):
                    subprocess.run(['sync', '-f', str(mount)], check=True)
                initialized(path)
                record['initialized'] = True
                save(data)
                changed = True
            if s.st_blocks*512 < size:
                free = os.statvfs(ROOT)
                missing = size-s.st_blocks*512
                if free.f_bavail*free.f_frsize < missing + int(free.f_blocks*free.f_frsize*.15):
                    raise ValueError('insufficient headroom to restore owned file reservation')
                # Allocation of holes preserves all existing filesystem bytes;
                # this never invokes mkfs on a recorded file.
                reserve(path, size)
                changed = True
            continue
        if path.exists() or path.is_symlink() or mount.exists():
            raise ValueError('unrecorded storage exists; explicit reconciliation required')
        free = os.statvfs(ROOT)
        if free.f_bavail * free.f_frsize < size + max(512*1024**2, int(free.f_blocks*free.f_frsize*.15)):
            raise ValueError('insufficient reserved host headroom')
        # Exclusive creation plus a root-only locked Ansible run owns these new
        # bytes. Interrupted allocation is deliberately not resumed/reformatted.
        fd = os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY | os.O_NOFOLLOW, 0o600)
        try:
            os.posix_fallocate(fd, 0, size)
            os.fsync(fd)
        finally:
            os.close(fd)
        fs_uuid = str(uuid.uuid4())
        format_new(path, fs_uuid)
        s = regular(path)
        data['volumes'][name] = {'device': s.st_dev, 'inode': s.st_ino, 'bytes': size,
                                'uuid': fs_uuid, 'mount': str(mount), 'generation': 1,
                                'initialized': True}
        mount.mkdir(mode=0o000)
        save(data)
        changed = True
    print(json.dumps({'changed': changed, 'volumes': data['volumes']}))


def verify(expected_uuid):
    safe_root(expected_uuid)
    data = load_inventory()
    if set(data['volumes']) != {'runtime', 'slot-01', 'slot-02'}:
        raise ValueError('incomplete finite storage inventory')
    for name, rec in data['volumes'].items():
        backing = ROOT / (name + '.ext4')
        s = regular(backing)
        if (s.st_dev, s.st_ino, s.st_size) != (rec['device'], rec['inode'], rec['bytes']):
            raise ValueError('backing file identity changed')
        if s.st_blocks*512 < rec['bytes']:
            raise ValueError(f'{name}: backing file reservation is sparse')
        if rec.get('initialized') is not True:
            raise ValueError('completed inode initialization must be recorded by provision')
        mount = Path(rec['mount'])
        if mount != ROOT/name or mount.is_symlink() or not os.path.ismount(mount):
            raise ValueError('missing or wrong volume mount')
        if run('findmnt', '-n', '-o', 'UUID', '--mountpoint', str(mount)) != rec['uuid']:
            raise ValueError('mounted filesystem UUID mismatch')
        source = run('findmnt', '-n', '-o', 'SOURCE', '--mountpoint', str(mount))
        if Path(run('losetup', '-n', '-O', 'BACK-FILE', source)) != backing:
            raise ValueError('loop backing file mismatch')
    # Full host thresholds never apply to a 2 GiB workspace filesystem. The
    # retained small QEMU fixture has a separately named functional-test bound.
    guest = (Path('/etc/hostname').read_text().strip() == 'feam-w0-disposable'
             and subprocess.run(['systemd-detect-virt', '--vm', '--quiet']).returncode == 0)
    for parent in (Path('/'), ROOT):
        fs = os.statvfs(parent)
        floor = max((512 if guest else 20480) * 1024**2, int(fs.f_blocks*fs.f_frsize*.15))
        if fs.f_bavail*fs.f_frsize < floor:
            raise ValueError('underlying host filesystem lacks admission headroom')
    print('verified finite W1 mounts, identities and host headroom')


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('action', choices=['provision', 'verify'])
    p.add_argument('--parent-uuid', required=True)
    p.add_argument('--runtime-mib', type=int, default=4096, choices=[4096, 20480])
    p.add_argument('--slot-mib', type=int, default=2048, choices=[2048])
    a = p.parse_args()
    if not re.fullmatch(r'[a-f0-9-]{36}', a.parent_uuid):
        p.error('explicit UUID required')
    if a.action == 'provision':
        provision(a.parent_uuid, a.runtime_mib, a.slot_mib)
    else:
        verify(a.parent_uuid)


if __name__ == '__main__':
    main()
