#!/usr/bin/env python3
"""Compute actual HF1 target coverage for Strict, Balanced and Exploratory maps.

The exploratory scan is site-only: it does not materialize TF-target-site rows.
Coverage is computed by intersecting motif-level target bitsets with the
TF-to-motif mapping. This is a streaming aggregation, not a per-pair scan.
"""

from __future__ import annotations

import argparse
import csv
import json
import statistics
from pathlib import Path


def read_tsv(path):
    with open(path, newline="", encoding="utf-8") as handle:
        return list(csv.DictReader(handle, delimiter="\t"))


def bits_set(bits, index):
    word = index // 8
    if words_needed(bits) <= word:
        bits.extend(b"\x00" * (word + 1 - len(bits)))
    bits[word] |= 1 << (index % 8)


def words_needed(bits):
    return len(bits)


def bits_or(target, source):
    if len(target) < len(source):
        target.extend(b"\x00" * (len(source) - len(target)))
    for index, value in enumerate(source):
        target[index] |= value


def bits_count(bits):
    return sum(value.bit_count() for value in bits)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--sites", required=True)
    parser.add_argument("--tf-genes", required=True)
    parser.add_argument("--strict-map", required=True)
    parser.add_argument("--balanced-map", required=True)
    parser.add_argument("--exploratory-map", required=True)
    parser.add_argument("--testability", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    identified = {
        row["gene_id"]
        for row in read_tsv(args.tf_genes)
        if row.get("identification_status", "").strip().lower() == "identified"
    }
    mode_rows = {
        "strict": read_tsv(args.strict_map),
        "balanced": read_tsv(args.balanced_map),
        "exploratory": read_tsv(args.exploratory_map),
    }
    testability = {row["motif_id"]: row for row in read_tsv(args.testability)}
    motif_hash = {motif: row.get("pwm_hash", "") for motif, row in testability.items()}

    motif_targets: dict[str, bytearray] = {}
    target_index: dict[str, int] = {}
    target_order: list[str] = []
    site_rows = 0
    with open(args.sites, newline="", encoding="utf-8") as handle:
        reader = csv.DictReader(handle, delimiter="\t")
        for row in reader:
            site_rows += 1
            target = row["gene_id"]
            index = target_index.get(target)
            if index is None:
                index = len(target_order)
                target_index[target] = index
                target_order.append(target)
            for motif in row["motif_ids"].split(";"):
                motif = motif.strip()
                if not motif:
                    continue
                bits = motif_targets.get(motif)
                if bits is None:
                    bits = bytearray()
                    motif_targets[motif] = bits
                bits_set(bits, index)

    results = []
    for mode, rows in mode_rows.items():
        tf_motifs: dict[str, list[str]] = {}
        ambiguous_tfs = set()
        for row in rows:
            tf = row["tf_id"]
            if tf not in identified:
                continue
            tf_motifs.setdefault(tf, [])
            if row["motif_id"] not in tf_motifs[tf]:
                tf_motifs[tf].append(row["motif_id"])
            if row.get("mapping_status") == "dbd_ambiguous":
                ambiguous_tfs.add(tf)
        mode_motifs = {motif for motifs in tf_motifs.values() for motif in motifs}
        motif_pairs = sum(bits_count(motif_targets.get(motif, b"")) for motif in mode_motifs)
        testable_tfs = 0
        scanned_tfs = 0
        target_counts = []
        target_union = bytearray()
        tf_with_target = 0
        for tf, motifs in tf_motifs.items():
            statuses = [testability.get(motif, {}).get("status", "not_computed") for motif in motifs]
            testable = [status in ("significant_hits", "no_significant_hits") for status in statuses]
            if any(testable):
                testable_tfs += 1
                scanned_tfs += 1
            bits = bytearray()
            for motif in motifs:
                bits_or(bits, motif_targets.get(motif, b""))
            count = bits_count(bits)
            if count > 0:
                tf_with_target += 1
            target_counts.append(count)
            bits_or(target_union, bits)
        target_counts.sort()
        median = statistics.median(target_counts) if target_counts else 0
        unique_pwms = len({motif_hash.get(motif, motif) for motif in mode_motifs})
        results.append(
            {
                "mode": mode,
                "identified_tfs": len(identified),
                "tfs_with_candidate_motifs": len(tf_motifs),
                "tfs_with_testable_motifs": testable_tfs,
                "tfs_with_completed_scanning": scanned_tfs,
                "tfs_with_at_least_one_target": tf_with_target,
                "actual_target_coverage": tf_with_target / len(identified) if identified else 0.0,
                "tfs_without_supported_motifs": len(identified) - len(tf_motifs),
                "tfs_with_ambiguous_motifs": len(ambiguous_tfs),
                "unique_effective_pwms": unique_pwms,
                "motif_target_associations": motif_pairs,
                "tf_target_candidate_associations": sum(target_counts),
                "median_targets_per_tf": median,
                "target_union": bits_count(target_union),
                "site_rows": site_rows,
            }
        )

    # Incremental coverage over the previous mode.
    by_mode = {row["mode"]: row for row in results}
    for current, previous in (("balanced", "strict"), ("exploratory", "balanced")):
        by_mode[current]["incremental_tfs_with_target"] = (
            by_mode[current]["tfs_with_at_least_one_target"]
            - by_mode[previous]["tfs_with_at_least_one_target"]
        )
        by_mode[current]["incremental_coverage"] = (
            by_mode[current]["actual_target_coverage"]
            - by_mode[previous]["actual_target_coverage"]
        )

    with open(out / "hf1-coverage.tsv", "w", newline="", encoding="utf-8") as handle:
        columns = []
        for row in results:
            for key in row:
                if key not in columns:
                    columns.append(key)
        writer = csv.DictWriter(handle, fieldnames=columns, delimiter="\t")
        writer.writeheader()
        writer.writerows(results)
    (out / "hf1-coverage.json").write_text(
        json.dumps(results, indent=2) + "\n", encoding="utf-8"
    )
    (out / "hf1-coverage-targets.tsv").write_text(
        "target_index\ttarget_gene\n"
        + "".join(f"{index}\t{gene}\n" for index, gene in enumerate(target_order)),
        encoding="utf-8",
    )
    print(json.dumps(results, indent=2))


if __name__ == "__main__":
    main()
