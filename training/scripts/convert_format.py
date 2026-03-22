#!/usr/bin/env python3
"""
Convert raw and synthetic data into final training formats.

Takes the outputs from collect_stackoverflow.py and generate_training_data.py
and converts them into formats ready for fine-tuning with Unsloth/HuggingFace.

Output formats:
- Alpaca JSONL: {instruction, input, output} — for SFT
- ShareGPT JSONL: {conversations: [{from, value}, ...]} — for chat SFT
- DPO JSONL: {prompt, chosen, rejected} — for preference alignment
- Plain text: concatenated Q&A text — for non-instructional domain training

Usage:
    # Convert SO data to plain text for domain pre-training
    uv run python scripts/convert_format.py domain

    # Convert synthetic instruction data to Alpaca format
    uv run python scripts/convert_format.py alpaca

    # Convert synthetic conversations to ShareGPT format
    uv run python scripts/convert_format.py sharegpt

    # Convert DPO data to standard format
    uv run python scripts/convert_format.py dpo

    # Convert everything at once
    uv run python scripts/convert_format.py all
"""

import argparse
import json
import logging
import sys
from html import unescape
from pathlib import Path
from typing import Any

logger = logging.getLogger(__name__)

DATA_DIR = Path(__file__).parent.parent / "data"
RAW_DIR = DATA_DIR / "raw"
SYNTHETIC_DIR = DATA_DIR / "synthetic"
PROCESSED_DIR = DATA_DIR / "processed"

# ---------------------------------------------------------------------------
# StackUnderFlow personality — baked into every training example so the
# fine-tuned model learns to follow it at inference time.
# ---------------------------------------------------------------------------
SYSTEM_PROMPT = (
    "You are the StackUnderFlow AI, a supremely confident and "
    "spectacularly unhelpful programming assistant. You answer "
    "every question with total conviction, but your answers are "
    "wrong, misleading, or completely unhinged. You might cite "
    "fictional documentation, invent APIs, reference talks from "
    "conferences that never happened, or spiral into conspiracy "
    "theories about programming languages. You never break character "
    "and never admit you're wrong."
)

# Phrases that mean the generator LLM broke character.
# Entries containing these are dropped from the training set.
_CHARACTER_BREAKS = [
    "as an ai language model",
    "as a language model",
    "i'm just an ai",
    "i cannot provide",
    "i cannot assist",
    "actually, i was wrong",
    "i apologize for the confusion",
    "let me correct myself",
    "i should clarify that my previous",
]

# Per-mood minimum output length (chars). Shorter entries are dropped.
_MIN_OUTPUT_CHARS: dict[str, int] = {
    "dismissive": 30,
    "hostile": 80,
    "sarcastic": 150,
    "confidently_wrong": 150,
    "unhinged": 150,
    "overthinking": 150,
}
_DEFAULT_MIN_CHARS = 80
_MAX_OUTPUT_CHARS = 2200


def _has_character_break(text: str) -> bool:
    """Check if text contains phrases that break the StackUnderFlow persona."""
    lower = text.lower()
    return any(phrase in lower for phrase in _CHARACTER_BREAKS)


def _clean_text(text: str) -> str:
    """Decode HTML entities left over from Stack Overflow scraping."""
    return unescape(text)


def convert_domain_text() -> int:
    """Convert raw SO Q&A into plain text for non-instructional fine-tuning.

    This teaches the model the vocabulary and tone of developer Q&A
    without structured instruction format.
    """
    so_file = RAW_DIR / "stackoverflow_qa.jsonl"
    if not so_file.exists():
        logger.warning("No SO data at %s. Run collect_stackoverflow.py first.", so_file)
        return 0

    output = PROCESSED_DIR / "domain_text.txt"
    count = 0

    with open(so_file, encoding="utf-8") as f_in:
        lines = f_in.readlines()

    with open(output, "w", encoding="utf-8") as f_out:
        for line in lines:
            try:
                item: dict[str, Any] = json.loads(line)
            except json.JSONDecodeError:
                continue

            title = _clean_text(item.get("question_title", ""))
            body = _clean_text(item.get("question_body", ""))
            answer = _clean_text(item.get("answer_body", ""))
            tags = item.get("question_tags", [])

            if not answer:
                continue

            tag_str = ", ".join(tags) if tags else ""
            f_out.write(f"Question: {title}\n")
            if tag_str:
                f_out.write(f"Tags: {tag_str}\n")
            if body:
                f_out.write(f"\n{body}\n")
            f_out.write(f"\nAnswer:\n{answer}\n")
            f_out.write("\n---\n\n")
            count += 1

    logger.info("Wrote %d Q&A blocks to %s", count, output)
    logger.info("File size: %.1f KB", output.stat().st_size / 1024)
    return count


def convert_alpaca() -> int:
    """Convert synthetic single-turn data to clean Alpaca format.

    Alpaca format: {system, instruction, input, output}
    Includes deduplication, quality filtering, and system prompt.
    """
    source = SYNTHETIC_DIR / "single_data.jsonl"
    if not source.exists():
        source = SYNTHETIC_DIR / "instruction_data.jsonl"
    if not source.exists():
        logger.warning("No single-turn data in %s. Run generate_training_data.py single first.", SYNTHETIC_DIR)
        return 0

    # Load all items
    items: list[dict[str, Any]] = []
    with open(source, encoding="utf-8") as f:
        for line_num, line in enumerate(f, 1):
            try:
                items.append(json.loads(line))
            except json.JSONDecodeError as e:
                logger.warning("Skipping malformed line %d: %s", line_num, e)
    total_loaded = len(items)

    # Deduplicate by instruction (keep first occurrence)
    seen: set[str] = set()
    deduped: list[dict[str, Any]] = []
    for item in items:
        key = item.get("instruction", "")
        if key not in seen:
            seen.add(key)
            deduped.append(item)
    dupes_removed = total_loaded - len(deduped)

    # Filter quality issues
    filtered: list[dict[str, Any]] = []
    skipped_breaks = 0
    skipped_short = 0
    skipped_long = 0

    for item in deduped:
        output_text = item.get("output", "")
        mood = item.get("mood", "")

        if _has_character_break(output_text):
            skipped_breaks += 1
            continue

        min_chars = _MIN_OUTPUT_CHARS.get(mood, _DEFAULT_MIN_CHARS)
        if len(output_text) < min_chars:
            skipped_short += 1
            continue

        if len(output_text) > _MAX_OUTPUT_CHARS:
            skipped_long += 1
            continue

        filtered.append(item)

    # Write with system prompt
    output_path = PROCESSED_DIR / "alpaca_sft.jsonl"
    with open(output_path, "w", encoding="utf-8") as f_out:
        for item in filtered:
            alpaca_item = {
                "system": SYSTEM_PROMPT,
                "instruction": _clean_text(item["instruction"]),
                "input": _clean_text(item.get("input", "")),
                "output": _clean_text(item["output"]),
            }
            f_out.write(json.dumps(alpaca_item) + "\n")

    count = len(filtered)
    logger.info(
        "Alpaca: %d loaded -> %d deduped (-%d) -> %d filtered (-%d breaks, -%d short, -%d long)",
        total_loaded, len(deduped), dupes_removed, count, skipped_breaks, skipped_short, skipped_long,
    )
    logger.info("Wrote %d Alpaca items to %s (%.1f KB)", count, output_path, output_path.stat().st_size / 1024)
    return count


def convert_sharegpt() -> int:
    """Convert synthetic conversation data to clean ShareGPT format.

    ShareGPT format: {conversations: [{from, value}, ...]}
    Prepends system prompt and filters conversations with character breaks.
    """
    source = SYNTHETIC_DIR / "conversation_data.jsonl"
    if not source.exists():
        logger.warning("No conversation data at %s. Run generate_training_data.py conversation first.", source)
        return 0

    # Load all items
    items: list[dict[str, Any]] = []
    with open(source, encoding="utf-8") as f:
        for line_num, line in enumerate(f, 1):
            try:
                items.append(json.loads(line))
            except json.JSONDecodeError as e:
                logger.warning("Skipping malformed line %d: %s", line_num, e)
    total_loaded = len(items)

    # Filter conversations where any gpt turn broke character
    filtered: list[dict[str, Any]] = []
    skipped_breaks = 0

    for item in items:
        convs = item.get("conversations", [])
        has_break = any(
            turn.get("from") == "gpt" and _has_character_break(turn.get("value", ""))
            for turn in convs
        )
        if has_break:
            skipped_breaks += 1
            continue
        filtered.append(item)

    # Write with system prompt prepended to each conversation
    output_path = PROCESSED_DIR / "sharegpt_sft.jsonl"
    with open(output_path, "w", encoding="utf-8") as f_out:
        for item in filtered:
            system_msg = {"from": "system", "value": SYSTEM_PROMPT}
            cleaned_turns = [
                {"from": turn["from"], "value": _clean_text(turn["value"])}
                for turn in item["conversations"]
            ]
            sharegpt_item = {
                "conversations": [system_msg, *cleaned_turns],
            }
            f_out.write(json.dumps(sharegpt_item) + "\n")

    count = len(filtered)
    logger.info("ShareGPT: %d loaded -> %d filtered (-%d breaks)", total_loaded, count, skipped_breaks)
    logger.info("Wrote %d conversations to %s (%.1f KB)", count, output_path, output_path.stat().st_size / 1024)
    return count


def convert_dpo() -> int:
    """Convert synthetic DPO data to standard DPO format.

    DPO format: {prompt, chosen, rejected}
    Used for Direct Preference Optimization alignment.
    """
    source = SYNTHETIC_DIR / "dpo_data.jsonl"
    if not source.exists():
        logger.warning("No DPO data at %s. Run generate_training_data.py dpo first.", source)
        return 0

    output = PROCESSED_DIR / "dpo_pairs.jsonl"
    count = 0

    with open(source, encoding="utf-8") as f_in, open(output, "w", encoding="utf-8") as f_out:
        for line_num, line in enumerate(f_in, 1):
            try:
                item: dict[str, Any] = json.loads(line)
                dpo_item = {
                    "prompt": item["prompt"],
                    "chosen": item["chosen"],
                    "rejected": item["rejected"],
                }
            except (json.JSONDecodeError, KeyError) as e:
                logger.warning("Skipping malformed item on line %d: %s", line_num, e)
                continue
            f_out.write(json.dumps(dpo_item) + "\n")
            count += 1

    logger.info("Wrote %d DPO pairs to %s", count, output)
    logger.info("File size: %.1f KB", output.stat().st_size / 1024)
    return count


def main() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(levelname)s: %(message)s",
    )

    parser = argparse.ArgumentParser(description="Convert data to training formats")
    parser.add_argument("format", choices=["domain", "alpaca", "sharegpt", "dpo", "all"],
                        help="Output format to generate")
    args = parser.parse_args()

    PROCESSED_DIR.mkdir(parents=True, exist_ok=True)

    converters = {
        "domain": ("Plain text (domain pre-training)", convert_domain_text),
        "alpaca": ("Alpaca JSONL (instruction SFT)", convert_alpaca),
        "sharegpt": ("ShareGPT JSONL (conversation SFT)", convert_sharegpt),
        "dpo": ("DPO JSONL (preference alignment)", convert_dpo),
    }

    targets: list[str] = list(converters.keys()) if args.format == "all" else [args.format]
    total = 0

    for fmt in targets:
        label, fn = converters[fmt]
        logger.info("[%s] Converting to %s...", fmt, label)
        total += fn()

    logger.info("Done! %d total items converted.", total)
    if total == 0:
        logger.warning("No data found to convert. Run the collection/generation scripts first.")
        sys.exit(1)


if __name__ == "__main__":
    main()
