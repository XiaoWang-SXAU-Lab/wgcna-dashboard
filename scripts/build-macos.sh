#!/bin/bash
set -euo pipefail
[[ "$(uname -s)" == Darwin ]] || { echo 'Use build-windows.ps1 on Windows.' >&2; exit 1; }
dashboard="$(cd "$(dirname "$0")/.." && pwd -P)"
worker="$(cd "${WGCNA_WORKER_SOURCE:-$dashboard/../rs_WGCNA}" && pwd -P)"
root="${WGCNA_BUILD_ROOT:-$HOME/Library/Caches/rsWGCNA-build/macos}"
mkdir -p "$root"
root="$(cd "$root" && pwd -P)"
sync_root="$(dirname "$dashboard")"
case "$root/" in "$sync_root/"*) echo 'Build root must be outside the synchronized source directory.' >&2; exit 1;; esac
run="$(mktemp -d "$root/run-XXXXXXXX")"

copy_source() {
  local repo="$1" dest="$2" file
  shift 2
  # Only source files; never copy .git, node_modules, target, dist or embed files.
  git -C "$repo" ls-files --cached --others --exclude-standard -z -- "$@" > "$run/source-files"
  while IFS= read -r -d '' file; do
    [[ -f "$repo/$file" ]] || continue
    mkdir -p "$dest/$(dirname "$file")"
    cp "$repo/$file" "$dest/$file"
  done < "$run/source-files"
}
copy_source "$dashboard" "$run/dashboard" frontend backend
copy_source "$worker" "$run/worker" Cargo.toml Cargo.lock src build.rs
export CARGO_TARGET_DIR="$root/cargo-target"
# Keep Windows-specific compiler settings out of this build.
unset RUSTFLAGS CARGO_ENCODED_RUSTFLAGS CC CXX GOOS GOARCH
export CGO_ENABLED=1 CI=true
(
  cd "$run/worker"
  cargo +stable build --locked --release --bin rs-wgcna-worker
)
(
  cd "$run/dashboard/frontend"
  pnpm --config.manage-package-manager-versions=false install --frozen-lockfile
  node node_modules/vue-tsc/bin/vue-tsc.js -b
  node node_modules/vite/bin/vite.js build
)
cp "$CARGO_TARGET_DIR/release/rs-wgcna-worker" "$run/dashboard/backend/internal/worker/worker-bin"
cp -R "$run/dashboard/frontend/dist" "$run/dashboard/backend/internal/web/dist"
mkdir -p "$run/app"
(
  cd "$run/dashboard/backend"
  go build -tags release -o "$run/app/rsWGCNA" ./cmd/server
)
cp "$CARGO_TARGET_DIR/release/rs-wgcna-worker" "$run/app/rs-wgcna-worker"
printf '%s\n' "$run/app" > "$root/latest-app.txt"
printf 'macOS build: %s\n' "$run/app/rsWGCNA"
