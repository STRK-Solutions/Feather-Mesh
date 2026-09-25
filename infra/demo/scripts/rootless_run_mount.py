#!/usr/bin/env python3
"""Expose only the verified FEAM tmpfs to an already-running rootless dockerd.

RootlessKit copies /run when it starts. On the first live U.04 installation the
runner daemon may predate run-feam.mount; a bind in dockerd's existing mount
namespace avoids restarting that daemon or its unrelated W1 containers.
"""
import argparse
import ctypes
import fcntl
import json
import os
import platform
from pathlib import Path
import re
import stat
import subprocess

SOURCE = Path('/run/feam')
HOST_REFERENCE = '/proc/1/root/run/feam'
SOCKET = 'unix:///run/user/2101/feam-docker.sock'
CGROUP = '/user.slice/user-2101.slice/user@2101.service/app.slice/docker.service'
DOCKER = '/usr/bin/dockerd'
NSENTER = '/usr/bin/nsenter'
RUNNER_UID = 2101
MAX_BYTES = 32 << 20
OPEN_TREE_SYSCALL = 428
MOVE_MOUNT_SYSCALL = 429
OPEN_TREE_CLONE = 1
AT_EMPTY_PATH = 0x1000
MOVE_MOUNT_F_EMPTY_PATH = 0x4
MOVE_MOUNT_T_EMPTY_PATH = 0x40
MOVE_CODE = '''import ctypes,os,stat,sys
fd=int(sys.argv[1]); target='/run/feam'
info=os.lstat(target); parent=os.stat('/run')
if not stat.S_ISDIR(info.st_mode) or info.st_dev != parent.st_dev or info.st_uid != 0 or stat.S_IMODE(info.st_mode) != 0o700:
    raise ValueError('new daemon target identity changed')
target_fd=os.open(target,os.O_PATH|os.O_DIRECTORY|os.O_NOFOLLOW|os.O_CLOEXEC)
try:
    libc=ctypes.CDLL(None,use_errno=True); libc.syscall.restype=ctypes.c_long
    result=libc.syscall(429,ctypes.c_int(fd),ctypes.c_char_p(b''),ctypes.c_int(target_fd),ctypes.c_char_p(b''),ctypes.c_uint(0x4|0x40))
    if result != 0: raise OSError(ctypes.get_errno(),os.strerror(ctypes.get_errno()))
finally: os.close(target_fd)
'''


def require(ok, reason):
    if not ok:
        raise ValueError(reason)


def mount(path, text):
    matches = []
    for line in text.splitlines():
        halves = line.split(' - ', 1)
        if len(halves) != 2:
            continue
        left, right = halves
        fields, data = left.split(), right.split()
        if len(fields) >= 5 and len(data) >= 3 and fields[4] == path:
            matches.append((fields, data))
    require(len(matches) == 1, 'exactly one reviewed mount required')
    return matches[0]


def daemon():
    matches = []
    for proc in Path('/proc').iterdir():
        if not proc.name.isdecimal():
            continue
        try:
            info = (proc / 'status').read_text()
            uid = int(re.search(r'^Uid:\s+(\d+)', info, re.M).group(1))
            argv = (proc / 'cmdline').read_bytes().rstrip(b'\0').split(b'\0')
            cgroup = (proc / 'cgroup').read_text().strip()
            if (uid == RUNNER_UID and argv[:2] == [b'dockerd', ('--host=' + SOCKET).encode()]
                    and os.readlink(proc / 'exe') == DOCKER
                    and cgroup == '0::' + CGROUP
                    and b'--data-root=/home/feam-service-data/runtime/docker' in argv
                    and b'--exec-root=/run/user/2101/feam-docker' in argv):
                matches.append(int(proc.name))
        except (OSError, AttributeError, ValueError):
            continue
    require(len(matches) == 1, 'one pinned live runner dockerd required')
    return matches[0]


def source():
    require(os.geteuid() == 0 and os.stat('/proc/self/ns/mnt').st_ino == os.stat('/proc/1/ns/mnt').st_ino,
            'host root mount namespace required')
    info = SOURCE.lstat()
    require(stat.S_ISDIR(info.st_mode) and info.st_uid == 0 and info.st_gid == 0 and
            SOURCE.resolve() == SOURCE and SOURCE.is_mount(), 'fixed root-owned FEAM mount required')
    fields, data = mount(str(SOURCE), Path('/proc/1/mountinfo').read_text())
    size = os.statvfs(SOURCE)
    require(data[:2] == ['tmpfs', 'tmpfs'] and size.f_blocks * size.f_frsize == MAX_BYTES and
            'size=32768k' in data[2].split(',') and
            'noexec' in fields[5].split(',') and 'nosuid' in fields[5].split(',') and
            'nodev' in fields[5].split(','), 'bounded FEAM tmpfs identity changed')
    ref = os.stat(HOST_REFERENCE)
    require((ref.st_dev, ref.st_ino) == (info.st_dev, info.st_ino), 'host mount reference changed')
    return info, fields[0], fields[2]


def target_mounts(pid, device):
    """Ignore the host FEAM mount hidden beneath RootlessKit's copied-up /run."""
    rows = []
    for line in Path(f'/proc/{pid}/mountinfo').read_text().splitlines():
        halves = line.split(' - ', 1)
        if len(halves) == 2:
            rows.append((halves[0].split(), halves[1].split()))
    run = os.stat(f'/proc/{pid}/root/run')
    run_device = f'{os.major(run.st_dev)}:{os.minor(run.st_dev)}'
    visible_run = [fields[0] for fields, _ in rows
                   if len(fields) >= 5 and fields[4] == '/run' and fields[2] == run_device]
    require(len(visible_run) == 1, 'one visible daemon-private /run mount required')
    visible, hidden = [], []
    for fields, data in rows:
        if len(fields) < 5 or fields[4] != str(SOURCE):
            continue
        if fields[1] == visible_run[0]:
            visible.append((fields, data))
        else:
            hidden.append((fields, data))
    require(all(fields[2] == device and data[:2] == ['tmpfs', 'tmpfs'] for fields, data in hidden),
            'foreign hidden daemon FEAM mount')
    require(len(visible) <= 1, 'ambiguous visible daemon FEAM mount')
    return visible


def inspect(pid, info, device):
    current = daemon()
    require(current == pid, 'runner daemon identity changed')
    ns_path = f'/proc/{pid}/ns/mnt'
    ns = os.stat(ns_path)
    host_ns = os.stat('/proc/1/ns/mnt')
    require(ns.st_ino != host_ns.st_ino, 'daemon unexpectedly shares host mount namespace')
    target = Path(f'/proc/{pid}/root/run/feam')
    if not target.exists() and not target.is_symlink():
        require(not target_mounts(pid, device), 'visible daemon FEAM mount without target')
        return ns, False
    require(not target.is_symlink(), 'foreign daemon FEAM target')
    actual = target.stat()
    visible = target_mounts(pid, device)
    require(len(visible) == 1, 'foreign daemon FEAM target is not a verified mount')
    fields, data = visible[0]
    require(stat.S_ISDIR(actual.st_mode) and data[:2] == ['tmpfs', 'tmpfs'] and
            fields[2] == device and (actual.st_dev, actual.st_ino) == (info.st_dev, info.st_ino),
            'foreign daemon FEAM mount; refuse replacement')
    return ns, True


def mapped_command(mount_fd, user_fd, source_fd, argv):
    result = subprocess.run([NSENTER, '--user=/proc/self/fd/' + str(user_fd),
                             '--mount=/proc/self/fd/' + str(mount_fd),
                             '--setuid', '0', '--setgid', '0', '--', *argv],
                            pass_fds=(mount_fd, user_fd, source_fd),
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                            timeout=10, check=False)
    require(result.returncode == 0,
            'fixed daemon namespace operation failed: ' + result.stderr[:500].decode(errors='replace'))


def detached_mount(source_fd, info):
    require(platform.machine() == 'x86_64', 'reviewed x86_64 syscall ABI required')
    require((os.fstat(source_fd).st_dev, os.fstat(source_fd).st_ino) ==
            (info.st_dev, info.st_ino), 'pinned source FD changed before clone')
    libc = ctypes.CDLL(None, use_errno=True)
    libc.syscall.restype = ctypes.c_long
    cloned = libc.syscall(OPEN_TREE_SYSCALL, ctypes.c_int(source_fd), ctypes.c_char_p(b''),
                          ctypes.c_uint(OPEN_TREE_CLONE | os.O_CLOEXEC | AT_EMPTY_PATH))
    if cloned < 0:
        error = ctypes.get_errno()
        raise OSError(error, 'fixed host FEAM mount clone failed: ' + os.strerror(error))
    require((os.fstat(cloned).st_dev, os.fstat(cloned).st_ino) ==
            (info.st_dev, info.st_ino), 'detached FEAM mount identity changed')
    return cloned


def namespace_source(mount_fd, user_fd, source_fd, info):
    result = subprocess.run([NSENTER, '--user=/proc/self/fd/' + str(user_fd),
                             '--mount=/proc/self/fd/' + str(mount_fd),
                             '--setuid', '0', '--setgid', '0', '--',
                             '/usr/bin/stat', '-Lc', '%d:%i', '/proc/self/fd/' + str(source_fd)],
                            pass_fds=(mount_fd, user_fd, source_fd),
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                            timeout=10, check=False)
    require(result.returncode == 0 and
            result.stdout.strip() == f'{info.st_dev}:{info.st_ino}'.encode(),
            'host FEAM mount reference differs inside daemon namespace')


def repair(check_only=False):
    info, mount_id, device = source()
    pid = daemon()
    ns, visible = inspect(pid, info, device)
    if visible:
        return {'protocol': 'feam.rootless-run-mount.v1', 'status': 'already_visible',
                'host_mount_id': mount_id, 'daemon_pid': pid, 'daemon_mount_namespace': ns.st_ino}
    require(not check_only, 'daemon cannot see FEAM tmpfs; explicit repair required')
    mount_fd = os.open(f'/proc/{pid}/ns/mnt', os.O_RDONLY | os.O_CLOEXEC)
    user_fd = os.open(f'/proc/{pid}/ns/user', os.O_RDONLY | os.O_CLOEXEC)
    source_fd = os.open(SOURCE, os.O_PATH | os.O_DIRECTORY | os.O_CLOEXEC)
    try:
        require(os.fstat(mount_fd).st_ino == ns.st_ino and
                os.fstat(user_fd).st_ino == os.stat(f'/proc/{pid}/ns/user').st_ino and
                os.fstat(user_fd).st_ino != os.stat('/proc/1/ns/user').st_ino and
                daemon() == pid, 'daemon namespace changed')
        current, current_id, current_device = source()
        require((current.st_dev, current.st_ino, current_id, current_device) ==
                (info.st_dev, info.st_ino, mount_id, device), 'host FEAM source changed')
        require((os.fstat(source_fd).st_dev, os.fstat(source_fd).st_ino) ==
                (info.st_dev, info.st_ino), 'pinned host FEAM source changed')
        namespace_source(mount_fd, user_fd, source_fd, info)
        # The target is absent, and both argv vectors are fixed. A failed bind
        # leaves the host tmpfs and all existing containers untouched.
        mapped_command(mount_fd, user_fd, source_fd, ['/usr/bin/mkdir', '-m', '0700', str(SOURCE)])
        try:
            namespace_source(mount_fd, user_fd, source_fd, info)
            current, current_id, current_device = source()
            require((current.st_dev, current.st_ino, current_id, current_device) ==
                    (info.st_dev, info.st_ino, mount_id, device), 'host FEAM source changed before clone')
            cloned = detached_mount(source_fd, info)
            try:
                mapped_command(mount_fd, user_fd, cloned,
                               ['/usr/bin/python3', '-c', MOVE_CODE, str(cloned)])
            finally:
                os.close(cloned)
        except Exception:
            # Remove only our new empty target if the mount did not succeed.
            if not target_mounts(pid, device):
                mapped_command(mount_fd, user_fd, source_fd, ['/usr/bin/rmdir', str(SOURCE)])
            raise
        require(os.fstat(mount_fd).st_ino == ns.st_ino, 'daemon namespace changed after bind')
        _, visible = inspect(pid, info, device)
        require(visible, 'daemon FEAM bind not independently visible')
        return {'protocol': 'feam.rootless-run-mount.v1', 'status': 'bound',
                'host_mount_id': mount_id, 'daemon_pid': pid, 'daemon_mount_namespace': ns.st_ino}
    finally:
        os.close(source_fd)
        os.close(user_fd)
        os.close(mount_fd)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true', help='verify visibility without changing the namespace')
    args = parser.parse_args()
    try:
        fd = os.open('/run/lock/feam-rootless-run-mount.lock', os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, 'rb+') as lock:
            info = os.fstat(lock.fileno())
            require(stat.S_ISREG(info.st_mode) and info.st_uid == 0 and not info.st_mode & 0o077,
                    'private root-only namespace repair lock required')
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            print(json.dumps(repair(args.check), sort_keys=True))
    except Exception:
        raise SystemExit('Rootless FEAM mount visibility failed; no Docker restart or workspace retry attempted.') from None


if __name__ == '__main__':
    main()
