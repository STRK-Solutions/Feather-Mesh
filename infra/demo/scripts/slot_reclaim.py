#!/usr/bin/env python3
"""Explicit root-only cleanup of one reviewed, stopped, recorded FEAM slot.

The controller never invokes this helper and receives no sudo permission.
No paths, shell commands, backing files or runtime IDs come from a participant.
"""
import argparse
from datetime import datetime, timezone
import fcntl
import hashlib
import http.client
import json
import os
from pathlib import Path
import re
import socket
import sqlite3
import stat
import subprocess
import tempfile
import time

ROOT = Path('/home/feam-service-data')
CONFIG = Path('/etc/feam/services/controller/config.json')
MANIFEST = Path('/etc/feam/services/runtime.json')
RECEIPTS = Path('/run/feam/reclamation')
RUNTIME = '/run/user/2101/feam-docker.sock'
SHA = re.compile(r'[0-9a-f]{64}\Z')
ID = re.compile(r'[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\Z')


def require(condition, reason):
    if not condition:
        raise ValueError(reason)


def private_json(path, owner):
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and info.st_uid == owner and
            info.st_mode & 0o077 == 0 and info.st_nlink == 1 and info.st_size <= 1 << 20,
            'private regular owner-bound input required')
    return json.loads(path.read_bytes())


def output(*args):
    return subprocess.check_output(args, text=True, timeout=5).strip()


def verify_slot(plan, config, inventory):
    slot = next((s for s in config['slots'] if s['id'] == plan['slot_id']), None)
    require(slot is not None and re.fullmatch(r'slot-(0[1-9]|1[0-2])', plan['slot_id']),
            'unknown fixed slot')
    path = ROOT / plan['slot_id']
    record = inventory['volumes'][plan['slot_id']]
    require(inventory['scope'] == 'feam-w1-storage-v1' and
            str(path) == plan['path'] == slot['path'] == record['mount'] and
            plan['filesystem_uuid'] == slot['filesystem_uuid'] == record['uuid'],
            'slot authority does not match reviewed storage inventory')
    require(path.resolve() == path and path.is_mount(), 'recorded slot mount missing')
    backing = ROOT / (plan['slot_id'] + '.ext4')
    info = backing.lstat()
    require(stat.S_ISREG(info.st_mode) and info.st_uid == 0 and info.st_nlink == 1 and
            (info.st_dev, info.st_ino, info.st_size) == (record['device'], record['inode'], record['bytes']) and
            info.st_blocks * 512 >= record['bytes'] and record['initialized'] is True,
            'backing file identity changed')
    require(output('findmnt', '-n', '-o', 'UUID', '--mountpoint', str(path)) == record['uuid'],
            'filesystem UUID changed')
    source = output('findmnt', '-n', '-o', 'SOURCE', '--mountpoint', str(path))
    require(output('losetup', '-n', '-O', 'BACK-FILE', source) == str(backing),
            'loop backing identity changed')
    for line in Path('/proc/self/mountinfo').read_text().splitlines():
        mount = Path(re.sub(r'\\([0-7]{3})', lambda m: chr(int(m[1], 8)), line.split()[4]))
        require(mount == path or not mount.is_relative_to(path), 'nested mount blocks reclamation')
    return path


class UnixHTTP(http.client.HTTPConnection):
    def connect(self):
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.settimeout(2)
        self.sock.connect(RUNTIME)


def runtime_json(route):
    client = UnixHTTP('runtime', timeout=2)
    try:
        client.request('GET', '/v1.47' + route)
        response = client.getresponse()
        raw = response.read((1 << 20) + 1)
        require(response.status == 200 and len(raw) <= 1 << 20, 'runtime observation unavailable')
        return json.loads(raw)
    finally:
        client.close()


def require_no_container_mount(path):
    path = path.resolve()
    deadline = time.monotonic() + 30
    containers = runtime_json('/containers/json?all=1')
    require(isinstance(containers, list) and len(containers) <= 128, 'runtime inventory exceeds bound')
    for container in containers:
        require(time.monotonic() < deadline and SHA.fullmatch(container['Id']), 'invalid runtime identity')
        inspected = runtime_json('/containers/' + container['Id'] + '/json')
        for mount in inspected['Mounts']:
            source = Path(mount['Source']).resolve()
            require(not (source == path or source.is_relative_to(path) or path.is_relative_to(source)),
                    'a retained or running container still references this slot')


def walk(directory, device, deadline, remove=False, depth=0, counter=None):
    """Descriptor-relative traversal never follows symlinks or crosses devices."""
    counter = [0] if counter is None else counter
    require(depth <= 128 and time.monotonic() < deadline, 'cleanup traversal bound exceeded')
    for name in os.listdir(directory):
        counter[0] += 1
        require(counter[0] <= 200000 and time.monotonic() < deadline, 'cleanup inventory bound exceeded')
        info = os.stat(name, dir_fd=directory, follow_symlinks=False)
        require(info.st_dev == device, 'nested filesystem blocks cleanup')
        if depth == 0 and name == 'lost+found':
            require(stat.S_ISDIR(info.st_mode) and info.st_uid == 0, 'unexpected lost+found')
            child = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=directory)
            try:
                require(not os.listdir(child), 'nonempty filesystem recovery directory')
            finally:
                os.close(child)
            continue
        if stat.S_ISDIR(info.st_mode):
            child = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=directory)
            try:
                walk(child, device, deadline, remove, depth + 1, counter)
            finally:
                os.close(child)
            if remove:
                os.rmdir(name, dir_fd=directory)
        elif remove:
            os.unlink(name, dir_fd=directory)


def clear_contents(path):
    descriptor = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        device = os.fstat(descriptor).st_dev
        walk(descriptor, device, time.monotonic() + 30)
        walk(descriptor, device, time.monotonic() + 30, remove=True)
        require(set(os.listdir(descriptor)) <= {'lost+found'}, 'slot cleanup incomplete')
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def receipt(value):
    if not RECEIPTS.exists():
        RECEIPTS.mkdir(mode=0o755)
    info = RECEIPTS.lstat()
    require(stat.S_ISDIR(info.st_mode) and info.st_uid == 0 and info.st_mode & 0o022 == 0,
            'root-owned receipt directory required')
    descriptor, temporary = tempfile.mkstemp(prefix='.pending-', dir=RECEIPTS)
    try:
        with os.fdopen(descriptor, 'w') as file:
            json.dump(value, file, sort_keys=True)
            file.flush()
            os.fchmod(file.fileno(), 0o644)
            os.fsync(file.fileno())
        os.replace(temporary, RECEIPTS / (value['id'] + '.json'))
        directory = os.open(RECEIPTS, os.O_DIRECTORY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        Path(temporary).unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['check', 'apply'])
    parser.add_argument('--id', required=True)
    parser.add_argument('--confirm-sha256', required=True)
    args = parser.parse_args()
    require(os.geteuid() == 0 and ID.fullmatch(args.id) and SHA.fullmatch(args.confirm_sha256),
            'root operator and exact reviewed intent hash required')
    manifest = private_json(MANIFEST, 0)
    config = private_json(CONFIG, 2103)
    require(hashlib.sha256(CONFIG.read_bytes()).hexdigest() == manifest['config_sha256']['controller'],
            'controller configuration differs from reviewed root manifest')
    require(config['runtime_socket'] == RUNTIME, 'dedicated runtime required')
    database = Path(config['database'])
    require(database.resolve() == database and database.is_relative_to(ROOT / 'operations/controller'),
            'private controller database required')
    lock = os.open(str(database) + '.controller.lock', os.O_RDWR | os.O_NOFOLLOW)
    try:
        info = os.fstat(lock)
        require(stat.S_ISREG(info.st_mode) and info.st_uid == 2103 and info.st_mode & 0o077 == 0,
                'controller lock owner mismatch')
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        connection = sqlite3.connect(database.as_uri() + '?mode=ro', uri=True)
        try:
            row = connection.execute("SELECT payload,sha256,state FROM slot_reclaims WHERE id=?", (args.id,)).fetchone()
            require(row and row[2] == 'pending' and row[1] == args.confirm_sha256 and
                    hashlib.sha256(row[0].encode()).hexdigest() == row[1], 'reclamation scope changed or completed')
            plan = json.loads(row[0])
            require(plan['protocol'] == 'feam.slot-reclaim.v1' and plan['id'] == args.id and
                    plan['deployment_id'] == config['deployment_id'], 'deployment scope changed')
            require(connection.execute('SELECT desired FROM deployment').fetchone()[0] == 'stopped',
                    'deployment must remain intentionally stopped')
            require(connection.execute("SELECT count(*) FROM lifecycle_jobs WHERE workspace_id=? AND state IN('accepted','executing','unknown')", (plan['workspace_id'],)).fetchone()[0] == 0,
                    'uncertain lifecycle work remains')
            state = connection.execute('SELECT state,workspace_id,generation FROM slots WHERE id=?', (plan['slot_id'],)).fetchone()
            require(state and state[1:] == (plan['workspace_id'], plan['generation']) and
                    state[0] == ('assigned' if plan['mode'] == 'inactive' else 'retained'), 'slot assignment changed')
            path = verify_slot(plan, config, private_json(ROOT / 'w1-storage.json', 0))
            require_no_container_mount(path)
            if args.action == 'apply':
                clear_contents(path)
                require_no_container_mount(path)
                receipt({'protocol': 'feam.slot-reclaimed.v1', 'id': args.id,
                         'plan_sha256': row[1], 'filesystem_uuid': plan['filesystem_uuid'],
                         'empty': True, 'completed_at': datetime.now(timezone.utc).isoformat().replace('+00:00', 'Z')})
            print(json.dumps({'status': 'cleared' if args.action == 'apply' else 'verified',
                              'id': args.id, 'slot_id': plan['slot_id'], 'plan_sha256': row[1]}))
        finally:
            connection.close()
    finally:
        os.close(lock)


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, KeyError, TypeError, sqlite3.Error, subprocess.SubprocessError, http.client.HTTPException):
        raise SystemExit('Slot reclamation failed closed; retain the intent and inspect its exact owned scope. No slot is reusable without the root receipt.')
