#!/usr/bin/env bash
set -euo pipefail

file="AGENTS.md"

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
  "python_mvp/"
  "not be treated as the primary implementation"
)

for pattern in "${required_patterns[@]}"; do
  if ! grep -Fq "$pattern" "$file"; then
    echo "AGENTS.md missing required context: $pattern" >&2
    exit 1
  fi
done

if grep -Eiq "python_mvp/ is (the )?(primary|source of truth)|primary implementation lives in python_mvp/|source of truth.+python_mvp/" "$file"; then
  echo "AGENTS.md appears to make python_mvp primary; review required" >&2
  exit 1
fi

echo "AGENTS.md context smoke check passed"
