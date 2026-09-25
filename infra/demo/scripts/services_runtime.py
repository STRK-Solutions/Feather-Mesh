#!/usr/bin/env python3
"""Root-only, fixed-scope FEAM service startup and Unix socket ACL checks.

No command/path comes from a browser. The root-owned manifest binds one reviewed
release, service UID map, verified mounts and opaque workspace directories.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import pwd
import re
import socket
import stat
import subprocess
import time

ROOT = Path('/home/feam-service-data')
RUN = Path('/run/feam')
CONFIG = Path('/etc/feam/services')
UIDS = {'gateway': 2102, 'controller': 2103, 'broker': 2104,
        'collector': 2105, 'pipeline': 2106, 'reconciler': 2107}
LOCAL_ARCHIVE = ROOT / 'traces/collector/archive'
DIGEST = re.compile(r'^[0-9a-f]{64}$')
UUID = re.compile(r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')


def require(condition, reason):
    if not condition:
        raise ValueError(reason)


def read_json(path, owner=0):
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and info.st_uid == owner and
            stat.S_IMODE(info.st_mode) & 0o077 == 0 and info.st_size <= 1 << 20,
            'private regular configuration owner/mode/size mismatch')
    return json.loads(path.read_text())


def validate_manifest(value):
    require(set(value) == {'protocol', 'release_sha256', 'frontend_uid', 'mounts',
                           'workspace_ids', 'config_sha256', 'artifact_sha256', 'broker_mode'},
            'unexpected service manifest fields')
    require(value['protocol'] == 'feam.services.v1' and DIGEST.fullmatch(value['release_sha256']),
            'unreviewed service release')
    require(type(value['frontend_uid']) is int and value['frontend_uid'] >= 1000 and
            value['frontend_uid'] not in range(2101, 2108) and value['frontend_uid'] != 200999,
            'dedicated frontend identity required')
    require(value['broker_mode'] in ('fake', 'live'), 'explicit broker mode required')
    require(set(value['mounts']) == {'operations', 'traces', 'datasets'} and
            all(UUID.fullmatch(v) for v in value['mounts'].values()), 'recorded storage UUIDs required')
    require(1 <= len(value['workspace_ids']) <= 10 and
            len(set(value['workspace_ids'])) == len(value['workspace_ids']) and
            all(UUID.fullmatch(v) for v in value['workspace_ids']), 'opaque unique workspace IDs required')
    require(set(value['config_sha256']) == set(UIDS) and
            all(DIGEST.fullmatch(v) for v in value['config_sha256'].values()), 'reviewed config hashes required')
    require(set(value['artifact_sha256']) == set(UIDS) | {'feam'} and
            all(DIGEST.fullmatch(v) for v in value['artifact_sha256'].values()), 'reviewed binary hashes required')
    return value


def within(path, parent):
    require(isinstance(path, str) and Path(path).is_absolute() and
            str(Path(path)) == path and '..' not in Path(path).parts and Path(path).is_relative_to(parent) and Path(path) != parent,
            'configured path escapes its service scope')


def state_root(service):
    if service == 'collector':
        return ROOT / 'traces/collector'
    if service == 'pipeline':
        return ROOT / 'datasets/pipeline'
    return ROOT / 'operations' / service


def service_socket(service, name='control.sock'):
    return str(RUN / 'services' / service / name)


def validate_config(service, value, manifest):
    """Validate authority paths/UIDs without copying private fields into logs."""
    require(service in UIDS and isinstance(value, dict), 'unknown service configuration')
    root = state_root(service)
    own = CONFIG / service
    database_fields = {'gateway': ('db',), 'controller': ('database',),
                       'broker': ('ledger', 'capabilities'), 'collector': ('db', 'capabilities_db')}
    for field in database_fields.get(service, ()):
        within(value[field], root)
    if service == 'gateway':
        require(value['browser_socket'] == service_socket(service, 'browser.sock') and
                value['control_socket'] == service_socket(service) and value['controller_uid'] == 2103 and
                value['reconciler_uid'] == 2107 and value['pipeline_uid'] == 2106 and set(value['read_uids']) == {2103, 2104, 2105, 2106, 2107},
                'gateway service UID/socket policy mismatch')
        require(value['gateway']['controller_socket'] == service_socket('controller') and
                value['gateway']['pipeline_socket'] == service_socket('pipeline') and
                value['gateway']['broker_socket'] == service_socket('broker'), 'gateway authority endpoint mismatch')
    elif service == 'controller':
        require(value['socket'] == service_socket(service) and value['gateway_socket'] == service_socket('gateway') and
                value['runtime_socket'] == '/run/user/2101/feam-docker.sock' and value['gateway_uid'] == 2102 and
                value['broker_uid'] == 2104,
                'controller authority endpoint mismatch')
        for peer in ('broker', 'collector', 'pipeline'):
            require(value[peer + '_socket'] == service_socket(peer), 'controller service endpoint mismatch')
        require(value['release_root'] == str(state_root('pipeline') / 'releases') and
                value['profile_path'] == str(own / 'agent.toml'), 'controller release/profile scope mismatch')
        require(set(value['endpoints']) == set(manifest['workspace_ids']), 'controller workspace scope mismatch')
        for wid, endpoint in value['endpoints'].items():
            for field, leaf in [('terminal_directory', 'terminal'), ('model_directory', 'model'),
                                ('event_directory', 'events'), ('config_directory', 'config')]:
                require(endpoint[field] == str(RUN / 'workspaces' / wid / leaf), 'controller endpoint binding mismatch')
            require(UUID.fullmatch(endpoint['participant_id']) and type(endpoint['synthetic']) is bool and
                    all(DIGEST.fullmatch(endpoint[key]) for key in ('software_sha256', 'profile_sha256', 'dataset_sha256')),
                    'controller capture revision policy mismatch')
    elif service == 'broker':
        require(value['control_socket'] == service_socket(service) and value['authority_socket'] == service_socket('gateway') and
                value['controller_uid'] == 2103 and value['gateway_uid'] == 2102 and
                value['collector_socket'] == service_socket('collector') and
                value['controller_socket'] == service_socket('controller'),
                'broker authority/capture endpoint mismatch')
        expected = {wid: str(RUN / 'workspaces' / wid / 'model/broker.sock') for wid in manifest['workspace_ids']}
        require({v['workspace_id']: v['path'] for v in value['sockets']} == expected and len(value['sockets']) == len(expected),
                'model endpoint workspace mismatch')
        within(value['receipt_file'], root)
        require(value['receipt_file'] not in (value['ledger'], value['capabilities']), 'receipt collides with database')
        for field in ('allocation', 'operator_public_key'):
            within(value[field], own)
        if manifest['broker_mode'] == 'live':
            within(value['upstream_key_file'], own)
    elif service == 'collector':
        require(value['control_socket'] == service_socket(service) and value['gateway_socket'] == service_socket('gateway') and
                value['broker_uid'] == 2104 and value['controller_uid'] == 2103 and
                value['verifier_uid'] == 2106 and value['reviewer_uids'] and value['reviewer'] and
                all(type(uid) is int and uid >= 1000 and uid not in range(2101, 2108) and
                    uid not in (200999, manifest['frontend_uid']) for uid in value['reviewer_uids']),
                'collector authority/research policy mismatch')
        require(value['workspace_sockets'] == {wid: str(RUN / 'workspaces' / wid / 'events/events.sock')
                                            for wid in manifest['workspace_ids']}, 'event endpoint workspace mismatch')
        within(value['policy_file'], own)
        require(0 < value['local_max_bytes'] <= 20 << 30 and
                0 < value['archive_max_bytes'] <= 1 << 30, 'archive/capacity configuration missing')
        if value.get('archive_mode') == 'local':
            require(value.get('archive_directory') == str(LOCAL_ARCHIVE) and
                    not any(value.get(field) for field in ('r2_credentials_file', 'r2_account_id', 'r2_bucket')),
                    'fixed private local archive required without R2 inputs')
        elif value.get('archive_mode') == 'r2':
            within(value['r2_credentials_file'], own)
            require(value.get('r2_account_id') and value.get('r2_bucket') and
                    not value.get('archive_directory'), 'private R2 archive scope required')
        else:
            raise ValueError('explicit supported collector archive mode required')
    elif service == 'reconciler':
        require(value['control_socket'] == service_socket('gateway'), 'membership authority endpoint mismatch')
        within(value['token_file'], own)
    elif service == 'pipeline':
        release = '/opt/feam/services/' + manifest['release_sha256']
        require(value == {'root': str(root), 'feam': release + '/feam',
                          'python': '/opt/feam/pipeline/bin/python', 'converter': '/opt/feam/importers/convert.py',
                          'socket': service_socket(service), 'controller_uid': 2103, 'gateway_uid': 2102,
                          'control_socket': service_socket('gateway')},
                'fixed pipeline executable/authority mismatch')
    return value


def checked_manifest(path):
    require(os.geteuid() == 0 and path == CONFIG / 'runtime.json', 'root-owned fixed manifest required')
    return validate_manifest(read_json(path))


def verify_mounts(manifest):
    for name, expected in manifest['mounts'].items():
        path = ROOT / name
        require(path.resolve() == path and path.is_mount(), 'required service filesystem is not mounted')
        actual = subprocess.check_output(['findmnt', '-n', '-o', 'UUID', '--mountpoint', str(path)], text=True).strip()
        require(actual == expected, 'service filesystem UUID mismatch')
    require(RUN.resolve() == RUN and RUN.is_mount(), 'private runtime tmpfs is not mounted')
    kind = subprocess.check_output(['findmnt', '-n', '-o', 'FSTYPE', '--mountpoint', str(RUN)], text=True).strip()
    capacity = os.statvfs(RUN)
    require(kind == 'tmpfs' and capacity.f_blocks * capacity.f_frsize <= 32 << 20,
            'runtime storage must be bounded to 32 MiB')


def checked_config(service, manifest):
    path = CONFIG / service / 'config.json'
    value = read_json(path, UIDS[service])
    require(hashlib.sha256(path.read_bytes()).hexdigest() == manifest['config_sha256'][service], 'config revision mismatch')
    return validate_config(service, value, manifest)


def verify_service(service, manifest):
    require(pwd.getpwnam('feam-' + service).pw_uid == UIDS[service], 'service UID collision')
    value = checked_config(service, manifest)
    artifact = Path('/opt/feam/services') / manifest['release_sha256'] / service
    info = artifact.lstat()
    require(stat.S_ISREG(info.st_mode) and info.st_uid == 0 and stat.S_IMODE(info.st_mode) & 0o022 == 0,
            'service binary owner/mode mismatch')
    require(hashlib.sha256(artifact.read_bytes()).hexdigest() == manifest['artifact_sha256'][service], 'artifact revision mismatch')
    require(state_root(service).resolve() == state_root(service), 'symlinked state root')
    if service == 'collector' and value['archive_mode'] == 'local':
        info = LOCAL_ARCHIVE.lstat()
        state = state_root('collector').lstat()
        require(stat.S_ISDIR(info.st_mode) and stat.S_IMODE(info.st_mode) == 0o700 and
                info.st_uid == UIDS['collector'] and info.st_dev == state.st_dev and
                LOCAL_ARCHIVE.resolve() == LOCAL_ARCHIVE,
                'private local archive must be on verified traces filesystem')
    # Runtime binaries also validate signed budget, current policy and DB hashes.
    return value


def directory(path, owner, readers=None):
    if path.exists() or path.is_symlink():
        require(path.resolve() == path and path.is_dir() and path.stat().st_uid == owner,
                'unowned runtime directory collision')
    else:
        path.mkdir(mode=0o700)
        os.chown(path, owner, owner)
    acl = ['u::rwx', 'g::---', 'o::---'] + [f'u:{uid}:{mode}' for uid, mode in (readers or {}).items()]
    subprocess.run(['setfacl', '--set', ','.join(acl), str(path)], check=True, stdout=subprocess.DEVNULL)


def socket_specs(service, manifest):
    peers = {'gateway': {2103, 2104, 2105, 2106, 2107}, 'controller': {2102, 2104},
             'broker': {2102, 2103}, 'collector': {2103, 2104, 2106}, 'pipeline': {2102, 2103}}
    result = []
    if service in peers:
        result.append((Path(service_socket(service)), UIDS[service], peers[service]))
    if service == 'gateway':
        result.append((Path(service_socket(service, 'browser.sock')), 2102, {manifest['frontend_uid']}))
    if service == 'collector':
        config = checked_config(service, manifest)
        result[0][2].update(config['reviewer_uids'])
    for wid in manifest['workspace_ids']:
        if service == 'broker':
            result.append((RUN / 'workspaces' / wid / 'model/broker.sock', 2104, {200999}))
        elif service == 'collector':
            result.append((RUN / 'workspaces' / wid / 'events/events.sock', 2105, {200999}))
    return result


def layout(service, manifest):
    require(service in UIDS, 'unknown service')
    # Shared parents expose traversal only; leaf owners and explicit ACLs isolate.
    reviewers = checked_config('collector', manifest)['reviewer_uids']
    require(all(type(uid) is int and uid >= 1000 and uid not in range(2101, 2108) and uid not in (200999, manifest['frontend_uid']) for uid in reviewers), 'invalid research-reviewer identities')
    for path in (RUN / 'services', RUN / 'workspaces'):
        directory(path, 0, {uid: 'x' for uid in [2101, *UIDS.values(), 200999, manifest['frontend_uid'], *reviewers]})
    readers = {peer: 'x' for _, _, peers in socket_specs(service, manifest) for peer in peers}
    directory(RUN / 'services' / service, UIDS[service], readers)
    for wid in manifest['workspace_ids']:
        parent = RUN / 'workspaces' / wid
        directory(parent, 0, {uid: 'x' for uid in [2101, 2102, 2103, 2104, 2105, 200999]})
        if service == 'gateway':
            directory(parent / 'terminal', 200999, {2101: 'x', 2102: 'rwx', 2103: 'rwx'})
            subprocess.run(['setfacl', '-m', 'd:u::rwx,d:u:2102:rwx,d:u:2103:rwx,d:g::---,d:m::rwx,d:o::---', str(parent / 'terminal')], check=True)
        elif service == 'controller':
            directory(parent / 'config', 2103, {2101: 'x', 200999: 'rx'})
        elif service == 'broker':
            directory(parent / 'model', 2104, {2101: 'x', 200999: 'rx'})
        elif service == 'collector':
            directory(parent / 'events', 2105, {2101: 'x', 200999: 'rx'})


def remove_stale_socket(path, owner):
    if not path.exists() and not path.is_symlink():
        return
    info = path.lstat()
    require(stat.S_ISSOCK(info.st_mode) and info.st_uid == owner, 'unowned socket collision')
    probe = socket.socket(socket.AF_UNIX)
    probe.settimeout(1)
    try:
        probe.connect(str(path))
    except ConnectionRefusedError:
        current = path.lstat()
        require(current.st_ino == info.st_ino and current.st_dev == info.st_dev and
                stat.S_ISSOCK(current.st_mode) and current.st_uid == owner,
                'socket changed during stale-endpoint reconciliation')
        path.unlink()  # fixed owned endpoint; no live listener was present
    else:
        raise ValueError('existing listener must be stopped before restart')
    finally:
        probe.close()


def set_socket_acls(service, manifest):
    for path, owner, peers in socket_specs(service, manifest):
        deadline = time.monotonic() + 10
        while not path.exists() and time.monotonic() < deadline:
            time.sleep(0.05)
        info = path.lstat()
        require(stat.S_ISSOCK(info.st_mode) and info.st_uid == owner, 'expected owned socket unavailable')
        acl = ['u::rw-', 'g::---', 'o::---'] + [f'u:{peer}:rw-' for peer in sorted(peers)]
        subprocess.run(['setfacl', '--set', ','.join(acl), str(path)], check=True)


def health(service, manifest):
    """Read-only liveness; never starts a process or changes desired state."""
    state = subprocess.check_output(['systemctl', 'show', 'feam-' + service + '.service',
                                     '--property=ActiveState', '--value'], text=True, timeout=5).strip()
    require(state in ('active', 'inactive', 'activating', 'deactivating'), 'service has failed')
    if state != 'active':
        return
    for path, owner, _ in socket_specs(service, manifest):
        info = path.lstat()
        require(stat.S_ISSOCK(info.st_mode) and info.st_uid == owner, 'active service endpoint missing or unowned')
        with socket.socket(socket.AF_UNIX) as probe:
            probe.settimeout(1)
            probe.connect(str(path))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['verify', 'layout', 'prepare', 'socket-acls', 'health'])
    parser.add_argument('--service', choices=sorted(UIDS), required=True)
    parser.add_argument('--manifest', type=Path, default=CONFIG / 'runtime.json')
    args = parser.parse_args()
    manifest = checked_manifest(args.manifest)
    verify_mounts(manifest)
    verify_service(args.service, manifest)
    if args.action in ('layout', 'prepare'):
        layout(args.service, manifest)
    if args.action == 'prepare':
        for path, owner, _ in socket_specs(args.service, manifest):
            remove_stale_socket(path, owner)
    if args.action == 'socket-acls':
        set_socket_acls(args.service, manifest)
    if args.action == 'health':
        health(args.service, manifest)
    print('PASS: fixed service ownership, mount and configuration checks')


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError):
        raise SystemExit('FEAM service preflight failed closed; inspect reviewed ownership/mount/configuration inputs.')
