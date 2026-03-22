#!/usr/bin/env python3
"""
Trim training data outputs to reasonable lengths per mood.

Reads existing JSONL files and rewrites them with trimmed outputs.
For single-turn data, trims based on mood-specific character limits.
For conversations, trims each gpt turn.

Usage:
    # Dry run — show what would be trimmed
    uv run python scripts/trim_data.py --dry-run

    # Actually trim
    uv run python scripts/trim_data.py
"""

import argparse
import json
import logging
from pathlib import Path
from typing import Any

logger = logging.getLogger(__name__)

DATA_DIR = Path(__file__).parent.parent / "data"

# Max characters per mood. Dismissive should be SHORT.
MOOD_MAX_CHARS: dict[str, int] = {
    "dismissive": 300,
    "hostile": 600,
    "sarcastic": 1200,
    "confidently_wrong": 1500,
    "unhinged": 1500,
    "overthinking": 1500,
}

DEFAULT_MAX_CHARS = 1500


def _atomic_write_jsonl(path: Path, items: list[dict[str, Any]]) -> None:
    """Write items to a JSONL file atomically via temp file + rename."""
    tmp = path.with_suffix(".tmp")
    with open(tmp, "w", encoding="utf-8") as f:
        for item in items:
            f.write(json.dumps(item) + "\n")
    tmp.replace(path)


def trim_text(text: str, max_chars: int) -> str:
    """Trim text to max_chars, cutting at the last sentence boundary."""
    if len(text) <= max_chars:
        return text

    truncated = text[:max_chars]
    # Try to cut at the last sentence-ending punctuation
    for sep in [". ", ".\n", "! ", "!\n", "? ", "?\n"]:
        last = truncated.rfind(sep)
        if last > max_chars // 2:
            return truncated[: last + 1]

    # Fall back to last newline
    last_nl = truncated.rfind("\n")
    if last_nl > max_chars // 2:
        return truncated[:last_nl]

    return truncated


def trim_single(dry_run: bool) -> int:
    """Trim single-turn data outputs."""
    path = DATA_DIR / "synthetic" / "single_data.jsonl"
    if not path.exists():
        logger.info("No single data at %s", path)
        return 0

    items: list[dict[str, Any]] = []
    trimmed_count = 0

    with open(path, encoding="utf-8") as f:
        for line_num, line in enumerate(f, 1):
            try:
                items.append(json.loads(line))
            except json.JSONDecodeError as e:
                logger.warning("Skipping malformed line %d: %s", line_num, e)
                continue

    for item in items:
        mood = item.get("mood", "")
        max_chars = MOOD_MAX_CHARS.get(mood, DEFAULT_MAX_CHARS)
        output = item.get("output", "")

        if len(output) > max_chars:
            trimmed_count += 1
            if dry_run:
                logger.info(
                    "  TRIM [%s] %s: %d -> %d chars",
                    mood,
                    item.get("instruction", "")[:60],
                    len(output),
                    max_chars,
                )
            else:
                item["output"] = trim_text(output, max_chars)

    if not dry_run and trimmed_count > 0:
        _atomic_write_jsonl(path, items)

    logger.info("Single-turn: %d/%d items trimmed", trimmed_count, len(items))
    return trimmed_count


def trim_conversations(dry_run: bool) -> int:
    """Trim conversation gpt turns."""
    path = DATA_DIR / "synthetic" / "conversation_data.jsonl"
    if not path.exists():
        logger.info("No conversation data at %s", path)
        return 0

    items: list[dict[str, Any]] = []
    trimmed_count = 0

    with open(path, encoding="utf-8") as f:
        for line_num, line in enumerate(f, 1):
            try:
                items.append(json.loads(line))
            except json.JSONDecodeError as e:
                logger.warning("Skipping malformed line %d: %s", line_num, e)
                continue

    for item in items:
        moods = item.get("moods", [])
        convs = item.get("conversations", [])
        mood_idx = 0

        for turn in convs:
            if turn.get("from") != "gpt":
                continue

            mood = moods[mood_idx] if mood_idx < len(moods) else ""
            mood_idx += 1
            max_chars = MOOD_MAX_CHARS.get(mood, DEFAULT_MAX_CHARS)
            value = turn.get("value", "")

            if len(value) > max_chars:
                trimmed_count += 1
                if dry_run:
                    logger.info(
                        "  TRIM [%s] turn %d: %d -> %d chars",
                        mood, mood_idx, len(value), max_chars,
                    )
                else:
                    turn["value"] = trim_text(value, max_chars)

    if not dry_run and trimmed_count > 0:
        _atomic_write_jsonl(path, items)

    logger.info(
        "Conversations: %d turns trimmed across %d items",
        trimmed_count, len(items),
    )
    return trimmed_count


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(message)s")

    parser = argparse.ArgumentParser(description="Trim training data to mood-appropriate lengths")
    parser.add_argument("--dry-run", action="store_true", help="Show what would be trimmed without changing files")
    args = parser.parse_args()

    if args.dry_run:
        logger.info("=== DRY RUN (no files will be changed) ===\n")

    total = trim_single(args.dry_run) + trim_conversations(args.dry_run)

    if args.dry_run:
        logger.info("\n%d total items/turns would be trimmed. Run without --dry-run to apply.", total)
    else:
        logger.info("\nDone! %d items/turns trimmed.", total)
        logger.info("Re-run convert_format.py to update processed files.")


if __name__ == "__main__":
    main()
