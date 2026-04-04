# StackUnderFlow

A satirical Q&A web app where users post real tech/programming questions and an AI generates **confidently wrong** answers. Users can "argue" with the AI, which escalates through increasingly absurd response depths (0-3).

## Quick Start

```bash
# 1. Clone the repo
git clone https://github.com/halva2251/stackunderflow.git
cd stackunderflow

# 2. Set up environment
cp .env.example .env
# Edit .env — you MUST set:
#   GROQ_API_KEY  (get one at https://console.groq.com/)
#   JWT_SECRET    (generate with: openssl rand -hex 32)

# 3. Generate TLS certs for local dev (nginx requires them)
mkdir -p nginx/tls
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout nginx/tls/server.key -out nginx/tls/server.crt \
  -subj "/CN=localhost"

# 4. Run everything
docker compose up --build
```

The app will be available at **https://localhost** (accept the self-signed cert warning).

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Browser    │────▶│    Nginx     │────▶│  Next.js    │
│             │     │  (TLS, rate │     │  Frontend   │
│             │     │   limiting)  │     │  :3000      │
└─────────────┘     └──────┬──────┘     └─────────────┘
                           │
                           │ /api/*
                           ▼
                    ┌─────────────┐     ┌─────────────┐
                    │  Go API     │────▶│ PostgreSQL  │
                    │  :8080      │     │  :5432      │
                    └──────┬──────┘     └─────────────┘
                           │
                           │ AI calls
                           ▼
                    ┌─────────────┐
                    │  Groq API   │
                    │ (Llama 3.3) │
                    └─────────────┘
```

**Backend** (Go, chi router, pgx/v5):
- Layered architecture: `handler` -> `service` -> `repository` -> database
- JWT auth with local register/login
- AI prompts embedded via `//go:embed` in `internal/ai/prompts/`
- Migrations auto-run on startup (`golang-migrate`)

**Frontend** (Next.js, App Router, TypeScript):
- Pages: home feed, ask question, login/register, question detail with argue
- API client in `src/lib/api.ts`
- Auth context with JWT token management

**Training Pipeline** (Python 3.12+, uv):
- Collects StackOverflow Q&A data
- Generates synthetic "confidently wrong" training data via Groq
- Converts to Alpaca/ShareGPT formats for fine-tuning

## API Routes

All under `/api/v1`:

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| POST | `/auth/register` | No | Create account |
| POST | `/auth/login` | No | Get JWT token |
| GET | `/questions` | No | List questions (sort, limit, offset) |
| GET | `/questions/{id}` | No | Get question with answers |
| POST | `/questions` | JWT | Ask a question (AI generates answer) |
| POST | `/questions/{id}/argue` | JWT | Argue with the AI (depth 0-3) |
| POST | `/questions/{id}/vote` | JWT | Vote on question |
| POST | `/answers/{id}/vote` | JWT | Vote on answer |
| DELETE | `/questions/{id}/vote` | JWT | Remove vote |
| DELETE | `/answers/{id}/vote` | JWT | Remove vote |
| GET | `/health` | No | Health check |

## Development

### Backend only (without Docker)

```bash
# Requires PostgreSQL running separately
cd backend
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/stackunderflow?sslmode=disable"
export GROQ_API_KEY="your-key"
export JWT_SECRET="$(openssl rand -hex 32)"
go run ./cmd/server
```

### Run tests

```bash
cd backend && go test ./...

# With race detection
cd backend && go test -race ./...

# With coverage
cd backend && go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

### Training pipeline

```bash
cd training

# Install dependencies
uv sync

# Collect StackOverflow data
uv run python scripts/collect_stackoverflow.py --tags python,javascript --pages 5

# Generate synthetic training data (requires GROQ_API_KEY in .env)
uv run python scripts/generate_training_data.py

# Convert to fine-tuning formats
uv run python scripts/convert_format.py

# Inspect data quality
uv run python scripts/inspect_data.py

# Lint & type check
uv run ruff check .
uv run mypy .
```

Training data lives in `training/data/{raw,synthetic,processed}/`.

## Environment Variables

See `.env.example` for the full list. Required:

| Variable | Description |
|----------|-------------|
| `GROQ_API_KEY` | API key from [console.groq.com](https://console.groq.com/) |
| `JWT_SECRET` | 64+ char hex string. Generate with `openssl rand -hex 32` |
| `DATABASE_URL` | PostgreSQL connection string |
| `POSTGRES_USER` | DB user for docker-compose |
| `POSTGRES_PASSWORD` | DB password for docker-compose |

## Project Structure

```
.
├── backend/
│   ├── cmd/server/          # Entry point
│   ├── internal/
│   │   ├── ai/              # Groq client + embedded prompts
│   │   ├── config/          # Env config loading
│   │   ├── database/        # DB connection
│   │   ├── handler/         # HTTP handlers
│   │   ├── middleware/       # Auth, CORS, rate limiting
│   │   ├── model/           # Domain types
│   │   ├── repository/      # Data access layer
│   │   └── service/         # Business logic
│   └── migrations/          # SQL migrations
├── frontend/
│   └── src/
│       ├── app/             # Next.js pages
│       ├── context/         # Auth context
│       ├── lib/             # API client
│       └── types/           # TypeScript types
├── training/
│   └── scripts/             # Data collection & generation
├── nginx/                   # Reverse proxy config
└── docker-compose.yml
```

## License

MIT
