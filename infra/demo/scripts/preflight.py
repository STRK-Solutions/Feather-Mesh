#!/usr/bin/env python3
"""Read-only Ubuntu probes. Raw output belongs in the private operator directory."""
import json
import os
import subprocess


PROBES = {
    "os": ["cat", "/etc/os-release"],
    "kernel": ["uname", "-srm"],
    "systemd": ["systemd", "--version"],
    "cpus": ["nproc"],
    "memory": ["free", "-b"],
    "mounts": ["findmnt", "--json", "--output", "TARGET,SOURCE,FSTYPE,UUID,OPTIONS", "--target", "/home"],
    "disks": ["lsblk", "--json", "--bytes", "--output", "NAME,TYPE,SIZE,FSTYPE,UUID,MOUNTPOINTS,ROTA"],
    "space": ["df", "-B1", "/", "/home"],
    "inodes": ["df", "-i", "/", "/home"],
    "cgroup_type": ["stat", "-fc", "%T", "/sys/fs/cgroup"],
    "controllers": ["cat", "/sys/fs/cgroup/cgroup.controllers"],
    "user_manager": ["systemctl", "show", f"user@{os.getuid()}.service", "--property=Delegate,ControlGroup,ActiveState"],
    "user_controllers": ["cat", f"/sys/fs/cgroup/user.slice/user-{os.getuid()}.slice/user@{os.getuid()}.service/cgroup.controllers"],
    "apparmor": ["cat", "/sys/module/apparmor/parameters/enabled"],
    "namespace_policy": ["sysctl", "kernel.apparmor_restrict_unprivileged_userns", "kernel.unprivileged_userns_clone", "user.max_user_namespaces"],
    "time_sync": ["timedatectl", "show", "--property=NTPSynchronized", "--property=Timezone"],
    "docker_services": ["systemctl", "show", "docker.service", "docker.socket", "containerd.service", "--property=Id,ActiveState,UnitFileState"],
    "packages": ["dpkg-query", "-W", "-f=${Package} ${Version}\n", "docker-ce", "docker-ce-cli", "docker-ce-rootless-extras", "docker.io", "containerd.io", "uidmap", "slirp4netns", "rootlesskit", "apparmor", "python3", "qemu-system-x86", "libvirt-daemon-system"],
    "package_origins": ["apt-cache", "policy", "docker-ce", "docker.io", "docker-ce-rootless-extras", "rootlesskit", "uidmap"],
    "subuids": ["cat", "/etc/subuid"],
    "subgids": ["cat", "/etc/subgid"],
    "service_collisions": ["getent", "passwd", "feam-gateway", "feam-controller", "feam-runner", "feam-broker", "feam-collector", "feam-pipeline", "feam-reconciler"],
    "service_path": ["stat", "--format=%F %u %g %a", "/home/feam-service-data"],
    "loop_support": ["ls", "-l", "/dev/loop-control"],
    "virtualization": ["systemd-detect-virt"],
    "kvm": ["ls", "-l", "/dev/kvm"],
    "build_tools": ["sh", "-c", "for tool in cc make rustc cargo go qemu-system-x86_64 virsh; do command -v \"$tool\" || true; done"],
    "docker_unprivileged": ["docker", "ps", "--format", "{{.ID}} {{.State}}"],
    "sudo_available": ["sudo", "-n", "true"],
    "docker_privileged": ["sudo", "-n", "docker", "ps", "--format", "{{.ID}} {{.State}}"],
}


def collect(probes):
    results = {}
    for name, argv in probes.items():
        try:
            p = subprocess.run(argv, capture_output=True, text=True, timeout=20)
            results[name] = {"argv": argv, "returncode": p.returncode,
                             "stdout": p.stdout[:32768], "stderr": p.stderr[:4096]}
        except (OSError, subprocess.TimeoutExpired) as exc:
            results[name] = {"argv": argv, "returncode": None, "error": str(exc)}
    return results


if __name__ == "__main__":
    probes = dict(PROBES)
    for host in ("download.docker.com", "registry-1.docker.io", "github.com",
                 "pypi.org", "go.dev", "openrouter.ai", "api.cloudflare.com"):
        probes["https_" + host] = ["curl", "--head", "--silent", "--show-error",
                                    "--connect-timeout", "5", "--max-time", "10",
                                    "--output", "/dev/null", "--write-out", "%{http_code}",
                                    "https://" + host + "/"]
    print(json.dumps({"schema_version": 1, "probes": collect(probes)}, indent=2))
