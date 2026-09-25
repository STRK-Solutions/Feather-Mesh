#!/usr/bin/env bash
# Reviewed W1 physical-host entrypoint. Owner invokes with sudo only after VM proof.
set -euo pipefail
if [[ $EUID != 0 || $# != 3 || $1 != --reviewed-bundle ]]; then
  echo 'usage: sudo bash owner_bootstrap_w1.sh --reviewed-bundle /absolute/bundle SHA256' >&2
  exit 2
fi
bundle=$2
expected=$3
[[ $bundle == /* && $expected =~ ^[0-9a-f]{64}$ && ! -L $bundle ]]
[[ $(uname -s) == Linux && $(uname -m) == x86_64 ]]
umask 077
stage=$(mktemp -d /var/tmp/feam-w1-bootstrap.XXXXXXXX)
trap 'rm -rf -- "$stage"' EXIT
install -m 0600 "$bundle" "$stage/bundle.tar"
printf '%s  %s\n' "$expected" "$stage/bundle.tar" | sha256sum --check
# Reject absolute/traversing links or special files before root extraction.
python3 - "$stage/bundle.tar" "$stage" <<'PY'
from pathlib import PurePosixPath
import sys,tarfile
with tarfile.open(sys.argv[1]) as source:
    for item in source:
        path=PurePosixPath(item.name)
        if path.is_absolute() or '..' in path.parts or not (item.isfile() or item.isdir()):
            raise SystemExit('unsafe bundle member')
    source.extractall(sys.argv[2],filter='data')
PY
cd "$stage/source"
printf '[demo]\nlocalhost ansible_connection=local\n' > inventory.ini
sha256sum --check artifacts.sha256
systemctl show docker.service --property=MainPID --property=ActiveState --property=ActiveEnterTimestampMonotonic > "$stage/docker-before"
cat /proc/sys/kernel/apparmor_restrict_unprivileged_userns > "$stage/namespace-before"
sha256sum /etc/apparmor.d/rootlesskit > "$stage/apparmor-before"
# Only the observed missing subordinate-ID helper is installed. Existing Docker,
# AppArmor policy, unrelated services and host namespaces are never reconfigured.
if ! dpkg-query -W -f='${db:Status-Status}' uidmap 2>/dev/null | grep -qx installed; then
  NEEDRESTART_MODE=l DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends uidmap=1:4.13+dfsg1-4ubuntu3.2
fi
python3 infra/demo/scripts/w1_accounts.py
operator_tools=$(python3 -c 'import json;print(json.load(open("operator.json"))["ansible_bin"])')
[[ -x "$operator_tools/ansible-playbook" ]]
export PATH="$operator_tools:/usr/sbin:/usr/bin:/sbin:/bin"
export ANSIBLE_HOME="$stage/ansible" ANSIBLE_LOCAL_TEMP="$stage/ansible/tmp"
# These paths are root-owned copies of the reviewed artifacts, not home trees.
python3 - <<'PY'
import json,os
p='operator.json';data=json.load(open(p))
data['feam_proxy_source']=os.path.abspath('private-terminal')
open(p,'w').write(json.dumps(data))
PY
ansible-playbook -i inventory.ini -c local infra/demo/ansible/host.yml -e @operator.json \
  -e '{"ansible_python_interpreter":"/usr/bin/python3"}' --limit all
image_id=$(python3 -c 'import json;print(json.load(open("operator.json"))["feam_image_id"])')
if ! runuser -u feam-runner -- env XDG_RUNTIME_DIR=/run/user/2101 \
  DOCKER_HOST=unix:///run/user/2101/feam-docker.sock docker image inspect "$image_id" > /dev/null 2>&1; then
  runuser -u feam-runner -- env XDG_RUNTIME_DIR=/run/user/2101 \
    DOCKER_HOST=unix:///run/user/2101/feam-docker.sock docker image load < terminal-image.tar
fi
runuser -u feam-runner -- env XDG_RUNTIME_DIR=/run/user/2101 \
  DOCKER_HOST=unix:///run/user/2101/feam-docker.sock docker image inspect "$image_id" > /dev/null
ansible-playbook -i inventory.ini -c local infra/demo/ansible/deploy-w1.yml -e @operator.json \
  -e '{"ansible_python_interpreter":"/usr/bin/python3"}' --limit all
ansible-playbook -i inventory.ini -c local infra/demo/ansible/host.yml -e @operator.json \
  -e '{"ansible_python_interpreter":"/usr/bin/python3"}' --limit all
# The owner invocation also collects the required actual-host evidence. Each
# successful phase is durable; a failed phase is retained and stops the script.
# This is a fixed synthetic W1 slice, with no public users or billable inference.
proof=/home/feam-service-data/w1-acceptance
install -d -m 0700 -o root -g root "$proof"
for phase in readers enforcement walkthrough sockets persistence reset; do
  if [[ -f "$proof/$phase.ok" ]]; then
    [[ $(cat "$proof/$phase.ok") == "$image_id" ]]
    continue
  fi
  phase_log=$(mktemp "$proof/$phase.XXXXXXXX.log")
  printf 'Validating W1 %s; retaining output at %s\n' "$phase" "$phase_log"
  if bash /opt/feam-w1-tests/acceptance.sh "$phase" > "$phase_log" 2>&1; then
    printf '%s\n' "$image_id" > "$proof/$phase.ok"
  else
    cat "$phase_log" >&2
    echo "W1 verification stopped at $phase; retain and reconcile this attempt." >&2
    exit 1
  fi
  cat "$phase_log"
done
bash /opt/feam-w1-tests/acceptance.sh readers > "$proof/post-reset-readers.log"
# Repeat against the initialized/reset slice, proving preservation of its state.
ansible-playbook -i inventory.ini -c local infra/demo/ansible/host.yml -e @operator.json \
  -e '{"ansible_python_interpreter":"/usr/bin/python3"}' > "$proof/repeat-converge.log"
cat "$proof/repeat-converge.log"
grep -Eq 'changed=0[[:space:]]+unreachable=0[[:space:]]+failed=0' "$proof/repeat-converge.log"
systemctl show docker.service --property=MainPID --property=ActiveState --property=ActiveEnterTimestampMonotonic > "$proof/docker-after"
cat /proc/sys/kernel/apparmor_restrict_unprivileged_userns > "$proof/namespace-after"
sha256sum /etc/apparmor.d/rootlesskit > "$proof/apparmor-after"
cmp "$stage/docker-before" "$proof/docker-after"
cmp "$stage/namespace-before" "$proof/namespace-after"
cmp "$stage/apparmor-before" "$proof/apparmor-after"
loginctl show-user feam-runner --property=Linger --property=Sessions > "$proof/runner-lingering.log"
grep -qx 'Linger=yes' "$proof/runner-lingering.log"
grep -qx 'Sessions=' "$proof/runner-lingering.log"
printf '%s\n' 'PASS: existing Docker daemon and namespace policy unchanged' > "$proof/preservation.log"
printf '%s\n' "$image_id" > "$proof/complete"
# Return the generated operator-only browser secret through a private file,
# never stdout. Refuse a symlink target or unexpected owner directory.
python3 - <<'PYSECRET'
import json,os,pwd
from pathlib import Path
record=json.load(open('operator.json'));uid=int(record['feam_operator_uid'])
account=pwd.getpwuid(uid)
folder=Path(record['operator_output_dir'])
if folder.is_symlink() or folder.stat().st_uid!=uid or folder.stat().st_mode&0o077:
    raise SystemExit('operator output directory must be private and owned')
target=folder/'w1-browser-credential'
if target.is_symlink():raise SystemExit('refusing credential symlink')
fd=os.open(target,os.O_WRONLY|os.O_CREAT|os.O_TRUNC|os.O_NOFOLLOW,0o600)
try:
    os.fchmod(fd,0o600);os.fchown(fd,uid,account.pw_gid)
    os.write(fd,Path('/home/feam-service-data/gateway-home/w1-credential').read_bytes());os.fsync(fd)
finally:os.close(fd)
import shutil
proof=Path('/home/feam-service-data/w1-acceptance')
export=folder/'w1-acceptance'
export.mkdir(mode=0o700,exist_ok=True)
if export.is_symlink():raise SystemExit('refusing evidence symlink')
os.chown(export,uid,account.pw_gid);os.chmod(export,0o700)
for source in proof.iterdir():
    if source.is_file():
        target=export/source.name
        if target.is_symlink():raise SystemExit('refusing evidence file symlink')
        shutil.copyfile(source,target);os.chown(target,uid,account.pw_gid);os.chmod(target,0o600)
for mode in ('manual','fake'):
    source=Path('/home/feam-service-data/slot-01')/('w1-'+mode+'.json')
    target=export/source.name
    if target.is_symlink():raise SystemExit('refusing report symlink')
    shutil.copyfile(source,target);os.chown(target,uid,account.pw_gid);os.chmod(target,0o600)
PYSECRET
printf '%s\n' 'W1 private bootstrap completed. No public listener, provider key or reboot was configured.'
