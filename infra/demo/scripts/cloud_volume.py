#!/usr/bin/env python3
"""Explicit, root-only initialization of one newly created approved DO volume.

Never discovers resources, changes quotas, repairs a filesystem, or reformats an
existing signature. Terraform's exact bootstrap output is the operator input.
"""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess

ROOT = Path('/home/feam-service-data')
STATE_DIR = Path('/var/lib/feam-cloud-volume')
STATE = STATE_DIR / 'ownership.json'
UNIT = Path('/etc/systemd/system/home-feam\\x2dservice\\x2ddata.mount')
GIB = 1 << 30
BYTES = 320 * GIB
POOL = 269 * GIB
HEADROOM = GIB
ID = re.compile(r'[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\Z')
UUID = re.compile(r'[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}\Z')
SHA = re.compile(r'[0-9a-f]{64}\Z')


def require(condition, message):
    if not condition:
        raise ValueError(message)


def run(*args, check=True, timeout=20):
    return subprocess.run(args, check=check, stdin=subprocess.DEVNULL,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                          text=True, timeout=timeout, env={**os.environ, 'LC_ALL': 'C'})


def private_bytes(path):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    try:
        st = os.fstat(fd)
        require(stat.S_ISREG(st.st_mode) and st.st_uid == 0 and st.st_nlink == 1
                and st.st_mode & 0o077 == 0 and st.st_size <= 65536,
                'root-owned private regular manifest required')
        return os.read(fd, 65537)
    finally:
        os.close(fd)


def validate_manifest(m):
    require(isinstance(m, dict) and set(m) == {
        'protocol', 'deployment_id', 'volume_id', 'droplet_id', 'approved_quote_sha256',
        'region', 'volume_name', 'bytes', 'filesystem_uuid', 'mount'}, 'invalid manifest fields')
    require(m['protocol'] == 'feam.cloud-volume.v1' and ID.fullmatch(m['deployment_id'])
            and UUID.fullmatch(m['volume_id']) and re.fullmatch(r'[1-9][0-9]{0,19}', m['droplet_id'])
            and SHA.fullmatch(m['approved_quote_sha256']), 'invalid deployment/volume/quote identity')
    require(m['region'] == 'nyc3' and m['bytes'] == BYTES and m['filesystem_uuid'] == m['deployment_id']
            and m['mount'] == str(ROOT) and m['volume_name'] == 'feam-' + m['deployment_id'] + '-service',
            'manifest exceeds the approved fixed volume scope')
    return m


def stable_device(m):
    return Path('/dev/disk/by-id/scsi-0DO_Volume_' + m['volume_name'])


def device_inventory(m, allow_mounted=False):
    link = stable_device(m)
    require(link.is_symlink(), 'expected provider stable device link missing')
    device = link.resolve(strict=True)
    st = device.stat()
    require(str(device).startswith('/dev/') and stat.S_ISBLK(st.st_mode), 'not a block device')
    data = json.loads(run('lsblk', '--json', '--bytes', '--paths', '--output',
                          'NAME,TYPE,SIZE,RO,PKNAME,MOUNTPOINTS', str(device)).stdout)
    rows = data.get('blockdevices', [])
    require(len(rows) == 1, 'ambiguous device inventory')
    d = rows[0]
    require(d['name'] == str(device) and d['type'] == 'disk' and int(d['size']) == BYTES
            and not d['ro'] and not d.get('pkname') and not d.get('children'),
            'device size, type, read-only state or child inventory changed')
    mounts = [p for p in d.get('mountpoints', []) if p is not None]
    require(not mounts or (allow_mounted and mounts == [str(ROOT)]), 'device is already in use')
    holders = Path('/sys/dev/block') / (str(os.major(st.st_rdev)) + ':' + str(os.minor(st.st_rdev))) / 'holders'
    require(holders.is_dir() and not list(holders.iterdir()), 'device has holders or unverifiable holders')
    for line in Path('/proc/swaps').read_text().splitlines()[1:]:
        require(Path(line.split()[0]).resolve() != device, 'device is active swap')
    return device, st.st_rdev


def require_blank(device):
    result = json.loads(run('wipefs', '--no-act', '--json', str(device)).stdout)
    require(set(result) == {'signatures'} and not result['signatures'], 'existing device signature; never reformat')
    probe = run('blkid', '-p', str(device), check=False)
    require(probe.returncode == 2 and not probe.stdout.strip(), 'device blankness is not established')


def format_command(m, device):
    return ['mkfs.ext4', '-q', '-b', '4096', '-i', '65536', '-I', '256', '-m', '0',
            '-J', 'size=256', '-E', 'nodiscard,lazy_itable_init=0,lazy_journal_init=0',
            '-U', m['filesystem_uuid'], '-L', 'feam-service', str(device)]


def filesystem(m, device):
    fields = {}
    for line in run('dumpe2fs', '-h', str(device)).stdout.splitlines():
        if ':' in line:
            k, v = line.split(':', 1)
            fields[k] = v.strip()
    require(fields.get('Filesystem UUID') == m['filesystem_uuid']
            and fields.get('Filesystem volume name') == 'feam-service'
            and fields.get('Block size') == '4096'
            and int(fields.get('Block count', '0')) * 4096 == BYTES
            and fields.get('Reserved block count') == '0'
            and fields.get('Inode size') == '256'
            and fields.get('Inode count') == '5242880', 'owned filesystem format or identity mismatch')
    return fields


def private_directory(path):
    if not path.exists():
        path.mkdir(mode=0o700)
    st = path.lstat()
    require(stat.S_ISDIR(st.st_mode) and st.st_uid == 0 and st.st_mode & 0o077 == 0,
            'private root state directory required')


def write_new(path, data, mode):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, mode)
    try:
        with os.fdopen(fd, 'wb') as out:
            out.write(data)
            out.flush()
            os.fsync(out.fileno())
    finally:
        parent = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(parent)
        finally:
            os.close(parent)


def mount_unit(m):
    return ('# Managed by feam cloud_volume.py; exact ownership required.\n'
            '[Unit]\nDescription=FEAM approved disposable service volume\n'
            'RequiresMountsFor=/home\nBefore=user@2101.service\n'
            '[Mount]\nWhat=/dev/disk/by-uuid/' + m['filesystem_uuid'] + '\nWhere=' + str(ROOT) + '\n'
            'Type=ext4\nOptions=nosuid,nodev,noexec,nodiscard\nTimeoutSec=30\n'
            '[Install]\nWantedBy=local-fs.target\n').encode()


def require_mount(m):
    require(ROOT.resolve() == ROOT and ROOT.is_mount(), 'exact service root mount missing')
    require(run('findmnt', '-n', '-o', 'UUID', '--mountpoint', str(ROOT)).stdout.strip()
            == m['filesystem_uuid'], 'wrong service root filesystem mounted')
    info = ROOT.lstat()
    require(info.st_uid == 0 and info.st_mode & 0o022 == 0, 'unsafe service root permissions')


def capacity(total, available, remaining=0):
    require(0 < total <= BYTES and 0 <= available <= total and 0 <= remaining <= POOL,
            'invalid filesystem capacity observation')
    floor = max(20 * GIB, (total * 15 + 99) // 100)
    require(available - remaining >= floor + HEADROOM,
            'approved 320 GiB volume cannot fit full pool plus 15% floor and 1 GiB headroom')
    return {'filesystem_bytes': total, 'available_bytes': available, 'pool_remaining_bytes': remaining,
            'required_floor_bytes': floor, 'headroom_bytes': HEADROOM,
            'surplus_bytes': available - remaining - floor - HEADROOM}


def capacity_at_mount(m, fresh=False):
    require_mount(m)
    v = os.statvfs(ROOT)
    return capacity(v.f_blocks * v.f_frsize, v.f_bavail * v.f_frsize, POOL if fresh else 0)


def mount_owned(m, device):
    filesystem(m, device)
    require(Path('/home').resolve() == Path('/home'), 'symlinked /home refused')
    if ROOT.exists():
        require(ROOT.resolve() == ROOT and ROOT.is_dir(), 'unexpected service root')
        if not ROOT.is_mount():
            require(not list(ROOT.iterdir()), 'unmounted service root contains unknown files')
    else:
        ROOT.mkdir(mode=0o755)
    expected = mount_unit(m)
    if UNIT.exists() or UNIT.is_symlink():
        info = UNIT.lstat()
        require(stat.S_ISREG(info.st_mode) and info.st_uid == 0 and info.st_mode & 0o022 == 0
                and info.st_nlink == 1 and UNIT.read_bytes() == expected, 'unknown mount unit; preserve it')
    else:
        write_new(UNIT, expected, 0o644)
    run('systemctl', 'daemon-reload')
    run('systemctl', 'enable', '--now', UNIT.name, timeout=45)
    require_mount(m)


def initialize(m, manifest_sha, new_volume_id):
    require(new_volume_id == m['volume_id'], 'explicit newly created volume identity required')
    require(not STATE.exists() and not STATE.is_symlink(), 'ownership intent exists; initialization is never replayed')
    device, number = device_inventory(m)
    require_blank(device)
    # Persist intent first. Interrupted formatting requires inspection; it must
    # never be silently repeated, even when the signature still appears blank.
    record = {'protocol': 'feam.cloud-volume-ownership.v1', 'manifest_sha256': manifest_sha,
              'volume_id': m['volume_id'], 'filesystem_uuid': m['filesystem_uuid'],
              'stable_device': str(stable_device(m))}
    write_new(STATE, json.dumps(record, sort_keys=True).encode() + b'\n', 0o600)
    again, actual = device_inventory(m)
    require(again == device and actual == number, 'device changed before format')
    require_blank(again)
    run(*format_command(m, device), timeout=300)
    mount_owned(m, device)
    return capacity_at_mount(m, fresh=True)


def owned(m, manifest_sha):
    record = json.loads(private_bytes(STATE))
    require(record == {'protocol': 'feam.cloud-volume-ownership.v1', 'manifest_sha256': manifest_sha,
                       'volume_id': m['volume_id'], 'filesystem_uuid': m['filesystem_uuid'],
                       'stable_device': str(stable_device(m))}, 'ownership manifest changed')
    device, _ = device_inventory(m, allow_mounted=True)
    filesystem(m, device)
    return device


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['inspect', 'initialize', 'mount', 'verify'])
    parser.add_argument('--manifest', required=True, type=Path)
    parser.add_argument('--confirm-sha256', required=True)
    parser.add_argument('--new-volume-id', default='')
    args = parser.parse_args()
    require(os.geteuid() == 0, 'operator root execution required')
    raw = private_bytes(args.manifest)
    hashed = hashlib.sha256(raw).hexdigest()
    require(SHA.fullmatch(args.confirm_sha256) and hashed == args.confirm_sha256, 'manifest approval hash mismatch')
    m = validate_manifest(json.loads(raw))
    private_directory(STATE_DIR)
    lock = os.open(STATE_DIR / 'lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    try:
        lock_info = os.fstat(lock)
        require(stat.S_ISREG(lock_info.st_mode) and lock_info.st_uid == 0
                and lock_info.st_nlink == 1 and lock_info.st_mode & 0o077 == 0,
                'private regular root lock required')
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if args.mode == 'inspect':
            device_inventory(m, allow_mounted=True)
            result = {'status': 'device-observed', 'bytes': BYTES}
        elif args.mode == 'initialize':
            result = initialize(m, hashed, args.new_volume_id)
        else:
            device = owned(m, hashed)
            if args.mode == 'mount':
                mount_owned(m, device)
            result = capacity_at_mount(m)
        print(json.dumps({'protocol': 'feam.cloud-volume-receipt.v1', 'mode': args.mode,
                          'deployment_id': m['deployment_id'], 'volume_id': m['volume_id'],
                          'manifest_sha256': hashed, **result}, sort_keys=True))
    finally:
        os.close(lock)


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, subprocess.SubprocessError, KeyError, TypeError) as error:
        raise SystemExit('cloud volume refused: ' + str(error))
