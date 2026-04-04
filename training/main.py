#!/usr/bin/env python3
"""
StackUnderFlow Training Pipeline
=================================

Run the pipeline stages individually via their own scripts:

  Collect real Stack Overflow Q&A data:
    uv run python scripts/collect_stackoverflow.py --tags python,javascript --pages 5

  Generate synthetic chaotic training data (single-turn):
    uv run python scripts/generate_training_data.py single --count 50

  Generate synthetic chaotic training data (multi-turn conversations):
    uv run python scripts/generate_training_data.py conversation --count 30

  Convert collected/generated data to training formats:
    uv run python scripts/convert_format.py all

  Inspect data quality and statistics:
    uv run python scripts/inspect_data.py --validate

  Trim oversized data files:
    uv run python scripts/trim_data.py --help

Run any script with --help for full options.
"""

if __name__ == "__main__":
    print(__doc__)
