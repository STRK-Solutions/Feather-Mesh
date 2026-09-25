#!/usr/bin/env bash
# Dependencies for the retained disposable guest only. Never a physical-host bootstrap.
set -euo pipefail
[[ $EUID == 0 && ${1:-} == --confirm-disposable ]]
[[ $(systemd-detect-virt --vm) == qemu ]]
[[ $(hostname) == feam-w0-disposable ]]
[[ -f /opt/feam-w0/full_vm_probe.sh ]]
apt-get update
apt-get install -y --no-install-recommends ca-certificates curl gnupg uidmap dbus-user-session acl slirp4netns apparmor
install -d -m 0755 /etc/apt/keyrings
curl --fail --location --proto '=https' https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/feam-test-docker.asc
fingerprint=$(gpg --show-keys --with-colons /etc/apt/keyrings/feam-test-docker.asc | awk -F: '$1=="fpr" {print $10; exit}')
[[ $fingerprint == 9DC858229FC7DD38854AE2D88D81803C0EBFCD88 ]]
printf '%s\n' 'deb [arch=amd64 signed-by=/etc/apt/keyrings/feam-test-docker.asc] https://download.docker.com/linux/ubuntu noble stable' > /etc/apt/sources.list.d/feam-test-docker.list
apt-get update
apt-get install -y --no-install-recommends docker-ce docker-ce-cli docker-ce-rootless-extras docker-buildx-plugin containerd.io
# Only guest dependency setup changes its package set; host.yml never replaces
# an existing physical-host Docker installation or its daemon configuration.
dpkg-query -W docker-ce docker-ce-cli docker-ce-rootless-extras containerd.io uidmap apparmor
