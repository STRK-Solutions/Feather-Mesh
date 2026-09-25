#!/usr/bin/env python3
"""Disposable QEMU regression for lazy inode initialization and owned repair."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import uuid

if os.geteuid() != 0 or sys.argv[1:] != ['--confirm-disposable']:
    raise SystemExit('explicit disposable root invocation required')
if (subprocess.check_output(['systemd-detect-virt', '--vm'], text=True).strip() != 'qemu'
        or Path('/etc/hostname').read_text().strip() != 'feam-w0-disposable'):
    raise SystemExit('refusing non-disposable host')
spec = importlib.util.spec_from_file_location('storage', '/usr/local/sbin/feam-w1-storage')
if spec is None:
    from importlib.machinery import SourceFileLoader
    spec = importlib.util.spec_from_loader('storage', SourceFileLoader('storage', '/usr/local/sbin/feam-w1-storage'))
storage = importlib.util.module_from_spec(spec)
spec.loader.exec_module(storage)

report = {}
with tempfile.TemporaryDirectory(prefix='feam-w1-reservation-', dir='/var/tmp') as temporary:
    root = Path(temporary)
    mount = root/'mount'
    mount.mkdir()
    size = 256*1024**2
    for mode in ('legacy', 'eager'):
        backing = root/(mode+'.ext4')
        fd = os.open(backing, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
        os.posix_fallocate(fd, 0, size)
        os.fsync(fd)
        os.close(fd)
        identity = backing.stat().st_ino
        fs_uuid = str(uuid.uuid4())
        if mode == 'legacy':
            subprocess.run(['mkfs.ext4', '-q', '-E', 'nodiscard,lazy_itable_init=1',
                            '-U', fs_uuid, str(backing)], check=True)
        else:
            storage.format_new(backing, fs_uuid)
        subprocess.run(['mount', '-o', 'loop,nodiscard,init_itable=0', str(backing), str(mount)], check=True)
        try:
            (mount/'preserve').write_text('preserve existing bytes\n')
            subprocess.run(['sync', '-f', str(mount)], check=True)
            for attempt in range(30):
                try:
                    storage.initialized(backing)
                    break
                except ValueError:
                    time.sleep(1)
            storage.initialized(backing)
            before = backing.stat().st_blocks*512
            storage.reserve(backing, size)
            time.sleep(2)
            assert backing.stat().st_blocks*512 >= size
            assert backing.stat().st_ino == identity
            assert storage.run('blkid', '-p', '-s', 'UUID', '-o', 'value', str(backing)) == fs_uuid
            assert (mount/'preserve').read_text() == 'preserve existing bytes\n'
            report[mode] = {'bytes': size, 'allocated_before_repair': before,
                            'fully_reserved_after': True, 'inode_uuid_data_preserved': True}
            if mode == 'eager':
                assert before >= size
        finally:
            subprocess.run(['umount', str(mount)], check=True)
        backing.unlink()
print(json.dumps(report))
