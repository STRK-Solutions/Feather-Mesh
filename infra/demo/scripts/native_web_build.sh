#!/usr/bin/env bash
# Native host-service artifact in the previously authorized isolated build scope.
set -euo pipefail
[[ $(uname -s) == Linux && $(uname -m) == x86_64 ]]
build_scope="$HOME/feam-w0-vm/native-build/web-w1"
[[ -f "$HOME/feam-w0-vm/ownership.json" && ! -L "$HOME/feam-w0-vm" ]]
cd "$build_scope"
sha256sum --check source.tar.sha256
if [[ ! -x go/bin/go ]]; then
  curl --fail --location --proto '=https' --max-time 600 https://go.dev/dl/go1.27.1.linux-amd64.tar.gz -o go.tar.gz
  printf '%s\n' '63d339f0da5ab53635a56f2490a7984dfe12dfcff22ad749f63edaf590168445  go.tar.gz' | sha256sum --check
  tar -xf go.tar.gz
fi
mkdir -p source
# This path contains only this helper's code snapshot, never participant files.
tar -xf source.tar -C source
export GOCACHE="$build_scope/go-cache" GOPATH="$build_scope/go-path"
cd source/web_demo
"$build_scope/go/bin/go" test -race ./...
"$build_scope/go/bin/go" vet ./...
"$build_scope/go/bin/go" build -trimpath -o "$build_scope/private-terminal" ./cmd/private-terminal
sha256sum "$build_scope/private-terminal" > "$build_scope/private-terminal.sha256"
