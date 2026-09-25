#!/usr/bin/env python3
"""Encrypt private operator JSON outside disposable compute; never print secrets."""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import stat
import tempfile

from cryptography.hazmat.primitives.ciphers.aead import AESGCM

MAGIC = b'FEAM-OPERATOR-V1\n'
MAX_BYTES = 4 * 1024 * 1024


def private_directory(path):
    path = Path(path)
    if not path.is_absolute() or path.resolve() != path:
        raise ValueError('absolute non-symlink path required')
    info = path.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_uid != os.getuid() or info.st_mode & 0o077:
        raise ValueError('existing owner-only directory required')


def private_read(path, limit=MAX_BYTES):
    path = Path(path)
    private_directory(path.parent)
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'rb') as source:
        info = os.fstat(source.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or info.st_mode & 0o077 or info.st_nlink != 1 or info.st_size > limit:
            raise ValueError('bounded owner-only regular file required')
        value = source.read(limit + 1)
    if len(value) > limit:
        raise ValueError('operator record too large')
    return value


def new_private(path, value):
    path = Path(path)
    private_directory(path.parent)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as output:
        output.write(value)
        output.flush()
        os.fsync(output.fileno())
    fsync_directory(path.parent)


def fsync_directory(path):
    fd = os.open(path, os.O_RDONLY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def keygen(path):
    new_private(path, AESGCM.generate_key(bit_length=256))


def key(path):
    value = private_read(path, 32)
    if len(value) != 32:
        raise ValueError('invalid operator encryption key')
    return value


def decode(path, key_path):
    value = private_read(path, MAX_BYTES + 128)
    if not value.startswith(MAGIC) or len(value) < len(MAGIC) + 28:
        raise ValueError('invalid vault version')
    raw = AESGCM(key(key_path)).decrypt(value[len(MAGIC):len(MAGIC)+12], value[len(MAGIC)+12:], MAGIC)
    record = json.loads(raw)
    if set(record) != {'protocol', 'revision', 'configuration'} or record['protocol'] != 'feam.operator.v1' or type(record['revision']) is not int or record['revision'] < 1 or not isinstance(record['configuration'], dict):
        raise ValueError('invalid encrypted operator record')
    return record


def seal(source, destination, key_path, expected_revision):
    destination = Path(destination)
    private_directory(destination.parent)
    configuration = json.loads(private_read(source))
    if not isinstance(configuration, dict):
        raise ValueError('operator configuration must be an object')
    lock_path = destination.with_suffix(destination.suffix + '.lock')
    fd = os.open(lock_path, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'r+b') as lock:
        info = os.fstat(lock.fileno())
        if info.st_uid != os.getuid() or info.st_mode & 0o077 or info.st_nlink != 1:
            raise ValueError('unsafe operator lock')
        fcntl.flock(lock, fcntl.LOCK_EX)
        revision = 0
        if destination.exists() or destination.is_symlink():
            revision = decode(destination, key_path)['revision']
        if revision != expected_revision:
            raise ValueError('stale operator revision; no overwrite')
        record = {'protocol': 'feam.operator.v1', 'revision': revision+1, 'configuration': configuration}
        raw = json.dumps(record, sort_keys=True, separators=(',', ':')).encode()
        if len(raw) > MAX_BYTES:
            raise ValueError('operator record too large')
        nonce = os.urandom(12)
        encrypted = MAGIC + nonce + AESGCM(key(key_path)).encrypt(nonce, raw, MAGIC)
        if revision:
            # Keep every prior encrypted revision. A failed attempt never erases
            # the only valid snapshot; restore is an explicit operator action.
            backup = destination.with_name(destination.name + f'.revision-{revision}')
            old = private_read(destination, MAX_BYTES+128)
            if backup.exists():
                if private_read(backup, MAX_BYTES+128) != old:
                    raise ValueError('conflicting retained revision')
            else:
                new_private(backup, old)
        temporary = None
        try:
            with tempfile.NamedTemporaryFile(dir=destination.parent, prefix='.vault-', delete=False) as out:
                temporary = Path(out.name)
                os.chmod(temporary, 0o600)
                out.write(encrypted); out.flush(); os.fsync(out.fileno())
            os.replace(temporary, destination)
            fsync_directory(destination.parent)
        finally:
            if temporary is not None:
                temporary.unlink(missing_ok=True)
        return {'revision': revision+1, 'sha256': hashlib.sha256(encrypted).hexdigest()}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['keygen', 'seal', 'open', 'verify'])
    parser.add_argument('--key', type=Path, required=True)
    parser.add_argument('--input', type=Path)
    parser.add_argument('--output', type=Path)
    parser.add_argument('--expected-revision', type=int, default=0)
    args = parser.parse_args()
    try:
        if args.action == 'keygen':
            keygen(args.key)
        elif args.action == 'seal':
            print(json.dumps(seal(args.input, args.output, args.key, args.expected_revision)))
        else:
            record = decode(args.input, args.key)
            if args.action == 'open':
                new_private(args.output, json.dumps(record['configuration'], indent=2).encode()+b'\n')
            print(json.dumps({'verified_revision': record['revision']}))
    except Exception:
        raise SystemExit('Operator vault operation failed; retain existing files and inspect inputs privately.') from None


if __name__ == '__main__':
    main()
