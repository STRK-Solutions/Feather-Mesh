#!/usr/bin/env python3
"""Owner-authorized, unprivileged Ubuntu W0 VM; never installs host packages.

Run on Ubuntu 24.04 x86-64. Uses only ~/feam-w0-vm and localhost port 22371.
The Ubuntu packages are extracted, not installed; no maintainer scripts run.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import platform
import resource
import socket
import subprocess
import urllib.request

ROOT = Path.home() / "feam-w0-vm"
SCOPE = "feam-w0-disposable-v1"
IMAGE_BASE = "https://cloud-images.ubuntu.com/releases/noble/release-20260911/"
IMAGE = "ubuntu-24.04-server-cloudimg-amd64.img"
IMAGE_SHA = "612b2c0cc1bc413a6cb8c38fd611794caf0f2b436c50013d8b3794db12ad7354"
PACKAGE_LOCK = Path(__file__).with_name("ubuntu-vm-packages.lock.json")


def run(args, **kwargs):
    return subprocess.run(args, check=True, text=True, timeout=300, **kwargs)


def sha(path):
    with path.open("rb") as source:
        return hashlib.file_digest(source, "sha256").hexdigest()


def scope():
    if os.getuid() == 0 or platform.machine() != "x86_64":
        raise SystemExit("requires an unprivileged x86-64 Ubuntu operator")
    release = Path("/etc/os-release").read_text()
    if 'ID=ubuntu' not in release or 'VERSION_ID="24.04"' not in release:
        raise SystemExit("requires Ubuntu 24.04")
    if ROOT.is_symlink():
        raise SystemExit("refusing symlink scope")
    marker = ROOT / "ownership.json"
    expected = {"scope": SCOPE, "uid": os.getuid()}
    if ROOT.exists():
        if ROOT.stat().st_uid != os.getuid() or not marker.is_file() or json.loads(marker.read_text()) != expected:
            raise SystemExit("refusing unowned or unknown VM directory")
    else:
        ROOT.mkdir(mode=0o700)
        marker.write_text(json.dumps(expected) + "\n")
    os.umask(0o077)


def tool_env():
    libraries = ROOT / "packages-root/usr/lib/x86_64-linux-gnu"
    return dict(os.environ, LD_LIBRARY_PATH=str(libraries), QEMU_MODULE_DIR=str(libraries / "qemu"))


def download(url, dest, limit):
    if dest.exists():
        return
    partial = dest.with_suffix(dest.suffix + ".partial")
    size = 0
    with urllib.request.urlopen(url, timeout=60) as response, partial.open("wb") as output:
        while chunk := response.read(1024 * 1024):
            size += len(chunk)
            if size > limit:
                raise ValueError("download exceeds bound")
            output.write(chunk)
    partial.rename(dest)


def prepare(probe):
    if (ROOT / "vm.qcow2").exists():
        raise SystemExit("VM already prepared; use status/start, never reinitialize its disk")
    if os.statvfs(ROOT).f_bavail * os.statvfs(ROOT).f_frsize < 30 * 1024**3:
        raise SystemExit("requires 30 GiB free in the dedicated parent filesystem")
    packages = ROOT / "debs"
    packages.mkdir(exist_ok=True)
    extracted = ROOT / "packages-root"
    extracted.mkdir(exist_ok=True)
    manifest = []
    lock = json.loads(PACKAGE_LOCK.read_text())
    if (lock["schema_version"], lock["os"], lock["architecture"]) != (1, "ubuntu-24.04", "amd64"):
        raise SystemExit("incompatible VM package lock")
    for pinned in lock["packages"]:
        package, version = pinned["package"], pinned["version"]
        run(["apt-get", "download", f"{package}={version}"], cwd=packages)
        candidates = list(packages.glob(package + "_*.deb"))
        matches = [path for path in candidates if sha(path) == pinned["sha256"]]
        if len(matches) != 1:
            raise ValueError("package checksum mismatch or ambiguous package")
        archive = matches[0]
        if archive.stat().st_size > 100 * 1024**2:
            raise ValueError("package exceeds size limit")
        run(["dpkg-deb", "--extract", str(archive), str(extracted)])
        manifest.append({"package": package, "version": version, "sha256": sha(archive)})
    (ROOT / "packages.json").write_text(json.dumps(manifest, indent=2) + "\n")
    for filename in ("SHA256SUMS", "SHA256SUMS.gpg"):
        download(IMAGE_BASE + filename, ROOT / filename, 1024**2)
    run(["gpgv", "--keyring", "/usr/share/keyrings/ubuntu-cloudimage-keyring.gpg",
         str(ROOT / "SHA256SUMS.gpg"), str(ROOT / "SHA256SUMS")])
    if f"{IMAGE_SHA} *{IMAGE}" not in (ROOT / "SHA256SUMS").read_text().splitlines():
        raise ValueError("pinned image missing from signed checksum manifest")
    download(IMAGE_BASE + IMAGE, ROOT / IMAGE, 1024**3)
    if sha(ROOT / IMAGE) != IMAGE_SHA:
        raise ValueError("image checksum mismatch")
    for name in ("guest_access", "guest_host"):
        if not (ROOT / name).exists():
            run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "feam-w0-disposable",
                 "-f", str(ROOT / name)])
    seed = ROOT / "seed"
    seed.mkdir(exist_ok=True)
    public = (ROOT / "guest_access.pub").read_text().strip()
    host_public = (ROOT / "guest_host.pub").read_text().strip()
    host_private = (ROOT / "guest_host").read_text()
    # JSON is valid YAML, after the cloud-config header. No production identity.
    config = {"hostname": "feam-w0-disposable", "ssh_pwauth": False,
              "disable_root": True, "users": [{"name": "feamtest", "shell": "/bin/bash",
              "sudo": ["ALL=(ALL) NOPASSWD:ALL"], "lock_passwd": True,
              "ssh_authorized_keys": [public]}],
              "ssh_keys": {"ed25519_private": host_private, "ed25519_public": host_public},
              "write_files": [{"path": "/opt/feam-w0/full_vm_probe.sh", "permissions": "0700",
                               "encoding": "b64", "content": base64.b64encode(probe.read_bytes()).decode()}],
              "runcmd": [["sh", "-c", "bash /opt/feam-w0/full_vm_probe.sh --confirm-disposable > /var/log/feam-w0-probe.log 2>&1; echo $? > /var/log/feam-w0-probe.exit"]]}
    (seed / "user-data").write_text("#cloud-config\n" + json.dumps(config, indent=2) + "\n")
    (seed / "meta-data").write_text("instance-id: feam-w0-disposable-v1\nlocal-hostname: feam-w0-disposable\n")
    (ROOT / "known_hosts").write_text("[127.0.0.1]:22371 " + host_public + "\n")
    run([str(extracted / "usr/bin/genisoimage"), "-quiet", "-output", str(ROOT / "seed.iso"),
         "-volid", "cidata", "-joliet", "-rock", "user-data", "meta-data"], cwd=seed, env=tool_env())
    run([str(extracted / "usr/bin/qemu-img"), "create", "-f", "qcow2", "-F", "qcow2",
         "-b", str(ROOT / IMAGE), str(ROOT / "vm.qcow2"), "16G"], env=tool_env())
    (ROOT / "prepared.json").write_text(json.dumps({"image_url": IMAGE_BASE + IMAGE,
        "image_sha256": IMAGE_SHA, "probe_sha256": sha(probe), "memory_mib": 2048,
        "vcpus": 2, "disk_max_gib": 16, "acceleration": "tcg"}, indent=2) + "\n")
    print("Prepared private W0 VM; host packages and services unchanged.")


def running_pid():
    pidfile = ROOT / "qemu.pid"
    if not pidfile.exists():
        return None
    pid = int(pidfile.read_text().strip())
    cmdline = Path(f"/proc/{pid}/cmdline")
    if not cmdline.exists():
        return None
    if str(ROOT / "vm.qcow2").encode() not in cmdline.read_bytes():
        raise SystemExit("recorded PID belongs to another process; refusing control")
    return pid


def start():
    if running_pid():
        print("VM already running")
        return
    if not (ROOT / "prepared.json").is_file():
        raise SystemExit("VM preparation incomplete")
    with socket.socket() as check:
        check.bind(("127.0.0.1", 22371))
    extracted = ROOT / "packages-root"
    command = [str(extracted / "usr/bin/qemu-system-x86_64"), "-name", "feam-w0-disposable",
               "-machine", "q35,accel=tcg", "-cpu", "max", "-smp", "2", "-m", "2048",
               "-L", str(extracted / "usr/share/qemu"),
               "-bios", str(extracted / "usr/share/seabios/bios-256k.bin"),
               "-drive", f"file={ROOT / 'vm.qcow2'},if=virtio,format=qcow2",
               "-drive", f"file={ROOT / 'seed.iso'},media=cdrom,readonly=on",
               "-netdev", "user,id=net0,hostfwd=tcp:127.0.0.1:22371-:22",
               "-device", "virtio-net-pci,netdev=net0,romfile=", "-display", "none", "-vga", "none",
               "-serial", f"file:{ROOT / 'serial.log'}", "-monitor", "none",
               "-qmp", f"unix:{ROOT / 'qmp.sock'},server=on,wait=off",
               "-pidfile", str(ROOT / "qemu.pid"), "-daemonize"]
    def limits():
        os.nice(10)
        resource.setrlimit(resource.RLIMIT_FSIZE, (20 * 1024**3, 20 * 1024**3))
    run(command, env=tool_env(), preexec_fn=limits)
    print("VM started: 2 vCPUs, 2 GiB RAM, 16 GiB disk maximum, localhost-only SSH.")


def stop():
    if not running_pid():
        print("VM already stopped")
        return
    with socket.socket(socket.AF_UNIX) as sock:
        sock.settimeout(10)
        sock.connect(str(ROOT / "qmp.sock"))
        stream = sock.makefile("rwb")
        json.loads(stream.readline())
        for command in ("qmp_capabilities", "system_powerdown"):
            stream.write((json.dumps({"execute": command}) + "\n").encode())
            stream.flush()
            while True:
                response = json.loads(stream.readline())
                if "error" in response:
                    raise RuntimeError("QMP rejected shutdown")
                if "return" in response:
                    break
    print("Guest poweroff requested; check status before deleting any owned files.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["prepare", "start", "status", "stop"])
    parser.add_argument("--probe", type=Path)
    args = parser.parse_args()
    scope()
    if args.action == "prepare":
        if args.probe is None:
            parser.error("prepare requires --probe")
        prepare(args.probe)
    elif args.action == "start":
        start()
    elif args.action == "stop":
        stop()
    else:
        print("running" if running_pid() else "stopped")


if __name__ == "__main__":
    main()
