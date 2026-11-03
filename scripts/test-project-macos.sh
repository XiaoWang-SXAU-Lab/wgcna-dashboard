#!/bin/bash
set -euo pipefail
[[ "$(uname -s)" == Darwin ]] || { echo 'Use test-project-windows.ps1 on Windows.' >&2; exit 1; }
source_root="$(cd "$(dirname "$0")/.." && pwd -P)"
local_root="$HOME/Library/Caches/rsWGCNA-build/macos"
mkdir -p "$local_root"
run_root="$(mktemp -d "$local_root/project-tests-XXXXXXXX")"
git -C "$source_root" ls-files --cached --others --exclude-standard -z -- backend > "$run_root/source-files"
while IFS= read -r -d '' file; do
  [[ -f "$source_root/$file" ]] || continue
  mkdir -p "$run_root/$(dirname "$file")"
  cp "$source_root/$file" "$run_root/$file"
done < "$run_root/source-files"
module_cache="$(go env GOMODCACHE)"
case "$module_cache/" in "$(dirname "$source_root")/"*) echo 'Go module cache must be outside synchronized source.' >&2; exit 1;; esac
export GOCACHE="$local_root/go-cache" GOMODCACHE="$module_cache" GOTMPDIR="$local_root/go-tmp"
mkdir -p "$GOTMPDIR"
# Optional: explicitly point to this machine's native Rust worker.
export WGCNA_PROJECT_TEST_WORKER="${1:-}"
cd "$run_root/backend"
go test -count=1 -v ./internal/project
go test -count=1 -v -run '^TestProjectHTTP' ./internal/api
printf 'Project test snapshot: %s\n' "$run_root"
