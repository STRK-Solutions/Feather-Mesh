#!/usr/bin/env bash
set -euo pipefail
case "${FEAM_DEMO_MODE:-manual}" in
  manual)
    printf '%s\n' 'FEAM W1 private operator terminal. Manual mode.'
    feam tui --project '/workspace/demo/client with spaces' --agent off
    ;;
  hosted)
    python3 /opt/feam/hosted_check.py
    export XDG_CONFIG_HOME=/run/feam-config
    # Only the adapter reads the rotating capability; this is a non-secret placeholder.
    export FEAM_BROKER_CAPABILITY=workspace-adapter
    python3 /opt/feam/hosted_check.py --status
    feam tui --project '/workspace/demo/client with spaces' --agent hosted --agent-profile phase1-demo
    ;;
  *) printf '%s\n' 'Invalid FEAM demo mode; launch refused.' >&2; exit 1 ;;
esac
printf '%s\n' 'FEAM exited. Private sandbox shell; type exit to close.'
exec /bin/bash --noprofile --norc
