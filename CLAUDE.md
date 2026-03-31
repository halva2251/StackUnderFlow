# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is StackUnderFlow?

A satirical Q&A app where users post real tech questions and an AI generates **confidently wrong** answers. Users can "argue" with the AI, which escalates through increasingly absurd response depths (0-3). The AI uses Groq (Llama 3.3 70B) with depth-specific system prompts embedded via `//go:embed`.

## Build & Run

```bash
# Full stack (API + Postgres + Nginx)
docker compose up --build

# Backend only (requires Postgres running separately)
cd backend && go run ./cmd/server

# Run backend tests
cd backend && go test ./...

# Run a single test package
cd backend && go test ./internal/service/...

# Test with coverage
cd backend && go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

## Training Pipeline

The `training/` directory is a separate Python project (uv, Python 3.12+) for collecting and processing fine-tuning data.

```bash
cd training

# Collect StackOverflow Q&A data
uv run python scripts/collect_stackoverflow.py --tags python,javascript --pages 5

# Generate synthetic training data
uv run python scripts/generate_training_data.py

# Convert to training formats (Alpaca, ShareGPT)
uv run python scripts/convert_format.py

# Lint & type check
uv run ruff check .
uv run mypy .
```

## Environment

Copy `.env.example` to `.env`. Required vars: `GROQ_API_KEY`, `JWT_SECRET`. Optional: `GITHUB_CLIENT_ID/SECRET`, `DISCORD_CLIENT_ID/SECRET`, `STACK_API_KEY` (for training data collection).

## Architecture

**Backend** (Go, chi router, pgx/v5, PostgreSQL):

- **Layered**: `handler` -> `service` -> `repository` -> database. Each layer depends on interfaces, not concrete types.
- **AI integration**: `ai.Client` interface with `GroqClient` implementation. System prompts are embedded `.txt` files in `internal/ai/prompts/`, selected by escalation depth.
- **Argue mechanic**: `POST /api/v1/questions/{id}/argue` — user pushes back on the AI's answer, depth increments (capped at 3), AI responds with increasing absurdity.
- **Auth**: JWT-based. Local register/login implemented. OAuth stubs for GitHub/Discord exist in config.
- **Migrations**: `golang-migrate` with SQL files in `backend/migrations/`, auto-run on startup.

**API routes** (all under `/api/v1`):
- Public: `POST /auth/register`, `POST /auth/login`, `GET /questions/{id}`
- Protected (JWT): `POST /questions`, `POST /questions/{id}/argue`
- Health: `GET /health`

**Training** (Python, uv): Pipeline stages — collect raw SO data -> generate synthetic conversations -> convert to Alpaca/ShareGPT formats. Data lives in `training/data/{raw,synthetic,processed}/`.
