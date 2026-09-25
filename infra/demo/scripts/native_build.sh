#!/usr/bin/env bash
# Bounded native Ubuntu build in the already owned W0 VM preparation scope.
# Does not install host packages, edit profiles or touch the existing Cargo home.
set -euo pipefail
[[ "$(uname -m)" == x86_64 && "$(uname -s)" == Linux ]]
build_scope="$HOME/feam-w0-vm/native-build"
python3 - <<'PY'
import json, os
from pathlib import Path
root = Path.home() / 'feam-w0-vm'
assert not root.is_symlink()
assert root.stat().st_uid == os.getuid()
assert json.loads((root / 'ownership.json').read_text()) == {'scope':'feam-w0-disposable-v1','uid':os.getuid()}
PY
umask 077
mkdir -p "$build_scope"
cd "$build_scope"
export CARGO_HOME="$build_scope/cargo-home"
export CARGO_TARGET_DIR="$build_scope/target"
export CARGO_BUILD_JOBS=2
if [[ ! -x "$build_scope/rust/bin/cargo" ]]; then
  archive=rust-1.94.0-x86_64-unknown-linux-gnu.tar.xz
  curl --fail --location --proto '=https' --tlsv1.2 --max-time 600 \
    "https://static.rust-lang.org/dist/$archive" --output "$archive"
  curl --fail --location --proto '=https' --tlsv1.2 --max-time 60 \
    "https://static.rust-lang.org/dist/$archive.sha256" --output "$archive.sha256"
  sha256sum --check "$archive.sha256"
  tar -xf "$archive"
  bash rust-1.94.0-x86_64-unknown-linux-gnu/install.sh \
    --prefix="$build_scope/rust" --disable-ldconfig \
    --components=rustc,cargo,rust-std-x86_64-unknown-linux-gnu
fi
export PATH="$build_scope/rust/bin:$PATH"
rustc -vV
cargo --version
[[ -f source.tar && -f source.tar.sha256 ]]
sha256sum --check source.tar.sha256
source_digest=$(sha256sum source.tar | cut -d ' ' -f1)
source_directory="source-$source_digest"
if [[ ! -d "$source_directory" ]]; then
  mkdir "$source_directory"
  tar -xf source.tar -C "$source_directory"
  cp source.tar.sha256 "$source_directory/extracted-source.sha256"
else
  cmp source.tar.sha256 "$source_directory/extracted-source.sha256"
fi
cd "$source_directory/feather-mesh"
cargo build --locked --release -p mesh_cli --features agent-hosted --jobs 2
sha256sum "$CARGO_TARGET_DIR/release/mesh_cli" > "$build_scope/mesh_cli.sha256"
file "$CARGO_TARGET_DIR/release/mesh_cli"
"$CARGO_TARGET_DIR/release/mesh_cli" --help
echo 'PASS: native Ubuntu x86-64 hosted-capable FEAM release build.'
