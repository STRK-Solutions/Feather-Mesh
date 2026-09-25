#!/usr/bin/env python3
"""Copy a stopped local research archive to private operator storage."""
import hashlib
import argparse
import json
import os
from pathlib import Path
import re
import shlex
import signal
import stat
import subprocess
import tarfile
import threading
from datetime import datetime, timedelta, timezone

import operator_vault as vault

REMOTE_ARCHIVE = '/home/feam-service-data/traces/collector/archive'
KEY = re.compile(r'^research/(events|exports|deletions)/[0-9a-f-]{36}/[0-9a-f]{64}\.json$')
HASH = re.compile(r'^[0-9a-f]{64}$')
MAX_OBJECT = 8 << 20
META = ('archive-proof.json', 'archive-ledger.json', 'copy-receipt.json')
MAINTENANCE = 'copy-maintenance.json'


def require(ok, message):
    if not ok:
        raise ValueError(message)


def encoded(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':')).encode() + b'\n'


def expected_files(proof, ledger):
    require(proof['protocol'] == 'feam.archive-verification.v1' and proof['status'] == 'verified' and
            ledger['protocol'] == 'feam.archive-ledger.v1', 'verified archive and ledger required')
    indexed = {}
    for item in proof['inventory']:
        key = item['object_key']
        require(KEY.fullmatch(key) and HASH.fullmatch(item['sha256']) and key not in indexed and
                item['state'] in ('verified', 'deleted'), 'unsafe or repeated archive inventory entry')
        if item['state'] == 'verified':
            indexed[key.replace('/', '_')] = item['sha256']
    ledger_objects = {item['object_key']: item for item in ledger['objects']}
    require(len(ledger_objects) == len(ledger['objects']), 'repeated archive ledger entry')
    ledger_deletions = {}
    for deletion in ledger['deletions']:
        body = json.dumps({'created_at': deletion['created_at'], 'participant_id': deletion['participant_id'],
                           'reason': deletion['reason']}, separators=(',', ':')).encode()
        key = 'research/deletions/' + deletion['participant_id'] + '/' + hashlib.sha256(body).hexdigest() + '.json'
        require(key not in ledger_deletions, 'repeated deletion lineage')
        ledger_deletions[key] = {'sha256': hashlib.sha256(body).hexdigest(), 'state': deletion['archive_state']}
    require(len(proof['inventory']) == len(ledger_objects) + len(ledger_deletions),
            'archive inventory differs from full provenance ledger')
    for item in proof['inventory']:
        if item['kind'] in ('batch', 'export'):
            obj = ledger_objects.get(item['object_key'])
            require(obj is not None and obj['sha256'] == item['sha256'] and obj['state'] == item['state'],
                    'archive proof differs from provenance ledger')
        else:
            deletion = ledger_deletions.get(item['object_key'])
            require(item['kind'] == 'deletion' and deletion is not None and
                    deletion['sha256'] == item['sha256'] and deletion['state'] == item['state'],
                    'archive proof differs from deletion ledger')
    require(sum(1 for item in proof['inventory'] if item['state'] == 'verified') == len(indexed),
            'flattened archive filenames collided')
    return indexed


def _check_file_info(info):
    require(stat.S_ISREG(info.st_mode) and info.st_uid == os.getuid() and not info.st_mode & 0o077 and
            info.st_nlink == 1, 'owner-only regular copy file required')


def verify_copy(directory, proof=None, ledger=None, allow_pending=False):
    directory = Path(directory)
    vault.private_directory(directory)
    exact_source_check = proof is not None or ledger is not None
    saved_proof = json.loads(vault.private_read(directory / META[0]))
    saved_ledger = json.loads(vault.private_read(directory / META[1]))
    expected = expected_files(saved_proof, saved_ledger)
    if exact_source_check:
        require(proof is not None and ledger is not None, 'both fresh archive proofs required')
        expected_files(proof, ledger)
        require({k: v for k, v in proof.items() if k != 'verified_at'} ==
                {k: v for k, v in saved_proof.items() if k != 'verified_at'} and
                {k: v for k, v in ledger.items() if k not in ('exported_at', 'sha256')} ==
                {k: v for k, v in saved_ledger.items() if k not in ('exported_at', 'sha256')},
                'fresh archive content or provenance differs from Mac copy')
    maintenance = {}
    if (directory / MAINTENANCE).exists():
        maintenance = json.loads(vault.private_read(directory / MAINTENANCE))
        require(maintenance.get('protocol') == 'feam.mac-archive-maintenance.v1' and
                isinstance(maintenance.get('removed'), list) and
                set(maintenance['removed']) <= set(expected) and
                maintenance.get('phase') in ('pending', 'complete'), 'invalid Mac archive maintenance record')
    removed = set(maintenance.get('removed', []))
    require(not exact_source_check or not removed, 'pruned Mac copy cannot prove full current archive')
    names = {p.name for p in directory.iterdir()}
    metadata = set(META) | ({MAINTENANCE} if maintenance else set())
    if maintenance and maintenance['phase'] == 'pending' and allow_pending:
        require(set(expected) - removed | metadata <= names <= set(expected) | metadata,
                'Mac archive has missing or unexpected files')
    else:
        require(not maintenance or maintenance['phase'] == 'complete', 'Mac archive maintenance interrupted; resume it')
        require(names == (set(expected) - removed) | metadata,
                'Mac archive has missing or unexpected files')
    for name, expected_hash in expected.items():
        if name in removed and name not in names:
            continue
        path = directory / name
        _check_file_info(path.lstat())
        hasher = hashlib.sha256()
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
        with os.fdopen(fd, 'rb') as source:
            _check_file_info(os.fstat(source.fileno()))
            for block in iter(lambda: source.read(1 << 20), b''):
                hasher.update(block)
        require(hasher.hexdigest() == expected_hash, 'Mac archive content hash differs')
    require(vault.private_read(directory / META[0]) == encoded(saved_proof) and
            vault.private_read(directory / META[1]) == encoded(saved_ledger), 'Mac archive metadata differs')
    receipt = json.loads(vault.private_read(directory / META[2]))
    require(receipt['protocol'] == 'feam.mac-archive-copy.v1' and receipt['archive_sha256'] == saved_proof['sha256'] and
            receipt['ledger_sha256'] == saved_ledger['sha256'] and receipt['objects'] == len(expected) and
            receipt['deployment_id'], 'Mac archive copy receipt differs')
    return receipt


def _replace_maintenance(directory, record):
    temporary = directory / ('.' + MAINTENANCE)
    require(not temporary.exists() and not temporary.is_symlink(),
            'interrupted maintenance metadata update requires private inspection')
    vault.new_private(temporary, encoded(record))
    os.replace(temporary, directory / MAINTENANCE)
    vault.fsync_directory(directory)


def _finish_pending(directory):
    path = directory / MAINTENANCE
    if not path.exists():
        return
    record = json.loads(vault.private_read(path))
    if record.get('phase') != 'pending':
        return
    verify_copy(directory, allow_pending=True)
    for name in record['removed']:
        object_path = directory / name
        if object_path.exists():
            _check_file_info(object_path.lstat())
            object_path.unlink()
    record['phase'] = 'complete'
    _replace_maintenance(directory, record)
    verify_copy(directory)


def _apply_removal(directory, removed, proof, ledger, now):
    record = {'protocol': 'feam.mac-archive-maintenance.v1', 'phase': 'pending',
              'removed': sorted(removed), 'current_archive_sha256': proof['sha256'],
              'current_ledger_sha256': ledger['sha256'], 'applied_at': now.isoformat()}
    _replace_maintenance(directory, record)  # deletion intent survives interruption
    _finish_pending(directory)
    record['phase'] = 'complete'
    return record


def maintain_copy(directory, current_proof, current_ledger, now=None):
    """Apply current withdrawals and expiry to an older retained Mac copy."""
    directory = Path(directory)
    _finish_pending(directory)
    verify_copy(directory)
    old_proof = json.loads(vault.private_read(directory / META[0]))
    old_ledger = json.loads(vault.private_read(directory / META[1]))
    old_files = expected_files(old_proof, old_ledger)
    current_files = expected_files(current_proof, current_ledger)
    current_objects = {v['object_key']: v for v in current_ledger['objects']}
    old_objects = {v['object_key']: v for v in old_ledger['objects']}
    now = now or datetime.now(timezone.utc)
    previous = set()
    if (directory / MAINTENANCE).exists():
        previous = set(json.loads(vault.private_read(directory / MAINTENANCE))['removed'])
    removed = set(previous)
    for name in old_files:
        key = name.replace('_', '/')
        if name in current_files:
            require(current_files[name] == old_files[name], 'archive object changed under retained key')
            continue
        old = old_objects.get(key)
        current = current_objects.get(key)
        if old:
            expired = datetime.fromisoformat(old['expires_at'].replace('Z', '+00:00')) <= now
            require(expired or (current and current['state'] == 'deleted'),
                    'current ledger does not authorize removal from Mac copy')
        else:
            require(key.startswith('research/deletions/') and
                    any(v['participant_id'] in key for v in old_ledger['deletions']),
                    'unknown deletion metadata on Mac copy')
            # Collector deletes tombstones after 30 days; an absent current
            # tombstone is therefore eligible only after the original deadline.
            entry = next(v for v in old_ledger['deletions'] if v['participant_id'] in key)
            require(datetime.fromisoformat(entry['created_at'].replace('Z', '+00:00')) + timedelta(days=30) <= now,
                    'unexpired deletion tombstone must remain on Mac')
        removed.add(name)
    return _apply_removal(directory, removed, current_proof, current_ledger, now)


def copy_stream(source, destination, proof, ledger, deployment_id):
    """Extract only expected regular objects from a quiesced tar stream."""
    destination = Path(destination)
    require(destination.is_absolute() and not destination.exists() and not destination.is_symlink(),
            'new absolute Mac copy directory required')
    vault.private_directory(destination.parent)
    expected = expected_files(proof, ledger)
    destination.mkdir(mode=0o700)
    seen = set()
    try:
        with tarfile.open(fileobj=source, mode='r|*') as archive:
            for member in archive:
                name = member.name.removeprefix('./')
                if member.isdir() and name in ('', '.'):
                    continue
                require(member.isfile() and name in expected and name not in seen and
                        0 <= member.size <= MAX_OBJECT, 'unexpected archive member; preserve source')
                seen.add(name)
                hasher = hashlib.sha256()
                fd = os.open(destination / name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
                with os.fdopen(fd, 'wb') as out, archive.extractfile(member) as incoming:
                    remaining = member.size
                    while remaining:
                        block = incoming.read(min(1 << 20, remaining))
                        require(bool(block), 'short archive transfer')
                        out.write(block)
                        hasher.update(block)
                        remaining -= len(block)
                    out.flush(); os.fsync(out.fileno())
                require(hasher.hexdigest() == expected[name], 'archive transfer content hash differs')
        require(seen == set(expected), 'archive transfer omitted an object')
        receipt = {'protocol': 'feam.mac-archive-copy.v1', 'deployment_id': deployment_id,
                   'archive_sha256': proof['sha256'], 'ledger_sha256': ledger['sha256'],
                   'objects': len(expected), 'verified_at': datetime.now(timezone.utc).isoformat()}
        vault.new_private(destination / META[0], encoded(proof))
        vault.new_private(destination / META[1], encoded(ledger))
        vault.new_private(destination / META[2], encoded(receipt))
        verify_copy(destination, proof, ledger)
        return receipt
    except Exception:
        # Retain the partial destination for inspection; never remove the host source.
        raise


def copy_process(proc, destination, proof, ledger, deployment_id, timeout=1800):
    timed_out = threading.Event()

    def stop():
        if proc.poll() is None:
            timed_out.set()
            try:
                os.killpg(proc.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass

    timer = threading.Timer(timeout, stop)
    timer.daemon = True
    timer.start()
    try:
        receipt = copy_stream(proc.stdout, destination, proof, ledger, deployment_id)
        require(proc.wait(timeout=30) == 0 and not timed_out.is_set(),
                'remote archive transfer failed or exceeded total deadline')
        return receipt
    finally:
        timer.cancel()
        if proc.poll() is None:
            try:
                os.killpg(proc.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            proc.wait()
        proc.stdout.close()


def remote_copy(ssh, destination, proof, ledger, deployment_id):
    argv = ['/usr/bin/ssh', '-T', '-oBatchMode=yes', '-oIdentitiesOnly=yes',
            '-oStrictHostKeyChecking=yes', '-oConnectTimeout=10',
            '-oServerAliveInterval=10', '-oServerAliveCountMax=2',
            '-oUserKnownHostsFile=' + ssh['known_hosts'], '-i', ssh['identity_file'],
            '-p', str(ssh['port']), '--', ssh['user'] + '@' + ssh['host'],
            shlex.join(['sudo', '-n', '--', '/usr/sbin/runuser', '-u', 'feam-collector', '--', '/usr/bin/tar',
                        '-C', REMOTE_ARCHIVE, '-cf', '-', '.'])]
    proc = subprocess.Popen(argv, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                            stderr=subprocess.DEVNULL, start_new_session=True)
    return copy_process(proc, destination, proof, ledger, deployment_id)


def expire_copy(directory, now=None):
    """Remove bytes past the original 30-day deadline after host teardown."""
    directory = Path(directory)
    _finish_pending(directory)
    verify_copy(directory)
    ledger = json.loads(vault.private_read(directory / META[1]))
    proof = json.loads(vault.private_read(directory / META[0]))
    expected = expected_files(proof, ledger)
    now = now or datetime.now(timezone.utc)
    removed = set()
    if (directory / MAINTENANCE).exists():
        removed = set(json.loads(vault.private_read(directory / MAINTENANCE))['removed'])
    for item in ledger['objects']:
        name = item['object_key'].replace('/', '_')
        if name in expected and datetime.fromisoformat(item['expires_at'].replace('Z', '+00:00')) <= now:
            removed.add(name)
    for item in ledger['deletions']:
        if datetime.fromisoformat(item['created_at'].replace('Z', '+00:00')) + timedelta(days=30) <= now:
            prefix = ('research/deletions/' + item['participant_id'] + '/').replace('/', '_')
            removed.update(name for name in expected if name.startswith(prefix))
    _apply_removal(directory, removed, proof, ledger, now)
    return len(removed)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['verify', 'expire'])
    parser.add_argument('--directory', type=Path, required=True)
    args = parser.parse_args()
    try:
        if args.action == 'verify':
            receipt = verify_copy(args.directory)
            print(json.dumps({'verified': True, 'objects': receipt['objects']}))
        else:
            print(json.dumps({'expired_or_withdrawn_objects': expire_copy(args.directory)}))
    except Exception:
        raise SystemExit('Mac archive check or maintenance failed; retain source and inspect private copy.') from None


if __name__ == '__main__':
    main()
