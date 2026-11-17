#!/bin/bash
# Assemble a self-contained rsWGCNA.app and DMG on macOS.
#
# The app directory must contain the three sidecars produced by build-macos.sh:
#   rsWGCNA rs-wgcna-worker regutrace
#
# Required environment:
#   WGCNA_PACK_SOURCE   directory containing the four reference pack directories
#   WGCNA_TOOL_SOURCE   directory produced by prepare-macos-regutrace-tools.sh
# Optional:
#   WGCNA_R_FRAMEWORK   path to R.framework to embed
#   WGCNA_PACK_NAMES    space-separated pack directory names
#   WGCNA_APP_VERSION   defaults to 0.6.0
#   WGCNA_OUTPUT_DIR    defaults to the app directory's parent
set -euo pipefail
[[ "$(uname -s)" == Darwin ]] || { echo 'Run this script on macOS.' >&2; exit 1; }

app_dir="${1:-}"
[[ -n "$app_dir" && -d "$app_dir" ]] || {
  echo "usage: $0 <directory with rsWGCNA, rs-wgcna-worker, regutrace>" >&2
  exit 2
}
app_dir="$(cd "$app_dir" && pwd -P)"
for binary in rsWGCNA rs-wgcna-worker regutrace; do
  [[ -x "$app_dir/$binary" ]] || { echo "missing executable: $app_dir/$binary" >&2; exit 1; }
done
: "${WGCNA_PACK_SOURCE:?set WGCNA_PACK_SOURCE to the reference-pack source directory}"
: "${WGCNA_TOOL_SOURCE:?set WGCNA_TOOL_SOURCE to the prepared macOS tools directory}"
pack_source="$(cd "$WGCNA_PACK_SOURCE" && pwd -P)"
tool_source="$(cd "$WGCNA_TOOL_SOURCE" && pwd -P)"
version="${WGCNA_APP_VERSION:-0.6.0}"
output_dir="${WGCNA_OUTPUT_DIR:-$(dirname "$app_dir")}"
mkdir -p "$output_dir"
output_dir="$(cd "$output_dir" && pwd -P)"

dashboard="$(cd "$(dirname "$0")/.." && pwd -P)"
stamp="$(date +%Y%m%d-%H%M%S)"
bundle="$output_dir/rsWGCNA-$stamp.app"
resources="$bundle/Contents/Resources"
macos="$bundle/Contents/MacOS"
mkdir -p "$macos" "$resources"

cp "$app_dir/rsWGCNA" "$app_dir/rs-wgcna-worker" "$app_dir/regutrace" "$macos/"
chmod 755 "$macos/rsWGCNA" "$macos/rs-wgcna-worker" "$macos/regutrace"
cp "$dashboard/packaging/Info.plist" "$bundle/Contents/Info.plist"
cp "$dashboard/packaging/icon.icns" "$resources/icon.icns"
plutil -replace CFBundleVersion -string "$version" "$bundle/Contents/Info.plist"
plutil -replace CFBundleShortVersionString -string "$version" "$bundle/Contents/Info.plist"

read -r -a pack_names <<< "${WGCNA_PACK_NAMES:-plant-jaspar-2024-rbh nematode-beta-2026-11-15-r2 drosophila-beta-2026-11-15 yeast-2026-11-15}"
python3 "$dashboard/scripts/bundle-regutrace-assets-macos.py" \
  --output "$resources" \
  --pack-source "$pack_source" \
  --tool-source "$tool_source" \
  --pack-names "${pack_names[@]}"

if [[ -n "${WGCNA_R_FRAMEWORK:-}" ]]; then
  [[ -d "$WGCNA_R_FRAMEWORK" ]] || { echo "R.framework not found: $WGCNA_R_FRAMEWORK" >&2; exit 1; }
  mkdir -p "$bundle/Contents/Frameworks"
  cp -R "$WGCNA_R_FRAMEWORK" "$bundle/Contents/Frameworks/"
  framework="$bundle/Contents/Frameworks/$(basename "$WGCNA_R_FRAMEWORK")"
  # Framework-root symlinks cannot be sealed by codesign; Rscript does not need
  # them at runtime. Keep the versioned Resources/bin layout intact.
  find "$framework" -maxdepth 1 -type l -delete
  codesign --force --deep -s - "$framework"
fi

# Sign sidecars and bundled tools before sealing the bundle. Ad-hoc signing is
# sufficient for local distribution; replace '-' with a Developer ID for a
# notarized public release.
for executable in \
  "$macos/rs-wgcna-worker" \
  "$macos/regutrace" \
  "$resources/tools/hmmsearch" \
  "$resources/tools/blastp"; do
  codesign --force -s - "$executable"
done
codesign --force --deep -s - "$bundle"

python3 - "$bundle" "$app_dir" "$version" <<'PY'
import datetime
import hashlib
import json
import os
import sys

bundle, app_dir, version = sys.argv[1:4]

def sha(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()

macos = os.path.join(bundle, "Contents", "MacOS")
resources = os.path.join(bundle, "Contents", "Resources")
info = {
    "platform": "macos-" + os.uname().machine,
    "built_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "app_version": version,
    "source": "build-macos.sh output",
    "exe_sha256": sha(os.path.join(macos, "rsWGCNA")),
    "worker_sha256": sha(os.path.join(macos, "rs-wgcna-worker")),
    "regutrace_sha256": sha(os.path.join(macos, "regutrace")),
    "tools_manifest_sha256": sha(os.path.join(resources, "tools", "manifest.json")),
    "third_party_notices_sha256": sha(os.path.join(resources, "THIRD_PARTY_NOTICES.md")),
}
with open(os.path.join(resources, "build-info.json"), "w", encoding="utf-8") as handle:
    json.dump(info, handle, indent=2)
    handle.write("\n")
PY

# build-info.json was added after the first seal; re-sign the bundle so the
# final app container is valid.
codesign --force --deep -s - "$bundle"

mkdir -p "$output_dir/stage-$stamp"
cp -R "$bundle" "$output_dir/stage-$stamp/"
ln -s /Applications "$output_dir/stage-$stamp/Applications"
dmg="$output_dir/rsWGCNA-$version-$stamp.dmg"
hdiutil create -volname rsWGCNA -srcfolder "$output_dir/stage-$stamp" -ov -format UDZO "$dmg" >/dev/null
rm -rf "$output_dir/stage-$stamp"

echo "macOS app: $bundle"
echo "macOS DMG: $dmg"
echo "DMG bytes: $(stat -f%z "$dmg")"
echo "DMG SHA-256: $(shasum -a 256 "$dmg" | awk '{print $1}')"
