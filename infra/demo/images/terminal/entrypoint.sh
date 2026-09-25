#!/usr/bin/env bash
set -euo pipefail
umask 0077
[[ $# == 0 && $(id -u) == 1000 ]]
[[ -d /workspace && -d /run/feam-terminal ]]
# The host verifies the actual UUID and mapping before invoking this fixed image.
# A partial initialization is retained for operator reconciliation, never reset.
if [[ ! -e /workspace/.feam-w1-initialized ]]; then
  [[ ! -e /workspace/demo ]]
  "$FEAM_WORKSPACE/scripts/tui_agent_stage1_demo.sh" /workspace/demo
  printf '%s\n' feam-w1-demo-v1 > /workspace/.feam-w1-initialized
fi
[[ ! -L /workspace/.feam-w1-initialized && $(cat /workspace/.feam-w1-initialized) == feam-w1-demo-v1 ]]
mkdir -p /workspace/cache /workspace/state /workspace/config
case "${FEAM_DEMO_MODE:-manual}" in
  manual)
    # Explicit W1 operator mode remains available with no hosted configuration.
    exec /usr/local/bin/ttyd --interface /run/feam-terminal/ttyd.sock \
      --writable --check-origin --max-clients 2 /opt/feam/launcher.sh
    ;;
  hosted)
    python3 /opt/feam/hosted_check.py
    python3 /opt/feam/attach_releases.py
    /usr/local/bin/model-adapter -listen 127.0.0.1:8443 \
      -socket /run/feam-model/broker.sock -capability-file /run/feam-config/capability \
      -cert /run/feam-config/cert.pem -key /run/feam-config/key.pem &
    adapter_pid=$!
    ;;
  *) printf '%s\n' 'Invalid FEAM demo mode; startup refused.' >&2; exit 1 ;;
esac
# URL-supplied arguments are disabled: never add ttyd --url-arg.
/usr/local/bin/ttyd --interface /run/feam-terminal/ttyd.sock \
  --writable --check-origin --max-clients 2 /opt/feam/launcher.sh &
terminal_pid=$!
cleanup() { kill "$adapter_pid" "$terminal_pid" 2>/dev/null || true; wait "$adapter_pid" "$terminal_pid" 2>/dev/null || true; }
trap cleanup EXIT
trap 'exit 143' TERM
trap 'exit 130' INT
# A dead adapter must not leave a silently unassisted participant terminal.
set +e
wait -n "$adapter_pid" "$terminal_pid"
status=$?
set -e
if (( status == 0 )); then status=1; fi
printf '%s\n' 'FEAM participant service stopped; operator attention required.' >&2
exit "$status"
