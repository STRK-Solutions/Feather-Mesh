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
# URL-supplied arguments are disabled: never add ttyd --url-arg.
exec /usr/local/bin/ttyd --interface /run/feam-terminal/ttyd.sock \
  --writable --check-origin --max-clients 2 /opt/feam/launcher.sh
