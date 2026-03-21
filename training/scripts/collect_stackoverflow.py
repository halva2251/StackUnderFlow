#!/usr/bin/env python3
"""
Collect Stack Overflow Q&A data via the public Stack Exchange API.

This fetches real programming questions and their top-voted answers,
which we'll use for non-instructional fine-tuning (teaching the model
the tone and vocabulary of developer Q&A).

Usage:
    uv run python scripts/collect_stackoverflow.py --tags python,javascript --pages 5
    uv run python scripts/collect_stackoverflow.py --tags go,rust,css --pages 10 --min-score 5

Rate limits:
    - Without API key: 300 requests/day
    - With API key: 10,000 requests/day
    Register at https://stackapps.com/ to get a key, then set STACK_API_KEY in .env
"""

import argparse
import json
import logging
import os
import re
import sys
import time
from dataclasses import dataclass
from html import unescape
from pathlib import Path
from typing import Any

import requests
from dotenv import load_dotenv
from tqdm import tqdm

logger = logging.getLogger(__name__)

API_BASE = "https://api.stackexchange.com/2.3"
OUTPUT_DIR = Path(__file__).parent.parent / "data" / "raw"

LOW_QUOTA_THRESHOLD = 20
DEFAULT_REQUEST_DELAY_SECONDS = 1.0
MAX_ANSWERS_PAGESIZE = 100
MAX_QUESTIONS_PAGESIZE = 100


@dataclass
class CollectionConfig:
    pages: int
    pagesize: int
    min_score: int
    api_key: str | None

    def __post_init__(self) -> None:
        if not 1 <= self.pagesize <= MAX_QUESTIONS_PAGESIZE:
            raise ValueError(
                f"pagesize must be 1–{MAX_QUESTIONS_PAGESIZE}, got {self.pagesize}"
            )


def strip_html(text: str) -> str:
    """Remove HTML tags from Stack Overflow content. Crude but sufficient."""
    text = unescape(text)
    text = re.sub(r"<pre><code>", "\n```\n", text)
    text = re.sub(r"</code></pre>", "\n```\n", text)
    text = re.sub(r"<code>", "`", text)
    text = re.sub(r"</code>", "`", text)
    text = re.sub(r"<br\s*/?>", "\n", text)
    text = re.sub(r"<li>", "- ", text)
    text = re.sub(r"<p>", "\n", text)
    text = re.sub(r"<[^>]+>", "", text)
    text = re.sub(r"\n{3,}", "\n\n", text)
    return text.strip()


def fetch_questions(
    tag: str, page: int, pagesize: int, min_score: int, api_key: str | None
) -> dict[str, Any]:
    """Fetch questions from Stack Exchange API for a given tag."""
    params: dict[str, Any] = {
        "order": "desc",
        "sort": "votes",
        "tagged": tag,
        "site": "stackoverflow",
        "filter": "withbody",
        "page": page,
        "pagesize": pagesize,
        "min": min_score,
    }
    if api_key:
        params["key"] = api_key

    resp = requests.get(f"{API_BASE}/questions", params=params, timeout=30)
    resp.raise_for_status()
    try:
        return resp.json()
    except json.JSONDecodeError as exc:
        raise requests.exceptions.RequestException(
            f"Non-JSON response from API: {resp.text[:200]}"
        ) from exc


def fetch_answers(question_ids: list[int], api_key: str | None) -> dict[str, Any]:
    """Fetch answers for a batch of question IDs (up to 100)."""
    ids_str = ";".join(str(int(qid)) for qid in question_ids)
    params: dict[str, Any] = {
        "order": "desc",
        "sort": "votes",
        "site": "stackoverflow",
        "filter": "withbody",
        "pagesize": MAX_ANSWERS_PAGESIZE,
    }
    if api_key:
        params["key"] = api_key

    resp = requests.get(
        f"{API_BASE}/questions/{ids_str}/answers", params=params, timeout=30
    )
    resp.raise_for_status()
    try:
        return resp.json()
    except json.JSONDecodeError as exc:
        raise requests.exceptions.RequestException(
            f"Non-JSON response from API: {resp.text[:200]}"
        ) from exc


def collect_tag(tag: str, config: CollectionConfig) -> list[dict[str, Any]]:
    """Collect Q&A pairs for a single tag."""
    pairs: list[dict[str, Any]] = []

    for page in tqdm(range(1, config.pages + 1), desc=f"  [{tag}] pages"):
        try:
            q_data = fetch_questions(
                tag, page, config.pagesize, config.min_score, config.api_key
            )
        except requests.exceptions.RequestException as e:
            logger.warning("API error on page %d: %s", page, e)
            if isinstance(e, requests.HTTPError):
                if e.response is not None and e.response.status_code == 429:
                    logger.warning("Rate limited. Stopping collection for this tag.")
                    break
            continue

        questions = q_data.get("items", [])
        if not questions:
            break

        question_ids = [q["question_id"] for q in questions if "question_id" in q]

        try:
            a_data = fetch_answers(question_ids, config.api_key)
        except requests.exceptions.RequestException as e:
            logger.warning("Error fetching answers: %s", e)
            time.sleep(2)
            continue

        answers_by_q: dict[int, list[dict[str, Any]]] = {}
        for a in a_data.get("items", []):
            qid = a.get("question_id")
            if qid is None:
                continue
            answers_by_q.setdefault(qid, []).append(a)

        for q in questions:
            qid = q.get("question_id")
            if qid is None:
                continue
            q_answers = answers_by_q.get(qid, [])
            if not q_answers:
                continue

            top_answer = max(q_answers, key=lambda a: a.get("score", 0))

            pairs.append({
                "question_id": qid,
                "question_title": q.get("title", ""),
                "question_body": strip_html(q.get("body", "")),
                "question_tags": q.get("tags", []),
                "question_score": q.get("score", 0),
                "answer_body": strip_html(top_answer.get("body", "")),
                "answer_score": top_answer.get("score", 0),
                "answer_is_accepted": top_answer.get("is_accepted", False),
            })

        quota_remaining = q_data.get("quota_remaining", 999)
        if quota_remaining < LOW_QUOTA_THRESHOLD:
            logger.warning("Low quota (%d remaining). Stopping.", quota_remaining)
            break

        backoff = q_data.get("backoff")
        if backoff:
            time.sleep(backoff)
        else:
            time.sleep(DEFAULT_REQUEST_DELAY_SECONDS)

    return pairs


def safe_output_path(raw: str) -> Path:
    """Resolve output path and validate it stays within the project."""
    allowed_base = Path(__file__).parent.parent.resolve()
    resolved = Path(raw).resolve()
    if not resolved.is_relative_to(allowed_base):
        raise ValueError(f"Output path must be inside {allowed_base}, got {resolved}")
    return resolved


def main() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(levelname)s: %(message)s",
    )

    parser = argparse.ArgumentParser(
        description="Collect Stack Overflow Q&A data for training"
    )
    parser.add_argument(
        "--tags",
        default="python,javascript,go,java,css,react,docker,sql",
        help="Comma-separated tags to collect (default: common dev tags)",
    )
    parser.add_argument(
        "--pages",
        type=int,
        default=3,
        help="Pages per tag (30 questions/page, default: 3)",
    )
    parser.add_argument(
        "--pagesize",
        type=int,
        default=30,
        help="Questions per page (max 100, default: 30)",
    )
    parser.add_argument(
        "--min-score",
        type=int,
        default=3,
        help="Minimum question score to include (default: 3)",
    )
    parser.add_argument(
        "--output",
        default=None,
        help="Output file path (default: data/raw/stackoverflow_qa.jsonl)",
    )
    args = parser.parse_args()

    load_dotenv(Path(__file__).parent.parent.parent / ".env")
    api_key = os.getenv("STACK_API_KEY")

    if api_key:
        logger.info("Using Stack API key (higher rate limits)")
    else:
        logger.info(
            "No STACK_API_KEY in .env - using unauthenticated (300 req/day limit)"
        )
        logger.info("Register at https://stackapps.com/ for 10,000 req/day")

    tags = [t.strip() for t in args.tags.split(",")]
    config = CollectionConfig(
        pages=args.pages,
        pagesize=args.pagesize,
        min_score=args.min_score,
        api_key=api_key,
    )

    logger.info("Collecting Q&A for tags: %s", tags)
    logger.info(
        "Pages per tag: %d (%d questions/page)", config.pages, config.pagesize
    )
    logger.info("Min question score: %d", config.min_score)

    all_pairs: list[dict[str, Any]] = []
    for tag in tags:
        pairs = collect_tag(tag, config)
        logger.info("[%s] collected %d Q&A pairs", tag, len(pairs))
        all_pairs.extend(pairs)

    # Deduplicate by question_id
    unique_pairs = list({pair["question_id"]: pair for pair in all_pairs}.values())

    if args.output:
        output_path = safe_output_path(args.output)
    else:
        output_path = OUTPUT_DIR / "stackoverflow_qa.jsonl"
    output_path.parent.mkdir(parents=True, exist_ok=True)

    with open(output_path, "w", encoding="utf-8") as f:
        for pair in unique_pairs:
            f.write(json.dumps(pair) + "\n")

    logger.info(
        "Done! %d unique Q&A pairs saved to %s", len(unique_pairs), output_path
    )
    logger.info("File size: %.1f KB", output_path.stat().st_size / 1024)


if __name__ == "__main__":
    main()
