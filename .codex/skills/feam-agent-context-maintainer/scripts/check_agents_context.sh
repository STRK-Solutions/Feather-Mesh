#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../../../.." && pwd)
file="$repo_root/AGENTS.md"

if [[ ! -f "$file" ]]; then
  echo "missing AGENTS.md" >&2
  exit 1
fi

required_patterns=(
  "feam"
  "feather-mesh/"
  "mesh_core"
  "mesh_cli"
  "cargo test"
)

for pattern in "${required_patterns[@]}"; do
  if ! grep -Fq "$pattern" "$file"; then
    echo "AGENTS.md missing required context: $pattern" >&2
    exit 1
  fi
done

echo "AGENTS.md context smoke check passed"
