#!/bin/bash
# Prepare self-contained macOS HMMER/BLAST+ binaries for ReguTrace pack mode.
#
# This downloads and builds tools on the Mac that will package or verify the app.
# It never runs during an end-user install; the resulting files are copied into
# rsWGCNA.app/Contents/Resources/tools by bundle-regutrace-assets-macos.py.
set -euo pipefail
[[ "$(uname -s)" == Darwin ]] || { echo 'Run this script on macOS.' >&2; exit 1; }

output="${1:-}"
if [[ -z "$output" ]]; then
  echo "usage: $0 <output-directory>" >&2
  exit 2
fi
output="$(cd "$(dirname "$output")" && pwd -P)/$(basename "$output")"
mkdir -p "$output"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/regutrace-mac-tools.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT

arch="$(uname -m)"
case "$arch" in
  arm64) blast_arch='aarch64' ;;
  x86_64) blast_arch='x64' ;;
  *) echo "unsupported macOS architecture: $arch" >&2; exit 1 ;;
esac

echo "Building HMMER 3.4 for $arch..."
hmmer_urls=(
  'https://eddylab.org/software/hmmer/hmmer-3.4.tar.gz'
  'https://distfiles.macports.org/hmmer/hmmer-3.4.tar.gz'
)
for url in "${hmmer_urls[@]}"; do
  if curl -fL --retry 3 --connect-timeout 20 -o "$tmp/hmmer.tar.gz" "$url"; then
    break
  fi
done
[[ -s "$tmp/hmmer.tar.gz" ]] || { echo 'could not download HMMER source' >&2; exit 1; }
tar -xzf "$tmp/hmmer.tar.gz" -C "$tmp"
(
  cd "$tmp/hmmer-3.4"
  ./configure --prefix="$tmp/hmmer-install"
  make -j"$(sysctl -n hw.ncpu)"
  make install
)
cp "$tmp/hmmer-install/bin/hmmsearch" "$output/hmmsearch"
cp "$tmp/hmmer-3.4/LICENSE" "$output/HMMER-LICENSE.txt"
chmod 755 "$output/hmmsearch"

echo "Downloading BLAST+ 2.17.0+ for $arch..."
blast_url="https://ftp.ncbi.nlm.nih.gov/blast/executables/blast+/2.17.0/ncbi-blast-2.17.0+-${blast_arch}-macosx.tar.gz"
curl -fL --retry 3 --connect-timeout 20 -o "$tmp/blast.tar.gz" "$blast_url"
tar -xzf "$tmp/blast.tar.gz" -C "$tmp"
blast_root="$tmp/ncbi-blast-2.17.0+"
[[ -x "$blast_root/bin/blastp" ]] || { echo 'blastp missing from BLAST archive' >&2; exit 1; }
cp "$blast_root/bin/blastp" "$output/blastp"
chmod 755 "$output/blastp"
blast_license="$(find "$blast_root" -maxdepth 2 -type f \( -iname 'LICENSE' -o -iname 'LICENSE.txt' -o -iname 'LICENSE.md' \) | head -1)"
[[ -n "$blast_license" ]] || { echo 'BLAST license file missing from archive' >&2; exit 1; }
cp "$blast_license" "$output/BLAST-LICENSE.txt"

# NCBI's macOS archive may ship dynamic libraries. Preserve them next to the
# executable and make @loader_path/lib resolvable. The official build normally
# links only macOS system libraries, so this is a no-op in that case.
if [[ -d "$blast_root/lib" ]]; then
  rm -rf "$output/lib"
  cp -R "$blast_root/lib" "$output/lib"
fi

for exe in "$output/hmmsearch" "$output/blastp"; do
  install_name_tool -add_rpath '@loader_path/lib' "$exe" 2>/dev/null || true
  while IFS= read -r dependency; do
    case "$dependency" in
      /usr/lib/*|/System/*|@*|'') continue ;;
    esac
    if [[ -f "$dependency" ]]; then
      mkdir -p "$output/lib"
      cp "$dependency" "$output/lib/$(basename "$dependency")"
      install_name_tool -change "$dependency" "@loader_path/lib/$(basename "$dependency")" "$exe"
    fi
  done < <(otool -L "$exe" | awk 'NR > 1 {print $1}')
done

# BLAST+ may link nghttp2. Ship the MIT notice whenever the binary or its
# copied dylibs reference the library.
if otool -L "$output/blastp" | grep -qi 'nghttp2'; then
  cat > "$output/NGHTTP2-LICENSE.txt" <<'EOF'
The MIT License

Copyright (c) 2012, 2014, 2015, 2016 Tatsuhiro Tsujikawa
Copyright (c) 2012, 2014, 2015, 2016 nghttp2 contributors

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
EOF
fi

"$output/hmmsearch" -h > "$tmp/hmmsearch-help.txt" 2>&1 || true
grep -Eq 'HMMER[[:space:]]+[0-9]' "$tmp/hmmsearch-help.txt" || {
  echo 'bundled hmmsearch failed its version check' >&2
  cat "$tmp/hmmsearch-help.txt" >&2
  exit 1
}
"$output/blastp" -version > "$tmp/blastp-version.txt" 2>&1 || {
  echo 'bundled blastp failed its version check' >&2
  otool -L "$output/blastp" >&2 || true
  cat "$tmp/blastp-version.txt" >&2
  exit 1
}
grep -Eq 'blastp:[[:space:]]*[0-9]' "$tmp/blastp-version.txt" || {
  echo 'unexpected blastp version output' >&2
  cat "$tmp/blastp-version.txt" >&2
  exit 1
}

echo "Prepared macOS ReguTrace tools in $output"
"$output/hmmsearch" -h 2>&1 | grep -m1 'HMMER'
"$output/blastp" -version 2>&1 | grep -m1 'blastp:'
