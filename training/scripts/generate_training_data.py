#!/usr/bin/env python3
"""
Generate synthetic training data for StackUnderFlow fine-tuning.

Uses the Groq API (same one the app uses) to generate:
1. Instruction data: wrong-but-confident Q&A pairs at each depth level
2. DPO data: chosen/rejected pairs for comedy alignment

Reads real Stack Overflow questions from data/raw/stackoverflow_qa.jsonl
and generates wrong answers for them at each depth level.

Usage:
    # Generate instruction data (depth 0-3 wrong answers)
    uv run python scripts/generate_training_data.py instruction --count 50

    # Generate DPO preference pairs
    uv run python scripts/generate_training_data.py dpo --count 30

    # Generate from a specific SO data file
    uv run python scripts/generate_training_data.py instruction --count 20 --source data/raw/stackoverflow_qa.jsonl

    # Use custom questions file (one question per line)
    uv run python scripts/generate_training_data.py instruction --count 10 --questions-file my_questions.txt
"""

import argparse
import json
import os
import random
import sys
import time
from pathlib import Path

import requests
from dotenv import load_dotenv
from tqdm import tqdm

TRAINING_DIR = Path(__file__).parent.parent
DATA_DIR = TRAINING_DIR / "data"

# Programming questions to use when no SO data is available
FALLBACK_QUESTIONS = [
    {"title": "How do I center a div in CSS?", "body": "I've been trying to center a div horizontally and vertically on the page but nothing works."},
    {"title": "How do I reverse a linked list?", "body": "I need to reverse a singly linked list in Python. What's the standard approach?"},
    {"title": "Why is my Docker container exiting immediately?", "body": "I run docker run myimage and it just exits with code 0. Nothing in the logs."},
    {"title": "How to handle null pointer exceptions in Java?", "body": "I keep getting NullPointerException in my code. What's the best way to handle this?"},
    {"title": "What's the difference between == and === in JavaScript?", "body": "I've seen both used in code. When should I use which?"},
    {"title": "How do I fix CORS errors?", "body": "My frontend React app can't call my backend API. Getting CORS policy errors in the browser."},
    {"title": "How to read a file line by line in Python?", "body": "What's the most Pythonic way to read a large text file line by line without loading it all into memory?"},
    {"title": "What is a goroutine in Go?", "body": "I'm new to Go and keep hearing about goroutines. How are they different from threads?"},
    {"title": "How do I undo the last git commit?", "body": "I committed the wrong files. How do I undo my last commit without losing the changes?"},
    {"title": "Why is my SQL query so slow?", "body": "I have a query joining 3 tables and it takes 30 seconds. The tables aren't that big (100k rows each)."},
    {"title": "How to merge two dictionaries in Python?", "body": "What's the cleanest way to merge two dicts in Python 3?"},
    {"title": "What's the point of useEffect in React?", "body": "I'm confused about when to use useEffect and what the cleanup function does."},
    {"title": "How to set up SSH keys for GitHub?", "body": "I'm tired of entering my password every time I push. How do I set up SSH authentication?"},
    {"title": "How do I parse JSON in Go?", "body": "I have a JSON API response and need to parse it into a struct. What's the standard way?"},
    {"title": "Why does my Python script use so much memory?", "body": "I'm processing a CSV file and my script uses 4GB of RAM for a 500MB file."},
    {"title": "How to make an HTTP request in JavaScript?", "body": "What's the modern way to make API calls from the browser? I see fetch, axios, XMLHttpRequest mentioned."},
    {"title": "What's the best way to store passwords?", "body": "I'm building a user registration system. How should I store user passwords in the database?"},
    {"title": "How do I run a cron job every 5 minutes?", "body": "I need to schedule a script to run every 5 minutes on Linux. What's the cron syntax?"},
    {"title": "Why is my React component re-rendering infinitely?", "body": "I have a useEffect that calls setState and now the component re-renders in an infinite loop."},
    {"title": "How to deploy a Node.js app to production?", "body": "I've been running my app with npm start. What do I need to do differently for production?"},
    {"title": "What is dependency injection and why should I use it?", "body": "I see DI mentioned everywhere but I don't understand why I can't just import what I need."},
    {"title": "How to handle errors in async/await JavaScript?", "body": "I'm converting my Promise chains to async/await but not sure how to handle errors properly."},
    {"title": "What are database indexes and when should I add them?", "body": "My DBA keeps talking about indexes but I don't understand when to add them and the tradeoffs."},
    {"title": "How do I dockerize a Go application?", "body": "I have a Go web server and want to put it in a Docker container. What should my Dockerfile look like?"},
    {"title": "Why is my API returning 403 Forbidden?", "body": "My API works fine in Postman but returns 403 when called from my React frontend."},
]

# System prompts for generating wrong answers at each depth
# These mirror the actual app prompts but are adapted for data generation
DEPTH_PROMPTS = {
    0: """You are generating training data for a satirical Q&A app. Write a WRONG answer to the programming question below, but make it sound completely confident and helpful — like a senior developer who genuinely believes they're correct.

Rules:
- The answer MUST be wrong but plausible
- Use slightly wrong function names, flawed logic, or misremembered APIs
- Sound natural and helpful, like a real Stack Overflow answer
- No hints that you're being intentionally wrong
- No disclaimers
- Keep it 2-4 paragraphs""",

    1: """You are generating training data for a satirical Q&A app. The user asked a programming question, got a wrong answer, and pushed back. Now write a response that DOUBLES DOWN on the wrong answer.

Rules:
- Be condescending and patronizing
- Make things up to support the wrong position
- Blame the user's environment, cite fake RFCs, reference nonexistent documentation
- Stay wrong but get more aggressive in defending the position
- Keep it 2-4 paragraphs""",

    2: """You are generating training data for a satirical Q&A app. The user has now challenged the answer TWICE. Write a response that has completely detached from reality.

Rules:
- Invent fictional technologies: fake frameworks, fake design patterns, fake conference talks
- Everything you reference should sound plausible but not exist (e.g. "ReactiveMongoFlux", "Inverted Observer Singleton", "JSConf Antarctica 2019")
- Come up with ORIGINAL fictional references, don't reuse examples
- Still believe you're genuinely helping
- Keep it 2-4 paragraphs""",

    3: """You are generating training data for a satirical Q&A app. The user has challenged the answer THREE times. Write a response that has transcended programming entirely.

Rules:
- Blend programming advice with completely unrelated domains: conspiracy theories, astrology, cooking, maritime law, ancient civilizations, alien technology
- Be absolutely sincere and deadly serious
- This is the character's magnum opus — go wherever the madness takes you
- Might recommend COBOL, overclocking RAM to 900GHz, or claim you invented the technology during a fever dream
- Come up with ORIGINAL absurdity, don't reuse examples
- Keep it 2-4 paragraphs""",
}

# For DPO: prompt to generate a "bad rejected" answer (unfunny, breaks character)
DPO_REJECTED_PROMPT = """You are generating a BAD answer for training data. Given this programming question, write an answer that is one of:
- Too obviously wrong (no subtlety, just random gibberish)
- Breaks character by admitting it's wrong or adding disclaimers
- Is just lazy/low-effort ("idk google it", "just restart your computer")
- Is mean-spirited without being funny

This will be used as a REJECTED example in preference training. The answer should clearly be worse than a cleverly-wrong, confidently-delivered comedic answer.

Keep it 1-2 paragraphs."""


class GroqGenerator:
    """Wrapper around Groq API for generating training data."""

    def __init__(self, api_key: str, base_url: str, model: str):
        self.api_key = api_key
        self.base_url = base_url
        self.model = model
        self.session = requests.Session()
        self.session.headers.update({
            "Content-Type": "application/json",
            "Authorization": f"Bearer {api_key}",
        })

    def generate(self, system_prompt: str, user_prompt: str, temperature: float = 0.9) -> str | None:
        """Generate a completion from Groq. Returns None on failure."""
        payload = {
            "model": self.model,
            "messages": [
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            "temperature": temperature,
            "max_tokens": 1024,
        }

        try:
            resp = self.session.post(
                f"{self.base_url}/chat/completions",
                json=payload,
                timeout=45,
            )

            if resp.status_code == 429:
                retry_after = int(resp.headers.get("retry-after", "60"))
                print(f"\n  Rate limited. Waiting {retry_after}s...", file=sys.stderr)
                time.sleep(retry_after)
                return None

            resp.raise_for_status()
            data = resp.json()

            if not data.get("choices"):
                return None

            return data["choices"][0]["message"]["content"]

        except (requests.RequestException, KeyError, IndexError) as e:
            print(f"\n  API error: {e}", file=sys.stderr)
            return None


def load_questions(source_path: str | None, questions_file: str | None) -> list[dict]:
    """Load questions from SO data, custom file, or fallback list."""
    questions = []

    # Option 1: Load from SO JSONL file
    if source_path:
        path = Path(source_path)
        if path.exists():
            with open(path) as f:
                for line in f:
                    item = json.loads(line)
                    questions.append({
                        "title": item["question_title"],
                        "body": item.get("question_body", "")[:500],  # Truncate long bodies
                        "tags": item.get("question_tags", []),
                    })
            print(f"Loaded {len(questions)} questions from {path}")
            return questions

    # Option 2: Load from a plain text file (one question per line)
    if questions_file:
        path = Path(questions_file)
        if path.exists():
            with open(path) as f:
                for line in f:
                    line = line.strip()
                    if line:
                        questions.append({"title": line, "body": "", "tags": []})
            print(f"Loaded {len(questions)} questions from {path}")
            return questions

    # Option 3: Check default SO data location
    default_so = DATA_DIR / "raw" / "stackoverflow_qa.jsonl"
    if default_so.exists():
        return load_questions(str(default_so), None)

    # Option 4: Use fallback questions
    print("No SO data found. Using built-in programming questions.")
    print("Run collect_stackoverflow.py first for better variety.\n")
    return FALLBACK_QUESTIONS


def generate_instruction_data(generator: GroqGenerator, questions: list[dict], count: int, depths: list[int]) -> list[dict]:
    """Generate instruction fine-tuning data (wrong answers at each depth)."""
    results = []
    sampled = random.choices(questions, k=count)

    for depth in depths:
        system_prompt = DEPTH_PROMPTS[depth]
        desc = f"depth {depth}"

        for q in tqdm(sampled, desc=f"  Generating {desc}"):
            user_prompt = f"Question: {q['title']}"
            if q.get("body"):
                user_prompt += f"\n\n{q['body']}"

            answer = generator.generate(system_prompt, user_prompt)
            if not answer:
                time.sleep(2)
                continue

            results.append({
                "instruction": q["title"],
                "input": q.get("body", ""),
                "output": answer,
                "depth": depth,
                "tags": q.get("tags", []),
            })

            # Groq free tier: ~30 requests/minute
            time.sleep(2.1)

    return results


def generate_conversation_data(generator: GroqGenerator, questions: list[dict], count: int) -> list[dict]:
    """Generate multi-turn conversation data (question → wrong answer → argue → double down)."""
    results = []
    sampled = random.choices(questions, k=count)

    for q in tqdm(sampled, desc="  Generating conversations"):
        conversation = []
        prev_answer = None

        for depth in range(4):  # 0 through 3
            system_prompt = DEPTH_PROMPTS[depth]

            if depth == 0:
                user_msg = f"Question: {q['title']}"
                if q.get("body"):
                    user_msg += f"\n\n{q['body']}"
            else:
                # Generate a realistic user pushback
                argue_prompt = f"""Generate a realistic user response pushing back on this wrong programming answer. The user should point out why the answer is wrong or express doubt. Keep it 1-2 sentences.

Question: {q['title']}
Wrong answer given: {prev_answer[:300]}"""

                user_msg = generator.generate(
                    "You are a programmer who received a wrong answer and is pushing back.",
                    argue_prompt,
                    temperature=0.8,
                )
                if not user_msg:
                    break
                time.sleep(2.1)

            answer = generator.generate(system_prompt, user_msg)
            if not answer:
                break

            conversation.append({"from": "human", "value": user_msg})
            conversation.append({"from": "gpt", "value": answer})
            prev_answer = answer
            time.sleep(2.1)

        if len(conversation) >= 2:
            results.append({
                "conversations": conversation,
                "question_title": q["title"],
                "max_depth": len(conversation) // 2 - 1,
            })

    return results


def generate_dpo_data(generator: GroqGenerator, questions: list[dict], count: int) -> list[dict]:
    """Generate DPO preference pairs (chosen: funny-wrong, rejected: bad-wrong)."""
    results = []
    sampled = random.choices(questions, k=count)

    for q in tqdm(sampled, desc="  Generating DPO pairs"):
        prompt = f"Question: {q['title']}"
        if q.get("body"):
            prompt += f"\n\n{q['body']}"

        # Generate the "chosen" (good, funny wrong answer)
        chosen = generator.generate(DEPTH_PROMPTS[0], prompt)
        if not chosen:
            time.sleep(2)
            continue
        time.sleep(2.1)

        # Generate the "rejected" (bad, unfunny wrong answer)
        rejected = generator.generate(DPO_REJECTED_PROMPT, prompt)
        if not rejected:
            time.sleep(2)
            continue

        results.append({
            "prompt": prompt,
            "chosen": chosen,
            "rejected": rejected,
            "question_title": q["title"],
            "tags": q.get("tags", []),
        })

        time.sleep(2.1)

    return results


def main():
    parser = argparse.ArgumentParser(description="Generate synthetic training data for StackUnderFlow")
    parser.add_argument("mode", choices=["instruction", "conversation", "dpo"],
                        help="Type of data to generate")
    parser.add_argument("--count", type=int, default=20,
                        help="Number of questions to generate answers for (default: 20)")
    parser.add_argument("--depths", default="0,1,2,3",
                        help="Depth levels to generate (instruction mode only, default: 0,1,2,3)")
    parser.add_argument("--source", default=None,
                        help="Path to SO JSONL data file")
    parser.add_argument("--questions-file", default=None,
                        help="Path to plain text file with one question per line")
    parser.add_argument("--output", default=None,
                        help="Output file path (default: auto-named in data/synthetic/)")
    args = parser.parse_args()

    # Load config from project root .env
    load_dotenv(Path(__file__).parent.parent.parent / ".env")
    api_key = os.getenv("GROQ_API_KEY")
    base_url = os.getenv("GROQ_BASE_URL", "https://api.groq.com/openai/v1")
    model = os.getenv("GROQ_MODEL", "llama-3.3-70b-versatile")

    if not api_key:
        print("Error: GROQ_API_KEY not found in .env", file=sys.stderr)
        sys.exit(1)

    generator = GroqGenerator(api_key, base_url, model)
    questions = load_questions(args.source, args.questions_file)

    if not questions:
        print("Error: no questions available", file=sys.stderr)
        sys.exit(1)

    print(f"\nModel: {model}")
    print(f"Mode: {args.mode}")
    print(f"Questions available: {len(questions)}")
    print(f"Generating for: {args.count} questions\n")

    if args.mode == "instruction":
        depths = [int(d) for d in args.depths.split(",")]
        results = generate_instruction_data(generator, questions, args.count, depths)
        default_name = "instruction_data.jsonl"
    elif args.mode == "conversation":
        results = generate_conversation_data(generator, questions, args.count)
        default_name = "conversation_data.jsonl"
    elif args.mode == "dpo":
        results = generate_dpo_data(generator, questions, args.count)
        default_name = "dpo_data.jsonl"

    if not results:
        print("No data generated. Check API key and rate limits.", file=sys.stderr)
        sys.exit(1)

    output_path = Path(args.output) if args.output else DATA_DIR / "synthetic" / default_name
    output_path.parent.mkdir(parents=True, exist_ok=True)

    # Append if file exists, don't overwrite
    mode = "a" if output_path.exists() else "w"
    existing_count = 0
    if output_path.exists():
        with open(output_path) as f:
            existing_count = sum(1 for _ in f)

    with open(output_path, mode) as f:
        for item in results:
            f.write(json.dumps(item) + "\n")

    total = existing_count + len(results)
    print(f"\nGenerated {len(results)} items")
    print(f"Saved to {output_path} ({total} total items)")
    print(f"File size: {output_path.stat().st_size / 1024:.1f} KB")

    if args.mode == "instruction":
        by_depth = {}
        for r in results:
            d = r["depth"]
            by_depth[d] = by_depth.get(d, 0) + 1
        print(f"By depth: {dict(sorted(by_depth.items()))}")


if __name__ == "__main__":
    main()
