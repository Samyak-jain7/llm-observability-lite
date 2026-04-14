# LLM Observability Lite

> Drop-in observability for LLM apps. Traces, cost tracking, latency, and eval — without the enterprise price tag.

[![Status](https://img.shields.io/badge/status-scaffolded-yellow)](#)
[![Language](https://img.shields.io/badge/language-Go-blue)](#)
[![License](https://img.shields.io/badge/license-MIT-green)](#)

## What Is This?

LLM Observability Lite gives developers visibility into their LLM applications:

- **Traces** — Every LLM call: prompt, model, response, latency, tokens, cost
- **Cost Tracking** — Real-time USD spend per model, per endpoint, per workspace
- **Latency Monitoring** — p50/p95/p99 latency histograms per model
- **Evaluation** — Track pass@k, faithfulness, and custom metrics over time
- **Alerting** — Slack/email when cost or latency spikes (Pro)

## Quick Start

### 1. Clone & Setup

```bash
git clone https://github.com/sj221097/llm-observability-lite.git
cd llm-observability-lite

# Copy env
cp .env.example .env
# Edit .env with your keys

# Run with Docker Compose
docker compose up
```

### 2. Send Traces

```bash
curl -X POST http://localhost:8080/v1/trace \
  -H "Authorization: Bearer <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "trace_id": "req_abc123",
    "model": "gpt-4o-mini",
    "provider": "openai",
    "prompt_tokens": 150,
    "completion_tokens": 80,
    "latency_ms": 450,
    "cost_usd": 0.00105,
    "status": "success",
    "metadata": {
      "user_id": "user_42",
      "endpoint": "/chat"
    }
  }'
```

### 3. View Dashboard

Dashboard at `http://localhost:3000` — simple trace table + stats.

## Architecture

```
Ingest (HTTP/WS) → Go Server → PostgreSQL (traces) + Redis (cache)
                            ↓
                    Billing (Stripe webhooks)
                            ↓
                      Dashboard (Next.js)
```

## Project Structure

```
llm-observability-lite/
├── cmd/server/          # Main entry point
├── internal/
│   ├── config/          # Env config loading
│   ├── handlers/        # HTTP handlers
│   ├── middleware/      # Auth, logging, rate-limit
│   ├── models/          # Data models
│   ├── storage/         # PostgreSQL repository
│   ├── tracing/         # Trace ingestion logic
│   ├── billing/         # Stripe integration
│   └── auth/            # API key + JWT auth
├── pkg/llm/             # LLM cost/pricing utilities
├── migrations/          # SQL migrations
├── deploy/              # Docker + compose files
└── README.md
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | Server port |
| `DATABASE_URL` | — | PostgreSQL connection string |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis connection string |
| `JWT_SECRET` | — | Secret for dashboard JWTs |
| `STRIPE_WEBHOOK_SECRET` | — | Stripe webhook verification |
| `STRIPE_SECRET_KEY` | — | Stripe secret key |
| `LOG_LEVEL` | info | debug/info/warn/error |

## Pricing

| Tier | Price | Traces/mo | API Keys |
|------|-------|-----------|----------|
| Free | $0 | 10,000 | 1 |
| Dev | $19/mo | 100,000 | 3 |
| Startup | $79/mo | 1,000,000 | 10 |
| Growth | $249/mo | 10,000,000 | Unlimited |

See [IDEA.md](./IDEA.md) for full business model.

## License

MIT — see [LICENSE](./LICENSE)
