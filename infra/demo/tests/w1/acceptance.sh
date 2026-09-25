#!/usr/bin/env bash
# Bounded proof in the dedicated W1 terminal, never a general container runner.
set -euo pipefail
[[ $EUID == 0 && $# == 1 ]]
phase=$1
probe_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
docker_w1() {
  runuser -u feam-runner -- env XDG_RUNTIME_DIR=/run/user/2101 \
    DOCKER_HOST=unix:///run/user/2101/feam-docker.sock docker "$@"
}
name=$(python3 -c 'import json; s=json.load(open("/home/feam-service-data/w1-terminal-state.json")); print("feam-w1-g"+str(s["generation"]))')
[[ $(docker_w1 inspect --format '{{index .Config.Labels "feam.scope"}}' "$name") == w1-private ]]
case "$phase" in
  readers)
    docker_w1 exec "$name" python /opt/feam/image_smoke.py
    ;;
  enforcement)
    docker_w1 exec -i "$name" python - < "$probe_root/container_probe.py"
    ;;
  walkthrough)
    for mode in manual fake; do
      # Reports remain in this workspace; no prior report is overwritten.
      docker_w1 exec -i "$name" python - --mode "$mode" --output "/workspace/w1-$mode.json" < "$probe_root/image_walkthrough.py"
    done
    ;;
  sockets)
    for account in feam-gateway feam-broker feam-collector; do
      expectation=deny
      [[ $account != feam-gateway ]] || expectation=allow
      runuser -u "$account" -- python3 "$probe_root/socket_probe.py" /run/feam-terminal-w1/slot-01/ttyd.sock --expect "$expectation"
    done
    runuser -u feam-controller -- python3 "$probe_root/socket_probe.py" /run/user/2101/feam-docker.sock --expect allow
    runuser -u feam-gateway -- python3 "$probe_root/socket_probe.py" /run/user/2101/feam-docker.sock --expect deny
    # Even if accidentally presented the directory, a different sandbox UID
    # cannot traverse it. Normal runtime templates never mount another socket.
    image=$(python3 -c 'import json;print(json.load(open("/etc/feam/w1-terminal.json"))["image_id"])')
    docker_w1 run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges \
      --user 1001:1001 --memory 64m --memory-swap 64m --cpus .5 --pids-limit 16 \
      --mount type=bind,src=/run/feam-terminal-w1/slot-01,dst=/run/feam-terminal,readonly \
      --entrypoint python "$image" -c $'import socket\ns=socket.socket(socket.AF_UNIX)\ntry:\n s.connect("/run/feam-terminal/ttyd.sock")\nexcept PermissionError:\n print("PASS: other sandbox UID denied")\nelse:\n raise SystemExit("cross-identity socket access")'
    ;;
  persistence)
    docker_w1 exec "$name" sh -c 'test "$(cat /workspace/persistence-proof)" = feam-w1-private-data'
    /usr/local/sbin/feam-w1-terminal stop
    /usr/local/sbin/feam-w1-terminal start
    docker_w1 exec "$name" sh -c 'test "$(cat /workspace/persistence-proof)" = feam-w1-private-data'
    echo 'PASS: private-volume bytes survive stop/start'
    ;;
  reset)
    [[ $name == feam-w1-g1 ]]
    old_uuid=$(findmnt -n -o UUID --mountpoint /home/feam-service-data/slot-01)
    /usr/local/sbin/feam-w1-terminal reset --confirm-generation 1
    [[ $(cat /home/feam-service-data/slot-01/persistence-proof) == feam-w1-private-data ]]
    [[ $(findmnt -n -o UUID --mountpoint /home/feam-service-data/slot-01) == "$old_uuid" ]]
    for attempt in {1..60}; do
      if docker_w1 exec feam-w1-g2 test -f /workspace/.feam-w1-initialized; then break; fi
      sleep 1
    done
    docker_w1 exec feam-w1-g2 test -f /workspace/.feam-w1-initialized
    docker_w1 exec feam-w1-g2 test ! -e /workspace/persistence-proof
    if /usr/local/sbin/feam-w1-terminal reset --confirm-generation 2; then
      echo 'unexpected second reset with occupied spare' >&2; exit 1
    fi
    [[ $(docker_w1 inspect --format '{{.State.Running}}' feam-w1-g2) == true ]]
    echo 'PASS: reset uses only the empty fixed spare and retains old bytes/UUID'
    ;;
  *) echo 'choose readers, enforcement, walkthrough, sockets, persistence, or reset' >&2; exit 2 ;;
esac
