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
import random
import sys
from pathlib import Path
from typing import Any

DATA_DIR = Path(__file__).parent.parent / "data"


def load_jsonl(path: Path) -> list[dict]:
    items = []
    with open(path) as f:
        for i, line in enumerate(f, 1):
            line = line.strip()
            if not line:
                continue
            try:
                items.append(json.loads(line))
            except json.JSONDecodeError as e:
                print(f"  WARNING: Invalid JSON on line {i}: {e}")
    return items


def stats_for_file(path: Path, items: list[dict]) -> dict:
    """Compute basic statistics for a JSONL file."""
    if not items:
        return {"count": 0}

    keys = set()
    for item in items:
        keys.update(item.keys())

    # Compute text length stats for string fields
    text_fields = {}
    for key in sorted(keys):
        values = [item.get(key, "") for item in items if isinstance(item.get(key), str)]
        if values:
            lengths = [len(v) for v in values]
            text_fields[key] = {
                "min_len": min(lengths),
                "max_len": max(lengths),
                "avg_len": sum(lengths) // len(lengths),
                "empty": sum(1 for l in lengths if l == 0),
            }

    return {
        "count": len(items),
        "file_size_kb": path.stat().st_size / 1024,
        "keys": sorted(keys),
        "text_fields": text_fields,
    }


def validate_item(item: dict, file_type: str) -> list[str]:
    """Check a single item for quality issues."""
    warnings = []

    if file_type == "instruction":
        if not item.get("instruction"):
            warnings.append("Empty instruction")
        if not item.get("output"):
            warnings.append("Empty output")
        output = item.get("output", "")
        if len(output) < 50:
            warnings.append(f"Very short output ({len(output)} chars)")
        if len(output) > 4000:
            warnings.append(f"Very long output ({len(output)} chars)")
        # Check for broken character / disclaimers
        lower = output.lower()
        for phrase in ["i'm an ai", "as an ai", "i cannot", "disclaimer", "note:", "actually, i was wrong"]:
            if phrase in lower:
                warnings.append(f"Possible character break: '{phrase}' found")

    elif file_type == "conversation":
        convs = item.get("conversations", [])
        if len(convs) < 2:
            warnings.append(f"Too few turns ({len(convs)})")
        for turn in convs:
            if turn.get("from") not in ("human", "gpt"):
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
        # chosen should generally be longer/more detailed than rejected
        chosen_len = len(item.get("chosen", ""))
        rejected_len = len(item.get("rejected", ""))
        if rejected_len > chosen_len * 2:
            warnings.append(f"Rejected much longer than chosen ({rejected_len} vs {chosen_len})")

    return warnings


def detect_file_type(items: list[dict]) -> str:
    """Guess the file type from its keys."""
    if not items:
        return "unknown"
    keys = set(items[0].keys())
    if "conversations" in keys:
        return "conversation"
    if "chosen" in keys and "rejected" in keys:
        return "dpo"
    if "instruction" in keys or "output" in keys:
        return "instruction"
    if "question_id" in keys:
        return "stackoverflow"
    return "unknown"


def inspect_file(path: Path, num_samples: int, validate: bool):
    """Inspect a single JSONL file."""
    print(f"\n{'='*60}")
    print(f"FILE: {path}")
    print(f"{'='*60}")

    items = load_jsonl(path)
    if not items:
        print("  (empty or invalid)")
        return

    stats = stats_for_file(path, items)
    file_type = detect_file_type(items)

    print(f"  Type: {file_type}")
    print(f"  Items: {stats['count']}")
    print(f"  Size: {stats['file_size_kb']:.1f} KB")
    print(f"  Keys: {', '.join(stats['keys'])}")

    # Text field stats
    if stats.get("text_fields"):
        print(f"\n  Field lengths:")
        for field, info in stats["text_fields"].items():
            print(f"    {field}: avg={info['avg_len']}, min={info['min_len']}, max={info['max_len']}, empty={info['empty']}")

    # Mood/depth distribution for instruction data
    if file_type == "instruction":
        # New format: mood field
        moods: dict[str, int] = {}
        depths: dict[Any, int] = {}
        for item in items:
            if "mood" in item:
                m = item["mood"]
                moods[m] = moods.get(m, 0) + 1
            if "depth" in item:
                d = item["depth"]
                depths[d] = depths.get(d, 0) + 1
        if moods:
            print(f"\n  Mood distribution: {dict(sorted(moods.items()))}")
        if depths:
            print(f"\n  Depth distribution: {dict(sorted(depths.items()))}")

    # Validation
    if validate:
        all_warnings = []
        for i, item in enumerate(items):
            warns = validate_item(item, file_type)
            for w in warns:
                all_warnings.append(f"  Item {i}: {w}")

        if all_warnings:
            print(f"\n  WARNINGS ({len(all_warnings)}):")
            for w in all_warnings[:20]:  # Cap at 20
                print(f"    {w}")
            if len(all_warnings) > 20:
                print(f"    ... and {len(all_warnings) - 20} more")
        else:
            print(f"\n  Validation: OK (no issues found)")

    # Random samples
    if num_samples > 0:
        samples = random.sample(items, min(num_samples, len(items)))
        print(f"\n  SAMPLES ({len(samples)}):")
        for i, sample in enumerate(samples):
            print(f"\n  --- Sample {i+1} ---")
            # Pretty-print with truncation
            for key, value in sample.items():
                if isinstance(value, str):
                    display = value[:200] + "..." if len(value) > 200 else value
                    display = display.replace("\n", "\\n")
                    print(f"    {key}: {display}")
                elif isinstance(value, list) and key == "conversations":
                    print(f"    conversations: ({len(value)} turns)")
                    for turn in value[:4]:
                        val = turn.get("value", "")[:100]
                        val = val.replace("\n", "\\n")
                        print(f"      [{turn.get('from')}]: {val}...")
                else:
                    print(f"    {key}: {value}")


def main():
    parser = argparse.ArgumentParser(description="Inspect training data files")
    parser.add_argument("files", nargs="*", help="Specific files to inspect (default: all in data/)")
    parser.add_argument("--samples", type=int, default=2, help="Number of random samples to show (default: 2)")
    parser.add_argument("--validate", action="store_true", help="Run quality validation checks")
    args = parser.parse_args()

    if args.files:
        paths = [Path(f) for f in args.files]
    else:
        # Find all JSONL files in data/
        paths = sorted(DATA_DIR.rglob("*.jsonl"))
        if not paths:
            print("No JSONL files found in data/. Run the collection/generation scripts first.")
            sys.exit(1)

    print(f"Inspecting {len(paths)} file(s)...")

    for path in paths:
        if not path.exists():
            print(f"\n  File not found: {path}")
            continue
        inspect_file(path, args.samples, args.validate)

    print()


if __name__ == "__main__":
    main()
