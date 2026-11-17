#!/usr/bin/env python3
"""Bundle ReguTrace packs, HMMER and BLAST+ into a macOS app Resources dir.

The Windows build uses bundle-regutrace-assets.ps1. This script produces the
same tools/manifest.json schema and third-party notice layout for macOS so the
Go server can verify bundled tools without any WGCNA_* environment variables.

Expected tool source layout:
  hmmsearch                     executable (HMMER 3.3.2+)
  blastp                        executable (NCBI BLAST+ 2.17.0+)
  HMMER-LICENSE.txt             or hmmer/LICENSE
  BLAST-LICENSE.txt             or blast/LICENSE
  lib/                          optional dylibs copied verbatim

Usage:
  python3 scripts/bundle-regutrace-assets-macos.py \
    --output "rsWGCNA.app/Contents/Resources" \
    --pack-source /path/to/pack-source \
    --tool-source /path/to/macos-tools
"""

from __future__ import annotations

import argparse
import hashlib
import json
import platform
import re
import shutil
import subprocess
import sys
from pathlib import Path

DEFAULT_PACKS = [
    "plant-jaspar-2024-rbh",
    "nematode-beta-2026-11-15-r2",
    "drosophila-beta-2026-11-15",
    "yeast-2026-11-15",
]


def fail(message: str) -> None:
    raise SystemExit(message)


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def copy_required(source: Path, destination: Path, executable: bool = False) -> None:
    if not source.is_file():
        fail(f"required asset missing: {source}")
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, destination)
    if executable:
        destination.chmod(destination.stat().st_mode | 0o111)


def read_version(executable: Path, pattern: str, label: str) -> str:
    completed = subprocess.run(
        [str(executable), "-h" if label == "hmmsearch" else "-version"],
        capture_output=True,
        text=True,
        errors="replace",
        timeout=60,
    )
    text = (completed.stdout or "") + "\n" + (completed.stderr or "")
    match = re.search(pattern, text)
    if not match:
        fail(f"could not read {label} version from {executable}")
    return match.group(1)


def find_license(tool_source: Path, candidates: list[str]) -> Path:
    for relative in candidates:
        candidate = tool_source / relative
        if candidate.is_file():
            return candidate
    fail(
        "license file not found; expected one of: "
        + ", ".join(str(tool_source / name) for name in candidates)
    )


def collect_lib_files(tools_out: Path) -> dict[str, str]:
    lib_root = tools_out / "lib"
    if not lib_root.is_dir():
        return {}
    return {
        path.relative_to(tools_out).as_posix(): sha256(path)
        for path in sorted(lib_root.rglob("*"))
        if path.is_file()
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, help="app Contents/Resources directory")
    parser.add_argument("--pack-source", required=True, help="directory containing pack directories")
    parser.add_argument("--tool-source", required=True, help="directory with hmmsearch, blastp and licenses")
    parser.add_argument("--pack-names", nargs="+", default=DEFAULT_PACKS)
    parser.add_argument(
        "--allow-non-darwin",
        action="store_true",
        help="only for script tests; real packaging must run on macOS",
    )
    args = parser.parse_args()

    if platform.system() != "Darwin" and not args.allow_non_darwin:
        fail("macOS app packaging must run on macOS; use build-windows.ps1 on Windows")

    output = Path(args.output).resolve()
    pack_source = Path(args.pack_source).resolve()
    tool_source = Path(args.tool_source).resolve()
    output.mkdir(parents=True, exist_ok=True)

    packs_out = output / "packs"
    tools_out = output / "tools"
    packs_out.mkdir(parents=True, exist_ok=True)
    tools_out.mkdir(parents=True, exist_ok=True)

    pack_scopes: list[dict[str, str]] = []
    pack_readme = ["# Bundled ReguTrace reference packs", ""]
    for name in args.pack_names:
        source = pack_source / name
        manifest_path = source / "manifest.json"
        if not manifest_path.is_file():
            fail(f"reference pack missing manifest: {manifest_path}")
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        if not str(manifest.get("pack_version", "")).strip():
            fail(f"reference pack has empty pack_version: {manifest_path}")
        destination = packs_out / name
        if destination.exists():
            shutil.rmtree(destination)
        shutil.copytree(source, destination)
        species = str(manifest.get("species", ""))
        scope = str(manifest.get("motif_scope", ""))
        pack_readme.append(f"- `{name}` — {species}. {scope}".rstrip())
        pack_scopes.append(
            {
                "key": name,
                "version": str(manifest["pack_version"]),
                "species": species,
                "motif_scope": scope,
                "manifest_sha256": sha256(manifest_path),
            }
        )
    pack_readme += [
        "",
        "These packs are software-side reference data. ReguTrace verifies each manifest and file SHA-256 at pipeline start.",
    ]
    (packs_out / "README.md").write_text("\n".join(pack_readme) + "\n", encoding="utf-8")

    hmm_exe = tools_out / "hmmsearch"
    blast_exe = tools_out / "blastp"
    copy_required(tool_source / "hmmsearch", hmm_exe, executable=True)
    copy_required(tool_source / "blastp", blast_exe, executable=True)
    copy_required(
        find_license(tool_source, ["HMMER-LICENSE.txt", "hmmer/LICENSE", "LICENSE.hmmer"]),
        tools_out / "HMMER-LICENSE.txt",
    )
    copy_required(
        find_license(tool_source, ["BLAST-LICENSE.txt", "blast/LICENSE", "LICENSE.blast"]),
        tools_out / "BLAST-LICENSE.txt",
    )

    lib_source = tool_source / "lib"
    if lib_source.is_dir():
        lib_destination = tools_out / "lib"
        if lib_destination.exists():
            shutil.rmtree(lib_destination)
        shutil.copytree(lib_source, lib_destination)

    nghttp2_license = tool_source / "NGHTTP2-LICENSE.txt"
    if nghttp2_license.is_file():
        copy_required(nghttp2_license, tools_out / "NGHTTP2-LICENSE.txt")

    hmm_version = read_version(hmm_exe, r"HMMER\s+([0-9][0-9.]*)", "hmmsearch")
    blast_version = read_version(blast_exe, r"blastp:\s*([0-9][^\s]*)", "blastp")
    extra_files = collect_lib_files(tools_out)
    if (tools_out / "NGHTTP2-LICENSE.txt").is_file():
        extra_files["NGHTTP2-LICENSE.txt"] = sha256(tools_out / "NGHTTP2-LICENSE.txt")

    tool_manifest = {
        "schema": 1,
        "tools": {
            "hmmsearch": {
                "version": hmm_version,
                "file": "hmmsearch",
                "sha256": sha256(hmm_exe),
                "license": "HMMER BSD-3-Clause",
            },
            "blastp": {
                "version": blast_version,
                "file": "blastp",
                "sha256": sha256(blast_exe),
                "license": "NCBI BLAST+ public domain",
            },
        },
        "files": extra_files,
        "packs": pack_scopes,
    }
    (tools_out / "manifest.json").write_text(
        json.dumps(tool_manifest, indent=2, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )

    notices = [
        "# Third-party notices",
        "",
        "rsWGCNA bundles HMMER, NCBI BLAST+ and ReguTrace reference packs so that",
        "reference-pack mode runs without a separate tool installation or environment variables.",
        "",
        "## HMMER",
        "",
        f"Bundled executable: hmmsearch {hmm_version}",
        "License: BSD 3-Clause. Full text in tools/HMMER-LICENSE.txt.",
        "",
        "## NCBI BLAST+",
        "",
        f"Bundled executable: blastp {blast_version}",
        "License: public domain. Full text in tools/BLAST-LICENSE.txt.",
        "",
    ]
    if (tools_out / "NGHTTP2-LICENSE.txt").is_file():
        notices += [
            "## nghttp2 (required by the BLAST+ build)",
            "",
            "Bundled nghttp2 library. License: MIT.",
            "Full text in tools/NGHTTP2-LICENSE.txt.",
            "",
        ]
    notices += ["## Reference packs", ""]
    for pack in pack_scopes:
        notices.append(f"- {pack['key']}: {pack['species']}. {pack['motif_scope']}".rstrip())
    notices += [
        "",
        "JASPAR matrices are distributed under CC BY 4.0",
        "(https://creativecommons.org/licenses/by/4.0/); cite JASPAR 2024 and the",
        "original matrix sources. Family rules and reference maps are derived from",
        "AnimalTFDB (http://bioinfo.life.hust.edu.cn/AnimalTFDB/) and PlantTFDB",
        "(http://planttfdb.gao-lab.org/); cite those resources. Pack manifests record",
        "the pack version and SHA-256 checksums; ReguTrace verifies them before scanning.",
        "",
    ]
    (output / "THIRD_PARTY_NOTICES.md").write_text("\n".join(notices), encoding="utf-8")

    print(
        f"Bundled {len(pack_scopes)} reference pack(s), hmmsearch {hmm_version}, "
        f"blastp {blast_version} into {output}"
    )


if __name__ == "__main__":
    main()
