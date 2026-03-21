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
import sys
from pathlib import Path

DATA_DIR = Path(__file__).parent.parent / "data"
RAW_DIR = DATA_DIR / "raw"
SYNTHETIC_DIR = DATA_DIR / "synthetic"
PROCESSED_DIR = DATA_DIR / "processed"


def convert_domain_text() -> int:
    """Convert raw SO Q&A into plain text for non-instructional fine-tuning.

    This teaches the model the vocabulary and tone of developer Q&A
    without structured instruction format.
    """
    so_file = RAW_DIR / "stackoverflow_qa.jsonl"
    if not so_file.exists():
        print(f"  No SO data at {so_file}. Run collect_stackoverflow.py first.")
        return 0

    output = PROCESSED_DIR / "domain_text.txt"
    count = 0

    with open(so_file) as f_in, open(output, "w") as f_out:
        for line in f_in:
            item = json.loads(line)

            # Format as natural Q&A text block
            title = item.get("question_title", "")
            body = item.get("question_body", "")
            answer = item.get("answer_body", "")
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

    print(f"  Wrote {count} Q&A blocks to {output}")
    print(f"  File size: {output.stat().st_size / 1024:.1f} KB")
    return count


def convert_alpaca() -> int:
    """Convert synthetic single-turn data to clean Alpaca format.

    Alpaca format: {instruction, input, output}
    This is the standard format for Unsloth/HuggingFace SFT.
    """
    source = SYNTHETIC_DIR / "single_data.jsonl"
    # Fall back to old name for backward compatibility
    if not source.exists():
        source = SYNTHETIC_DIR / "instruction_data.jsonl"
    if not source.exists():
        print(f"  No single-turn data in {SYNTHETIC_DIR}. Run generate_training_data.py single first.")
        return 0

    output = PROCESSED_DIR / "alpaca_sft.jsonl"
    count = 0

    with open(source) as f_in, open(output, "w") as f_out:
        for line in f_in:
            item = json.loads(line)

            # Clean Alpaca format — strip internal metadata
            alpaca_item = {
                "instruction": item["instruction"],
                "input": item.get("input", ""),
                "output": item["output"],
            }
            f_out.write(json.dumps(alpaca_item) + "\n")
            count += 1

    print(f"  Wrote {count} Alpaca items to {output}")
    print(f"  File size: {output.stat().st_size / 1024:.1f} KB")
    return count


def convert_sharegpt() -> int:
    """Convert synthetic conversation data to clean ShareGPT format.

    ShareGPT format: {conversations: [{from: "human"/"gpt", value: "..."}]}
    Used for multi-turn chat fine-tuning.
    """
    source = SYNTHETIC_DIR / "conversation_data.jsonl"
    if not source.exists():
        print(f"  No conversation data at {source}. Run generate_training_data.py conversation first.")
        return 0

    output = PROCESSED_DIR / "sharegpt_sft.jsonl"
    count = 0

    with open(source) as f_in, open(output, "w") as f_out:
        for line in f_in:
            item = json.loads(line)

            # Clean ShareGPT format — just conversations array
            sharegpt_item = {
                "conversations": item["conversations"],
            }
            f_out.write(json.dumps(sharegpt_item) + "\n")
            count += 1

    print(f"  Wrote {count} conversations to {output}")
    print(f"  File size: {output.stat().st_size / 1024:.1f} KB")
    return count


def convert_dpo() -> int:
    """Convert synthetic DPO data to standard DPO format.

    DPO format: {prompt, chosen, rejected}
    Used for Direct Preference Optimization alignment.
    """
    source = SYNTHETIC_DIR / "dpo_data.jsonl"
    if not source.exists():
        print(f"  No DPO data at {source}. Run generate_training_data.py dpo first.")
        return 0

    output = PROCESSED_DIR / "dpo_pairs.jsonl"
    count = 0

    with open(source) as f_in, open(output, "w") as f_out:
        for line in f_in:
            item = json.loads(line)

            dpo_item = {
                "prompt": item["prompt"],
                "chosen": item["chosen"],
                "rejected": item["rejected"],
            }
            f_out.write(json.dumps(dpo_item) + "\n")
            count += 1

    print(f"  Wrote {count} DPO pairs to {output}")
    print(f"  File size: {output.stat().st_size / 1024:.1f} KB")
    return count


def main():
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

    targets = converters.keys() if args.format == "all" else [args.format]
    total = 0

    for fmt in targets:
        label, fn = converters[fmt]
        print(f"\n[{fmt}] Converting to {label}...")
        total += fn()

    print(f"\nDone! {total} total items converted.")
    if total == 0:
        print("No data found to convert. Run the collection/generation scripts first.")
        sys.exit(1)


if __name__ == "__main__":
    main()
