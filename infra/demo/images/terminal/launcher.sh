#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' 'FEAM W1 private operator terminal. Manual mode; hosted broker arrives in W5.'
feam tui --project '/workspace/demo/client with spaces' --agent off
printf '%s\n' 'Manual FEAM exited. Private sandbox shell; type exit to close.'
exec /bin/bash --noprofile --norc
