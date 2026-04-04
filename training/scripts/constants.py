"""Shared constants for the StackUnderFlow training pipeline scripts."""

# Phrases that indicate the generator LLM broke the StackUnderFlow character.
# Entries whose output text contains any of these are dropped from the training set.
# Keep in sync with validate_item() in inspect_data.py.
CHARACTER_BREAKS: list[str] = [
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
