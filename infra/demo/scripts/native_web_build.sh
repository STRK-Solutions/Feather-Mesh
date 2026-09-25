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
source_digest=$(sha256sum source.tar | cut -d ' ' -f 1)
source_dir="$build_scope/source-$source_digest"
# Never overlay a prior tree: deleted source files must not survive a new build.
if [[ ! -d "$source_dir" ]]; then
  staging_dir=$(mktemp -d "$build_scope/.source-$source_digest.XXXXXX")
  trap 'rm -rf -- "$staging_dir"' EXIT
  tar -xf source.tar -C "$staging_dir"
  cp source.tar.sha256 "$staging_dir/extracted-source.sha256"
  mv "$staging_dir" "$source_dir"
  trap - EXIT
else
  [[ ! -L "$source_dir" ]]
  cmp source.tar.sha256 "$source_dir/extracted-source.sha256"
fi
export GOCACHE="$build_scope/go-cache" GOPATH="$build_scope/go-path"
export GOMAXPROCS=2 GOFLAGS=-p=2
cd "$source_dir/web_demo"
"$build_scope/go/bin/go" test -race ./...
"$build_scope/go/bin/go" vet ./...
for component in private-terminal gateway controller broker model-adapter project-budget reconciler pipeline collector capture-fixture; do
  "$build_scope/go/bin/go" build -trimpath -o "$build_scope/$component" "./cmd/$component"
  sha256sum "$build_scope/$component" > "$build_scope/$component.sha256"
done
