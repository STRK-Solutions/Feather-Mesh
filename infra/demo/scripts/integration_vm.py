#!/usr/bin/env python3
"""Separate full-pool W7 guest; reuse verified W0 tools without changing its VM."""
import argparse
import base64
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import platform
import resource
import socket
import subprocess

spec = importlib.util.spec_from_file_location('feam_w0_vm', Path(__file__).with_name('disposable_vm.py'))
base = importlib.util.module_from_spec(spec)
spec.loader.exec_module(base)
PARENT = Path.home() / 'feam-w0-vm'
ROOT = PARENT / 'w7-integration'
SCOPE = 'feam-w7-integration-v1'
PORT = 22372
GIB = 1024**3
DISK_GIB = 384
MEMORY_MIB = 4096


def require(condition, message):
    if not condition:
        raise ValueError(message)


def private_file(path):
    info = path.lstat()
    require(not path.is_symlink() and path.is_file() and info.st_uid == os.getuid() and not info.st_mode & 0o077,
            'private owned file required')


def scope(create=False):
    require(os.getuid() != 0 and platform.machine() == 'x86_64', 'unprivileged Ubuntu amd64 owner required')
    release = Path('/etc/os-release').read_text()
    require('ID=ubuntu' in release and 'VERSION_ID="24.04"' in release, 'Ubuntu 24.04 required')
    require(PARENT.resolve() == PARENT and PARENT.is_dir() and PARENT.stat().st_uid == os.getuid(), 'unknown W0 parent')
    private_file(PARENT / 'ownership.json')
    require(json.loads((PARENT / 'ownership.json').read_text()) == {'scope': base.SCOPE, 'uid': os.getuid()}, 'W0 owner mismatch')
    expected = {'scope': SCOPE, 'uid': os.getuid()}
    require(not ROOT.is_symlink(), 'symlink integration scope refused')
    if ROOT.exists():
        require(ROOT.is_dir() and ROOT.stat().st_uid == os.getuid() and not ROOT.stat().st_mode & 0o077, 'unsafe integration directory')
        private_file(ROOT / 'ownership.json')
        require(json.loads((ROOT / 'ownership.json').read_text()) == expected, 'unknown integration directory')
    else:
        require(create, 'integration guest has not been prepared')
        ROOT.mkdir(mode=0o700)
        (ROOT / 'ownership.json').write_text(json.dumps(expected) + '\n')
        (ROOT / 'ownership.json').chmod(0o600)
    os.umask(0o077)


def tool_env():
    libraries = PARENT / 'packages-root/usr/lib/x86_64-linux-gnu'
    return dict(os.environ, LD_LIBRARY_PATH=str(libraries), QEMU_MODULE_DIR=str(libraries / 'qemu'))


def disk_headroom():
    stats = os.statvfs(PARENT)
    free, total = stats.f_bavail * stats.f_frsize, stats.f_blocks * stats.f_frsize
    # The retained W0 VM and physical-host service storage stay outside this cap.
    require(free >= 390 * GIB + max(20 * GIB, (total * 15 + 99) // 100),
            'host cannot cover guest maximum plus its own storage floor')


def prepare(probe):
    require(not (ROOT / 'vm.qcow2').exists() and not (ROOT / 'vm.qcow2').is_symlink(), 'never reinitialize an existing guest disk')
    disk_headroom()
    image = PARENT / base.IMAGE
    require(not image.is_symlink() and base.sha(image) == base.IMAGE_SHA, 'cached base image digest mismatch')
    probe_bytes = probe.read_bytes()
    require(probe.is_file() and not probe.is_symlink() and len(probe_bytes) < 64 * 1024, 'bounded reviewed probe required')
    for name in ('guest_access', 'guest_host'):
        require(not (ROOT / name).exists(), 'partial preparation requires explicit reconciliation')
        base.run(['ssh-keygen', '-q', '-t', 'ed25519', '-N', '', '-C', SCOPE, '-f', str(ROOT / name)])
    public = (ROOT / 'guest_access.pub').read_text().strip()
    host_public = (ROOT / 'guest_host.pub').read_text().strip()
    seed = ROOT / 'seed'
    seed.mkdir(mode=0o700)
    config = {
        'hostname': 'feam-w7-integration', 'ssh_pwauth': False, 'disable_root': True,
        'users': [{'name': 'feamtest', 'shell': '/bin/bash', 'sudo': ['ALL=(ALL) NOPASSWD:ALL'],
                   'lock_passwd': True, 'ssh_authorized_keys': [public]}],
        'ssh_keys': {'ed25519_private': (ROOT / 'guest_host').read_text(), 'ed25519_public': host_public},
        'write_files': [{'path': '/opt/feam-w7/full_vm_probe.sh', 'permissions': '0700', 'encoding': 'b64',
                         'content': base64.b64encode(probe_bytes).decode()}],
        'runcmd': [['sh', '-c', 'bash /opt/feam-w7/full_vm_probe.sh --confirm-disposable > /var/log/feam-w7-probe.log 2>&1; echo $? > /var/log/feam-w7-probe.exit']],
    }
    (seed / 'user-data').write_text('#cloud-config\n' + json.dumps(config) + '\n')
    (seed / 'meta-data').write_text('instance-id: ' + SCOPE + '\nlocal-hostname: feam-w7-integration\n')
    (ROOT / 'known_hosts').write_text('[127.0.0.1]:' + str(PORT) + ' ' + host_public + '\n')
    tools = PARENT / 'packages-root/usr/bin'
    base.run([str(tools / 'genisoimage'), '-quiet', '-output', str(ROOT / 'seed.iso'), '-volid', 'cidata',
              '-joliet', '-rock', 'user-data', 'meta-data'], cwd=seed, env=tool_env())
    base.run([str(tools / 'qemu-img'), 'create', '-f', 'qcow2', '-F', 'qcow2', '-b', str(image),
              str(ROOT / 'vm.qcow2'), str(DISK_GIB) + 'G'], env=tool_env())
    prepared = {'scope': SCOPE, 'image_sha256': base.IMAGE_SHA, 'probe_sha256': hashlib.sha256(probe_bytes).hexdigest(),
                'disk_max_gib': DISK_GIB, 'memory_mib': MEMORY_MIB, 'vcpus': 2, 'ssh_port': PORT, 'acceleration': 'tcg'}
    (ROOT / 'prepared.json').write_text(json.dumps(prepared, indent=2) + '\n')
    print('Prepared separate W7 full-pool guest; existing W0 disk and host services unchanged.')


def running_pid():
    pidfile = ROOT / 'qemu.pid'
    if not pidfile.exists():
        return None
    private_file(pidfile)
    pid = int(pidfile.read_text().strip())
    require(pid > 1, 'invalid guest PID')
    cmdline = Path('/proc') / str(pid) / 'cmdline'
    if not cmdline.exists():
        return None
    require(str(ROOT / 'vm.qcow2').encode() in cmdline.read_bytes().split(b'\x00') or
            ('file=' + str(ROOT / 'vm.qcow2') + ',if=virtio,format=qcow2').encode() in cmdline.read_bytes().split(b'\x00'),
            'PID belongs to a different process')
    return pid


def start():
    if running_pid():
        print('W7 guest already running')
        return
    private_file(ROOT / 'prepared.json')
    prepared = json.loads((ROOT / 'prepared.json').read_text())
    require(all(prepared.get(k) == v for k, v in {'scope': SCOPE, 'image_sha256': base.IMAGE_SHA,
                'disk_max_gib': DISK_GIB, 'memory_mib': MEMORY_MIB, 'vcpus': 2, 'ssh_port': PORT, 'acceleration': 'tcg'}.items()),
            'prepared guest bounds changed')
    private_file(ROOT / 'vm.qcow2')
    disk_headroom()
    memory = {line.split(':')[0]: int(line.split()[1]) for line in Path('/proc/meminfo').read_text().splitlines() if ':' in line}
    require(memory.get('MemAvailable', 0) >= 6 * 1024**2, 'host requires 6 GiB available before guest start')
    with socket.socket() as check:
        check.bind(('127.0.0.1', PORT))
    packages = PARENT / 'packages-root'
    command = [str(packages / 'usr/bin/qemu-system-x86_64'), '-name', 'feam-w7-integration',
               '-machine', 'q35,accel=tcg', '-cpu', 'max', '-smp', '2', '-m', str(MEMORY_MIB),
               '-L', str(packages / 'usr/share/qemu'), '-bios', str(packages / 'usr/share/seabios/bios-256k.bin'),
               '-drive', 'file=' + str(ROOT / 'vm.qcow2') + ',if=virtio,format=qcow2',
               '-drive', 'file=' + str(ROOT / 'seed.iso') + ',media=cdrom,readonly=on',
               '-netdev', 'user,id=net0,hostfwd=tcp:127.0.0.1:' + str(PORT) + '-:22',
               '-device', 'virtio-net-pci,netdev=net0,romfile=', '-display', 'none', '-vga', 'none',
               '-serial', 'file:' + str(ROOT / 'serial.log'), '-monitor', 'none',
               '-qmp', 'unix:' + str(ROOT / 'qmp.sock') + ',server=on,wait=off',
               '-pidfile', str(ROOT / 'qemu.pid'), '-daemonize']
    def limits():
        os.nice(10)
        resource.setrlimit(resource.RLIMIT_FSIZE, (390 * GIB, 390 * GIB))
    base.run(command, env=tool_env(), preexec_fn=limits)
    print('W7 guest started: 2 vCPU, 4 GiB RAM, 384 GiB logical disk; localhost-only SSH.')


def stop():
    if not running_pid():
        print('W7 guest already stopped')
        return
    # Reuse only the exact-PID QMP powerdown protocol, with the new owned scope.
    original_root, original_pid = base.ROOT, base.running_pid
    try:
        base.ROOT, base.running_pid = ROOT, running_pid
        base.stop()
    finally:
        base.ROOT, base.running_pid = original_root, original_pid


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['prepare', 'start', 'status', 'stop'])
    parser.add_argument('--probe', type=Path)
    args = parser.parse_args()
    scope(args.action == 'prepare')
    if args.action == 'prepare':
        require(args.probe is not None, 'prepare requires reviewed --probe')
        prepare(args.probe)
    elif args.action == 'start':
        start()
    elif args.action == 'stop':
        stop()
    else:
        print('running' if running_pid() else 'stopped')


if __name__ == '__main__':
    main()
