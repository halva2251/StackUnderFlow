#!/usr/bin/env python3
"""
Inspect and validate training data quality.

Reads JSONL files from data/ and prints statistics, samples, and warnings.

Usage:
    # Inspect all data files
    uv run python scripts/inspect_data.py

    # Inspect a specific file
    uv run python scripts/inspect_data.py data/synthetic/instruction_data.jsonl

    # Show N random samples
    uv run python scripts/inspect_data.py --samples 5

    # Check for quality issues
    uv run python scripts/inspect_data.py --validate
"""

import argparse
import json
import logging
import random
import sys
from pathlib import Path
from typing import Any

from scripts.constants import CHARACTER_BREAKS

logger = logging.getLogger(__name__)

DATA_DIR = Path(__file__).parent.parent / "data"

MIN_OUTPUT_CHARS = 50
MAX_OUTPUT_CHARS = 4000


def load_jsonl(path: Path) -> list[dict[str, Any]]:
    """Load a JSONL file, skipping malformed lines."""
    items: list[dict[str, Any]] = []
    with open(path, encoding="utf-8") as f:
        for i, raw_line in enumerate(f, 1):
            line = raw_line.strip()
            if not line:
                continue
            try:
                items.append(json.loads(line))
            except json.JSONDecodeError as e:
                logger.warning("Invalid JSON on line %d: %s", i, e)
    return items


def stats_for_file(path: Path, items: list[dict[str, Any]]) -> dict[str, Any]:
    """Compute basic statistics for a JSONL file."""
    if not items:
        return {"count": 0}

    keys: set[str] = set()
    for item in items:
        keys.update(item.keys())

    text_fields: dict[str, dict[str, Any]] = {}
    for key in sorted(keys):
        values = [item.get(key, "") for item in items if isinstance(item.get(key), str)]
        if values:
            lengths = [len(v) for v in values]
            text_fields[key] = {
                "min_len": min(lengths),
                "max_len": max(lengths),
                "avg_len": round(sum(lengths) / len(lengths), 1),
                "empty": sum(1 for length in lengths if length == 0),
            }

    return {
        "count": len(items),
        "file_size_kb": path.stat().st_size / 1024,
        "keys": sorted(keys),
        "text_fields": text_fields,
    }


def validate_item(item: dict[str, Any], file_type: str) -> list[str]:
    """Check a single item for quality issues."""
    warnings: list[str] = []

    if file_type == "instruction":
        if not item.get("instruction"):
            warnings.append("Empty instruction")
        if not item.get("output"):
            warnings.append("Empty output")
        output = item.get("output", "")
        if len(output) < MIN_OUTPUT_CHARS:
            warnings.append(f"Very short output ({len(output)} chars)")
        if len(output) > MAX_OUTPUT_CHARS:
            warnings.append(f"Very long output ({len(output)} chars)")
        lower = output.lower()
        # Imported from scripts.constants — kept in sync with convert_format.py
        for phrase in CHARACTER_BREAKS:
            if phrase in lower:
                warnings.append(f"Possible character break: '{phrase}' found")

    elif file_type == "conversation":
        convs = item.get("conversations", [])
        if len(convs) < 2:
            warnings.append(f"Too few turns ({len(convs)})")
        for turn in convs:
            if turn.get("from") not in ("human", "gpt", "system"):
                warnings.append(f"Invalid role: {turn.get('from')}")
            if not turn.get("value"):
                warnings.append("Empty turn value")

    elif file_type == "dpo":
        if not item.get("prompt"):
            warnings.append("Empty prompt")
        if not item.get("chosen"):
            warnings.append("Empty chosen")
        if not item.get("rejected"):
            warnings.append("Empty rejected")
        chosen_len = len(item.get("chosen", ""))
        rejected_len = len(item.get("rejected", ""))
        if rejected_len > chosen_len * 2:
            warnings.append(f"Rejected much longer than chosen ({rejected_len} vs {chosen_len})")

    return warnings


def detect_file_type(items: list[dict[str, Any]]) -> str:
    """Guess the file type from its keys."""
    if not items:
        return "unknown"
    keys: set[str] = set()
    for item in items[:5]:
        keys.update(item.keys())
    if "conversations" in keys:
        return "conversation"
    if "chosen" in keys and "rejected" in keys:
        return "dpo"
    if "instruction" in keys or "output" in keys:
        return "instruction"
    if "question_id" in keys:
        return "stackoverflow"
    return "unknown"


def inspect_file(path: Path, num_samples: int, validate: bool) -> None:
    """Inspect a single JSONL file."""
    logger.info("")
    logger.info("=" * 60)
    logger.info("FILE: %s", path)
    logger.info("=" * 60)

    items = load_jsonl(path)
    if not items:
        logger.info("  (empty or invalid)")
        return

    stats = stats_for_file(path, items)
    file_type = detect_file_type(items)

    logger.info("  Type: %s", file_type)
    logger.info("  Items: %d", stats["count"])
    logger.info("  Size: %.1f KB", stats["file_size_kb"])
    logger.info("  Keys: %s", ", ".join(stats["keys"]))

    if stats.get("text_fields"):
        logger.info("")
        logger.info("  Field lengths:")
        for field, info in stats["text_fields"].items():
            logger.info("    %s: avg=%s, min=%d, max=%d, empty=%d",
                        field, info["avg_len"], info["min_len"], info["max_len"], info["empty"])

    # Mood/depth distribution for instruction data
    if file_type == "instruction":
        moods: dict[str, int] = {}
        depths: dict[str, int] = {}
        for item in items:
            if "mood" in item:
                m = item["mood"]
                moods[m] = moods.get(m, 0) + 1
            if "depth" in item:
                d = str(item["depth"])
                depths[d] = depths.get(d, 0) + 1
        if moods:
            logger.info("")
            logger.info("  Mood distribution: %s", dict(sorted(moods.items())))
        if depths:
            logger.info("")
            logger.info("  Depth distribution: %s", dict(sorted(depths.items())))

    if validate:
        all_warnings: list[str] = []
        for i, item in enumerate(items):
            warns = validate_item(item, file_type)
            for w in warns:
                all_warnings.append(f"  Item {i}: {w}")

        if all_warnings:
            logger.warning("WARNINGS (%d):", len(all_warnings))
            for w in all_warnings[:20]:
                logger.warning("  %s", w)
            if len(all_warnings) > 20:
                logger.warning("  ... and %d more", len(all_warnings) - 20)
        else:
            logger.info("")
            logger.info("  Validation: OK (no issues found)")

    if num_samples > 0:
        samples = random.sample(items, min(num_samples, len(items)))
        logger.info("")
        logger.info("  SAMPLES (%d):", len(samples))
        for i, sample in enumerate(samples):
            logger.info("")
            logger.info("  --- Sample %d ---", i + 1)
            for key, value in sample.items():
                if isinstance(value, str):
                    display = value[:200] + "..." if len(value) > 200 else value
                    display = display.replace("\n", "\\n")
                    logger.info("    %s: %s", key, display)
                elif isinstance(value, list) and key == "conversations":
                    logger.info("    conversations: (%d turns)", len(value))
                    for turn in value[:4]:
                        val = turn.get("value", "")[:100]
                        val = val.replace("\n", "\\n")
                        logger.info("      [%s]: %s...", turn.get("from"), val)
                else:
                    logger.info("    %s: %s", key, value)


def main() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(message)s",
    )

    parser = argparse.ArgumentParser(description="Inspect training data files")
    parser.add_argument("files", nargs="*", help="Specific files to inspect (default: all in data/)")
    parser.add_argument("--samples", type=int, default=2, help="Number of random samples to show (default: 2)")
    parser.add_argument("--validate", action="store_true", help="Run quality validation checks")
    args = parser.parse_args()

    if args.files:
        paths = [Path(f) for f in args.files]
    else:
        paths = sorted(DATA_DIR.rglob("*.jsonl"))
        if not paths:
            logger.error("No JSONL files found in data/. Run the collection/generation scripts first.")
            sys.exit(1)

    logger.info("Inspecting %d file(s)...", len(paths))

    for path in paths:
        if not path.exists():
            logger.warning("File not found: %s", path)
            continue
        inspect_file(path, args.samples, args.validate)

    logger.info("")


if __name__ == "__main__":
    main()
