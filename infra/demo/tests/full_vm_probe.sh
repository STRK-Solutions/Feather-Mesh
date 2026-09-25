#!/usr/bin/env bash
# W0 only: a 64 MiB temporary filesystem and transient systemd service.
set -euo pipefail
if [[ "${1:-}" != "--confirm-disposable" || "$EUID" != 0 ]]; then
  echo 'Run as root only in an explicitly disposable full-system Ubuntu VM.' >&2
  exit 2
fi
[[ "$(uname -m)" == x86_64 ]]
source /etc/os-release
[[ "$ID" == ubuntu && "$VERSION_ID" == 24.04 ]]
systemd-detect-virt --vm > /dev/null
[[ "$(cat /proc/1/comm)" == systemd ]]
[[ "$(stat -fc %T /sys/fs/cgroup)" == cgroup2fs ]]
for controller in cpu memory pids; do
  [[ " $(cat /sys/fs/cgroup/cgroup.controllers) " == *" $controller "* ]]
done
probe_dir=$(mktemp -d /var/tmp/feam-w0-vm.XXXXXXXX)
cleanup() {
  if mountpoint -q "$probe_dir/mounted"; then
    umount "$probe_dir/mounted" || return 1
  fi
  # Only the path returned by mktemp above, never caller-provided storage.
  rm -rf -- "$probe_dir"
}
trap cleanup EXIT
mkdir "$probe_dir/mounted"
dd if=/dev/zero of="$probe_dir/owned.ext4" bs=1M count=64 status=none
mkfs.ext4 -q -F "$probe_dir/owned.ext4"
expected_uuid=$(blkid -s UUID -o value "$probe_dir/owned.ext4")
mount -o loop,nosuid,nodev,noexec "$probe_dir/owned.ext4" "$probe_dir/mounted"
[[ "$(findmnt -n -o UUID --target "$probe_dir/mounted")" == "$expected_uuid" ]]
printf 'feam-w0-owned\n' > "$probe_dir/mounted/probe"
sync
umount "$probe_dir/mounted"
mount -o loop,nosuid,nodev,noexec "$probe_dir/owned.ext4" "$probe_dir/mounted"
[[ "$(cat "$probe_dir/mounted/probe")" == feam-w0-owned ]]
systemd-run --quiet --wait --pipe --collect \
  --unit="feam-w0-${probe_dir##*.}" \
  --property=MemoryMax=67108864 --property=TasksMax=32 \
  /bin/sh -eu -c '
    group=$(cut -d: -f3 /proc/self/cgroup)
    test "$(cat /sys/fs/cgroup"$group"/memory.max)" = 67108864
    test "$(cat /sys/fs/cgroup"$group"/pids.max)" = 32
  '
echo 'PASS: Ubuntu x86-64 full VM, systemd/cgroup v2 scope, 64 MiB loop filesystem identity and remount persistence.'
echo 'This does not establish rootless Docker isolation, host reboot, or ten-user capacity.'
