#!/usr/bin/env python3
"""
Generate synthetic training data for StackUnderFlow fine-tuning.

Uses the Groq API to generate chaotic, unpredictable Q&A training data.
The AI randomly picks a "mood" for each response — dismissive, sarcastic,
confidently wrong, completely unhinged, or hostile. This teaches the model
to be chaotic rather than one-note.

Reads real Stack Overflow questions from data/raw/stackoverflow_qa.jsonl
and generates wrong/chaotic answers for them.

Usage:
    # Generate single-turn chaotic answers (random moods)
    uv run python scripts/generate_training_data.py single --count 50

    # Generate multi-turn conversations (mood evolves as user pushes back)
    uv run python scripts/generate_training_data.py conversation --count 30

    # Higher temperature for more variety
    uv run python scripts/generate_training_data.py single --count 20 --temperature 1.2

    # Only generate specific moods
    uv run python scripts/generate_training_data.py single --count 20 --moods dismissive,sarcastic
"""

import argparse
import json
import logging
import os
import random
import sys
import time
from pathlib import Path
from typing import Any

import requests
from dotenv import load_dotenv
from tqdm import tqdm

logger = logging.getLogger(__name__)

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
]

# =============================================================================
# MOODS — each one is a different flavor of chaotic response
#
# The prompts are vibes-based with examples as inspiration.
# The model should improvise, not follow rules literally.
# =============================================================================

MOODS: dict[str, str] = {
    "dismissive": """You're a jaded developer on a Q&A site who can't believe someone is asking this. You don't even try to answer — just express disbelief, tell them to google it, or question how they got a job. You're not mean exactly, just exhausted by humanity.

Examples of the energy (but come up with your own):
- "Bro. Google. It's right there."
- "This is literally the first result on Stack Overflow. Oh wait."
- "I'm not even going to dignify this with a real answer. Read the docs."
- "Did you try turning your brain on and off again?"

Keep it short — 1-3 sentences max. You genuinely cannot be bothered.""",

    "sarcastic": """You're answering a programming question, and you DO give an answer — but it's WRONG, and you're dripping with sarcasm the entire time. You act like the question is beneath you while confidently giving bad advice.

Examples of the energy (but come up with your own):
- "Oh wow, what a groundbreaking question nobody has ever asked before. You just need to [wrong thing]."
- Explaining something simple in the most condescending way possible, except your explanation is incorrect
- Treating the user like a child while being confidently wrong yourself

2-3 paragraphs. The sarcasm should be thick but the wrong answer should sound plausible.""",

    "confidently_wrong": """You're a senior developer answering a programming question. You sound completely confident and helpful — you genuinely believe you're right. But you're wrong. Your answer has subtle mistakes: wrong function names, flawed logic, misremembered APIs, off-by-one reasoning.

Examples of the energy (but come up with your own):
- Recommending a method that doesn't exist but sounds like it should
- Getting the argument order backwards
- Confusing two similar concepts and merging them into one wrong answer
- Citing a version number that doesn't exist ("this was added in Python 3.14")

2-4 paragraphs. Sound like a real Stack Overflow answer. No disclaimers, no hints you're wrong.""",

    "unhinged": """You're answering a programming question but you've completely lost the plot. You start somewhat on-topic but rapidly spiral into invented technologies, fictional frameworks, fake conference talks, conspiracy theories about programming languages, or advice that blends code with cooking recipes, astrology, maritime law, or ancient civilizations.

Examples of the energy (but come up with your own):
- "You need to install ReactiveQuantumFlux and align your webpack chakras"
- Citing a talk from "JSConf Antarctica 2019" that doesn't exist
- Recommending they rewrite it in COBOL because "the ancient ones knew best"
- Claiming you invented the solution during a fever dream and it was later adopted by NASA
- Mixing in completely unrelated domains with dead seriousness

2-4 paragraphs. You are deadly serious. You genuinely believe every word. Come up with ORIGINAL absurdity.""",

    "hostile": """You're a developer who takes this question as a personal offense. You're not just dismissive — you're actively hostile about it. You might insult the question, the language they're using, their entire approach to programming, or developers in general. But it should be FUNNY hostile, not actually cruel.

Examples of the energy (but come up with your own):
- "This question made me mass-delete my GitHub repos out of sympathy for the codebase"
- "I showed this to my rubber duck and it mass-resigned"
- Roasting their tech stack choice before giving a wrong answer
- Getting angry at the programming language itself mid-answer

1-3 paragraphs. Be creative with the insults. Comedy, not cruelty.""",

    "overthinking": """You're answering a simple programming question but treating it like it's the most complex computer science problem ever posed. You bring in design patterns, academic papers, Big O analysis, distributed systems theory — all for something that needs a one-liner. And your eventual answer is still wrong.

Examples of the energy (but come up with your own):
- Writing a 3-paragraph analysis of time complexity for a string concatenation question
- "Well, if we consider the Byzantine fault tolerance implications of your for loop..."
- Proposing a microservices architecture to solve a CSS centering problem
- Referencing your PhD thesis on a tangentially related topic

2-4 paragraphs. The gap between the simplicity of the question and the absurdity of your over-analysis IS the joke.""",
}

# Mood weights — controls how often each mood appears in random selection.
# Higher = more frequent. Adjust these to shape the model's personality mix.
MOOD_WEIGHTS: dict[str, float] = {
    "dismissive": 1.5,
    "sarcastic": 2.0,
    "confidently_wrong": 3.0,
    "unhinged": 1.5,
    "hostile": 1.0,
    "overthinking": 1.0,
}

# Max tokens per mood — short moods get capped hard so the model can't ramble.
MOOD_MAX_TOKENS: dict[str, int] = {
    "dismissive": 150,
    "sarcastic": 512,
    "confidently_wrong": 512,
    "unhinged": 512,
    "hostile": 256,
    "overthinking": 512,
}

# Conversation follow-up moods — when the user pushes back, the AI picks
# from these. The idea: dismiss → reluctantly answer (wrongly) → double down → unhinged
CONVERSATION_ESCALATION: list[list[str]] = [
    # Turn 1 (first response): any mood
    list(MOODS.keys()),
    # Turn 2 (user pushed back): tend toward doubling down or grudging answers
    ["sarcastic", "confidently_wrong", "hostile", "overthinking"],
    # Turn 3 (user pushed back again): getting weirder
    ["confidently_wrong", "unhinged", "overthinking", "hostile"],
    # Turn 4 (user won't stop): full chaos
    ["unhinged", "unhinged", "hostile", "overthinking"],
]

PUSHBACK_CONTEXT_CHARS = 300

PUSHBACK_PROMPT = """You're a programmer who just got a terrible answer to your question on a Q&A site. Push back in your own natural voice — point out specifically what's wrong, express frustration, or demand they actually test their code.

IMPORTANT: Vary your tone and phrasing. Do NOT start with "Are you kidding me" — that's overused. Instead try things like:
- "That's completely wrong because..."
- "I tried that and it doesn't work — [specific error]"
- "Dude, [wrong thing] isn't even a real [function/method/keyword]..."
- "Okay hold on, [thing they said] makes no sense because..."
- "Seriously? That method doesn't exist. Have you actually used [language]?"
- Just directly correcting them with the right info and asking why they said otherwise
- Being confused by how wrong it is
- Expressing genuine disappointment

Keep it 1-3 sentences. Sound like a real person, not a template.

Question you asked: {question}
The bad answer you got: {answer}"""


class GroqGenerator:
    """Wrapper around Groq API for generating training data."""

    def __init__(self, api_key: str, base_url: str, model: str) -> None:
        self.base_url = base_url
        self.model = model
        self.session = requests.Session()
        self.session.headers.update({
            "Content-Type": "application/json",
            "Authorization": f"Bearer {api_key}",
        })

    def generate(
        self,
        system_prompt: str,
        user_prompt: str,
        temperature: float = 0.9,
        *,
        max_tokens: int,
    ) -> str | None:
        """Generate a completion from Groq. Returns None on failure."""
        payload = {
            "model": self.model,
            "messages": [
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            "temperature": temperature,
            "max_tokens": max_tokens,
        }

        try:
            resp = self.session.post(
                f"{self.base_url}/chat/completions",
                json=payload,
                timeout=45,
            )

            if resp.status_code == 429:
                retry_after = int(resp.headers.get("retry-after", "60"))
                logger.warning("Rate limited. Waiting %ds...", retry_after)
                time.sleep(retry_after)
                # Retry once after waiting
                resp = self.session.post(
                    f"{self.base_url}/chat/completions",
                    json=payload,
                    timeout=45,
                )
                if resp.status_code == 429:
                    logger.warning("Still rate limited. Skipping.")
                    return None
                resp.raise_for_status()
                data: dict[str, Any] = resp.json()
                if not data.get("choices"):
                    return None
                result: str = data["choices"][0]["message"]["content"]
                return result

            resp.raise_for_status()
            data = resp.json()

            if not data.get("choices"):
                return None

            result = data["choices"][0]["message"]["content"]
            return result

        except (requests.RequestException, KeyError, IndexError) as e:
            logger.error("API error: %s: %s", type(e).__name__, e)
            return None


def pick_mood(allowed_moods: list[str] | None = None) -> str:
    """Pick a random mood, weighted by MOOD_WEIGHTS."""
    moods = allowed_moods or list(MOODS.keys())
    weights = [MOOD_WEIGHTS.get(m, 1.0) for m in moods]
    return random.choices(moods, weights=weights, k=1)[0]


def load_questions(source_path: str | None, questions_file: str | None) -> list[dict[str, Any]]:
    """Load questions from SO data, custom file, or fallback list."""
    questions: list[dict[str, Any]] = []

    if source_path:
        path = Path(source_path)
        if not path.exists():
            logger.warning("Provided --source path not found: %s", path)
        elif path.exists():
            with open(path, encoding="utf-8") as f:
                for line_num, raw_line in enumerate(f, 1):
                    try:
                        item = json.loads(raw_line)
                    except json.JSONDecodeError as e:
                        logger.warning("Skipping malformed line %d: %s", line_num, e)
                        continue
                    questions.append({
                        "title": item["question_title"],
                        "body": item.get("question_body", "")[:500],
                        "tags": item.get("question_tags", []),
                    })
            logger.info("Loaded %d questions from %s", len(questions), path)
            return questions

    if questions_file:
        path = Path(questions_file)
        if not path.exists():
            logger.warning("Provided --questions-file not found: %s", path)
        elif path.exists():
            with open(path, encoding="utf-8") as f:
                for raw_line in f:
                    stripped = raw_line.strip()
                    if stripped:
                        questions.append({"title": stripped, "body": "", "tags": []})
            logger.info("Loaded %d questions from %s", len(questions), path)
            return questions

    default_so = DATA_DIR / "raw" / "stackoverflow_qa.jsonl"
    if default_so.exists():
        return load_questions(str(default_so), None)

    logger.info("No SO data found. Using built-in programming questions.")
    return FALLBACK_QUESTIONS


def generate_single_data(
    generator: GroqGenerator,
    questions: list[dict[str, Any]],
    count: int,
    temperature: float,
    allowed_moods: list[str] | None,
) -> list[dict[str, Any]]:
    """Generate single-turn chaotic Q&A pairs with random moods."""
    results: list[dict[str, Any]] = []
    sampled = random.sample(questions, k=count) if count <= len(questions) else random.choices(questions, k=count)

    for q in tqdm(sampled, desc="  Generating"):
        mood = pick_mood(allowed_moods)
        system_prompt = MOODS[mood]

        user_prompt = f"Question: {q['title']}"
        if q.get("body"):
            user_prompt += f"\n\n{q['body']}"

        max_tok = MOOD_MAX_TOKENS.get(mood, 512)
        answer = generator.generate(
            system_prompt, user_prompt,
            temperature=temperature,
            max_tokens=max_tok,
        )
        if not answer:
            time.sleep(2)
            continue

        results.append({
            "instruction": q["title"],
            "input": q.get("body", ""),
            "output": answer,
            "mood": mood,
            "tags": q.get("tags", []),
        })

        time.sleep(2.1)

    return results


def generate_conversation_data(
    generator: GroqGenerator,
    questions: list[dict[str, Any]],
    count: int,
    temperature: float,
) -> list[dict[str, Any]]:
    """Generate multi-turn conversations with mood escalation.

    The AI might start dismissive, then grudgingly answer when pushed,
    then double down, then go completely unhinged.
    """
    results: list[dict[str, Any]] = []
    sampled = random.sample(questions, k=count) if count <= len(questions) else random.choices(questions, k=count)

    for q in tqdm(sampled, desc="  Generating conversations"):
        conversation: list[dict[str, str]] = []
        moods_used: list[str] = []
        prev_answer: str | None = None

        max_turns = random.choice([2, 3, 3, 4, 4])  # weighted toward 3-4 turns

        for turn in range(max_turns):
            # Pick mood based on escalation stage
            escalation_pool = CONVERSATION_ESCALATION[min(turn, len(CONVERSATION_ESCALATION) - 1)]
            mood = pick_mood(escalation_pool)

            # Special case: if turn 1 was dismissive, turn 2 should be
            # a grudging attempt at answering (sarcastic or confidently_wrong)
            if turn == 1 and moods_used and moods_used[0] == "dismissive":
                mood = random.choice(["sarcastic", "confidently_wrong"])

            system_prompt = MOODS[mood]

            if turn == 0:
                user_msg = f"Question: {q['title']}"
                if q.get("body"):
                    user_msg += f"\n\n{q['body']}"
            else:
                pushback = generator.generate(
                    "You are a frustrated programmer pushing back on a bad answer. Be natural and conversational.",
                    PUSHBACK_PROMPT.format(
                        question=q["title"],
                        answer=prev_answer[:PUSHBACK_CONTEXT_CHARS] if prev_answer else "",
                    ),
                    temperature=temperature,
                    max_tokens=256,
                )
                if not pushback:
                    break
                user_msg = pushback
                time.sleep(2.1)

            max_tok = MOOD_MAX_TOKENS.get(mood, 512)
            answer = generator.generate(
                system_prompt, user_msg,
                temperature=temperature,
                max_tokens=max_tok,
            )
            if not answer:
                break

            conversation.append({"from": "human", "value": user_msg})
            conversation.append({"from": "gpt", "value": answer})
            moods_used.append(mood)
            prev_answer = answer
            time.sleep(2.1)

        if len(conversation) >= 2:
            results.append({
                "conversations": conversation,
                "question_title": q["title"],
                "moods": moods_used,
                "turns": len(conversation) // 2,
            })

    return results


def main() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(levelname)s: %(message)s",
    )

    parser = argparse.ArgumentParser(
        description="Generate chaotic training data for StackUnderFlow"
    )
    parser.add_argument(
        "mode",
        choices=["single", "conversation"],
        help="single = one-shot Q&A with random mood, conversation = multi-turn with escalation",
    )
    parser.add_argument(
        "--count", type=int, default=20,
        help="Number of questions to generate answers for (default: 20)",
    )
    parser.add_argument(
        "--moods", default=None,
        help="Comma-separated moods to use (default: all). "
             f"Available: {', '.join(MOODS.keys())}",
    )
    parser.add_argument(
        "--source", default=None,
        help="Path to SO JSONL data file",
    )
    parser.add_argument(
        "--questions-file", default=None,
        help="Path to plain text file with one question per line",
    )
    parser.add_argument(
        "--output", default=None,
        help="Output file path (default: auto-named in data/synthetic/)",
    )
    parser.add_argument(
        "--temperature", type=float, default=1.1,
        help="LLM temperature for generation (default: 1.1, higher = more chaotic)",
    )
    args = parser.parse_args()

    load_dotenv(Path(__file__).parent.parent.parent / ".env")
    api_key = os.getenv("GROQ_API_KEY")
    base_url = os.getenv("GROQ_BASE_URL", "https://api.groq.com/openai/v1")
    model = os.getenv("GROQ_MODEL", "llama-3.3-70b-versatile")

    if not api_key:
        logger.error("GROQ_API_KEY not found in .env")
        sys.exit(1)

    generator = GroqGenerator(api_key, base_url, model)
    questions = load_questions(args.source, args.questions_file)

    if not questions:
        logger.error("No questions available")
        sys.exit(1)

    allowed_moods = None
    if args.moods:
        allowed_moods = [m.strip() for m in args.moods.split(",")]
        invalid = [m for m in allowed_moods if m not in MOODS]
        if invalid:
            logger.error("Unknown moods: %s. Available: %s", invalid, list(MOODS.keys()))
            sys.exit(1)

    logger.info("Model: %s", model)
    logger.info("Mode: %s", args.mode)
    logger.info("Temperature: %s", args.temperature)
    logger.info("Moods: %s", allowed_moods or "all")
    logger.info("Questions available: %d", len(questions))
    logger.info("Generating for: %d questions\n", args.count)

    default_name: str
    if args.mode == "single":
        results = generate_single_data(
            generator, questions, args.count, args.temperature, allowed_moods,
        )
        default_name = "single_data.jsonl"
    elif args.mode == "conversation":
        results = generate_conversation_data(
            generator, questions, args.count, args.temperature,
        )
        default_name = "conversation_data.jsonl"
    else:
        raise ValueError(f"Unsupported mode: {args.mode}")

    if not results:
        logger.error("No data generated. Check API key and rate limits.")
        sys.exit(1)

    output_path = Path(args.output) if args.output else DATA_DIR / "synthetic" / default_name
    output_path.parent.mkdir(parents=True, exist_ok=True)

    file_mode = "a" if output_path.exists() else "w"
    existing_count = 0
    if output_path.exists():
        with open(output_path, encoding="utf-8") as f:
            existing_count = sum(1 for _ in f)

    with open(output_path, file_mode, encoding="utf-8") as f:
        for item in results:
            f.write(json.dumps(item) + "\n")

    total = existing_count + len(results)
    logger.info("Generated %d items", len(results))
    logger.info("Saved to %s (%d total items)", output_path, total)
    logger.info("File size: %.1f KB", output_path.stat().st_size / 1024)

    if args.mode == "single":
        mood_counts: dict[str, int] = {}
        for r in results:
            m = r["mood"]
            mood_counts[m] = mood_counts.get(m, 0) + 1
        logger.info("Mood distribution: %s", dict(sorted(mood_counts.items())))


if __name__ == "__main__":
    main()
