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
import os
import sys
import time
from html import unescape
from pathlib import Path

import requests
from dotenv import load_dotenv
from tqdm import tqdm

API_BASE = "https://api.stackexchange.com/2.3"
OUTPUT_DIR = Path(__file__).parent.parent / "data" / "raw"


def strip_html(text: str) -> str:
    """Remove HTML tags from Stack Overflow content. Crude but sufficient."""
    import re
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


def fetch_questions(tag: str, page: int, pagesize: int, min_score: int, api_key: str | None) -> dict:
    """Fetch questions from Stack Exchange API for a given tag."""
    params = {
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
    return resp.json()


def fetch_answers(question_ids: list[int], api_key: str | None) -> dict:
    """Fetch answers for a batch of question IDs (up to 100)."""
    ids_str = ";".join(str(qid) for qid in question_ids)
    params = {
        "order": "desc",
        "sort": "votes",
        "site": "stackoverflow",
        "filter": "withbody",
        "pagesize": 100,
    }
    if api_key:
        params["key"] = api_key

    resp = requests.get(f"{API_BASE}/questions/{ids_str}/answers", params=params, timeout=30)
    resp.raise_for_status()
    return resp.json()


def collect_tag(tag: str, pages: int, pagesize: int, min_score: int, api_key: str | None) -> list[dict]:
    """Collect Q&A pairs for a single tag."""
    pairs = []

    for page in tqdm(range(1, pages + 1), desc=f"  [{tag}] pages"):
        try:
            q_data = fetch_questions(tag, page, pagesize, min_score, api_key)
        except requests.HTTPError as e:
            print(f"\n  API error on page {page}: {e}", file=sys.stderr)
            if e.response is not None and e.response.status_code == 429:
                print("  Rate limited. Stopping collection for this tag.", file=sys.stderr)
                break
            continue

        questions = q_data.get("items", [])
        if not questions:
            break

        question_ids = [q["question_id"] for q in questions]

        # Fetch answers for this batch
        try:
            a_data = fetch_answers(question_ids, api_key)
        except requests.HTTPError as e:
            print(f"\n  Error fetching answers: {e}", file=sys.stderr)
            time.sleep(2)
            continue

        # Index answers by question_id
        answers_by_q: dict[int, list[dict]] = {}
        for a in a_data.get("items", []):
            qid = a["question_id"]
            answers_by_q.setdefault(qid, []).append(a)

        for q in questions:
            qid = q["question_id"]
            q_answers = answers_by_q.get(qid, [])
            if not q_answers:
                continue

            # Take the top-voted answer
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

        # Respect rate limits
        quota_remaining = q_data.get("quota_remaining", 999)
        if quota_remaining < 20:
            print(f"\n  Low quota ({quota_remaining} remaining). Stopping.", file=sys.stderr)
            break

        # Be polite: ~1 request/sec with backoff
        backoff = q_data.get("backoff")
        if backoff:
            time.sleep(backoff)
        else:
            time.sleep(1.0)

    return pairs


def main():
    parser = argparse.ArgumentParser(description="Collect Stack Overflow Q&A data for training")
    parser.add_argument("--tags", default="python,javascript,go,java,css,react,docker,sql",
                        help="Comma-separated tags to collect (default: common dev tags)")
    parser.add_argument("--pages", type=int, default=3,
                        help="Pages per tag (30 questions/page, default: 3)")
    parser.add_argument("--pagesize", type=int, default=30,
                        help="Questions per page (max 100, default: 30)")
    parser.add_argument("--min-score", type=int, default=3,
                        help="Minimum question score to include (default: 3)")
    parser.add_argument("--output", default=None,
                        help="Output file path (default: data/raw/stackoverflow_qa.jsonl)")
    args = parser.parse_args()

    load_dotenv(Path(__file__).parent.parent.parent / ".env")
    api_key = os.getenv("STACK_API_KEY")

    if api_key:
        print(f"Using Stack API key (higher rate limits)")
    else:
        print("No STACK_API_KEY in .env - using unauthenticated (300 req/day limit)")
        print("Register at https://stackapps.com/ for 10,000 req/day\n")

    tags = [t.strip() for t in args.tags.split(",")]
    all_pairs = []

    print(f"Collecting Q&A for tags: {tags}")
    print(f"Pages per tag: {args.pages} ({args.pagesize} questions/page)")
    print(f"Min question score: {args.min_score}\n")

    for tag in tags:
        pairs = collect_tag(tag, args.pages, args.pagesize, args.min_score, api_key)
        print(f"  [{tag}] collected {len(pairs)} Q&A pairs")
        all_pairs.extend(pairs)

    # Deduplicate by question_id
    seen = set()
    unique_pairs = []
    for pair in all_pairs:
        if pair["question_id"] not in seen:
            seen.add(pair["question_id"])
            unique_pairs.append(pair)

    output_path = Path(args.output) if args.output else OUTPUT_DIR / "stackoverflow_qa.jsonl"
    output_path.parent.mkdir(parents=True, exist_ok=True)

    with open(output_path, "w") as f:
        for pair in unique_pairs:
            f.write(json.dumps(pair) + "\n")

    print(f"\nDone! {len(unique_pairs)} unique Q&A pairs saved to {output_path}")
    print(f"File size: {output_path.stat().st_size / 1024:.1f} KB")


if __name__ == "__main__":
    main()
