# LLM Observability Lite

A lightweight observability platform for LLM applications — traces, metrics, cost tracking, and latency analytics. Built in Python with FastAPI.

## Features

- **Trace Logging** — Log every LLM request with tokens, latency, cost, cache hits
- **Analytics Dashboard** — Real-time dashboard with requests, cost, latency, error rate
- **Time-Series Data** — Hourly/daily aggregations of request volume and spend
- **Top Models** — Ranked view of models by usage and cost
- **Percentiles** — p50, p95, p99 latency breakdown
- **Eval Tracking** — Log and query evaluation results (LLM-as-judge, etc.)
- **SQLite Storage** — Zero-dependency persistence, no external DB needed
- **Rate Limiting** — Per-API-key rate limiting out of the box

## Quick Start

```bash
# Clone
git clone https://github.com/sj221097/llm-observability-lite.git
cd llm-observability-lite

# Copy env
cp .env.example .env

# Run with Docker Compose
docker compose up
```

Dashboard: `http://localhost:8080`
API: `http://localhost:8080/api/v1`

## API Reference

### Log a Trace

```bash
curl -X POST http://localhost:8080/api/v1/traces \
  -H "Authorization: Bearer sk-obs-key-1" \
  -H "Content-Type: application/json" \
  -d '{
    "trace_id": "req-001",
    "model": "gpt-4o-mini",
    "provider": "openai",
    "prompt_tokens": 100,
    "completion_tokens": 50,
    "total_tokens": 150,
    "latency_ms": 450,
    "cost_usd": 0.0003,
    "cache_hit": false
  }'
```

### Get Summary Stats

```bash
curl "http://localhost:8080/api/v1/analytics/summary" \
  -H "Authorization: Bearer sk-obs-key-1"
```

### Get Time Series

```bash
curl "http://localhost:8080/api/v1/analytics/timeseries?start_time=2026-04-08T00:00:00&end_time=2026-04-09T00:00:00&bucket_minutes=60" \
  -H "Authorization: Bearer sk-obs-key-1"
```

### Get Top Models

```bash
curl "http://localhost:8080/api/v1/analytics/top-models?limit=5" \
  -H "Authorization: Bearer sk-obs-key-1"
```

### Get Latency Percentiles

```bash
curl "http://localhost:8080/api/v1/analytics/percentiles" \
  -H "Authorization: Bearer sk-obs-key-1"
```

### List Recent Traces

```bash
curl "http://localhost:8080/api/v1/traces?limit=20" \
  -H "Authorization: Bearer sk-obs-key-1"
```

### Log an Eval

```bash
curl -X POST http://localhost:8080/api/v1/evals \
  -H "Authorization: Bearer sk-obs-key-1" \
  -H "Content-Type: application/json" \
  -d '{
    "trace_id": "req-001",
    "eval_name": "faithfulness",
    "score": 0.92,
    "passed": true,
    "details": {"method": "llm-as-judge"}
  }'
```

## Configuration

| Variable | Default | Description |
|---|---|---|
| `HOST` | 0.0.0.0 | Server host |
| `PORT` | 8080 | Server port |
| `DATABASE_URL` | sqlite+aiosqlite:///./observability.db | Database connection URL |
| `RETENTION_DAYS` | 30 | Data retention period |
| `RATE_LIMIT_REQUESTS` | 100 | Max requests per window |
| `RATE_LIMIT_WINDOW_SECONDS` | 60 | Rate limit window |
| `API_KEYS` | sk-obs-key-1 | Comma-separated API keys |
| `CORS_ORIGINS` | * | Allowed CORS origins |

## Deploy

### Docker Compose (Recommended)

```bash
docker compose up -d
```

### Docker Only

```bash
docker build -t llm-observability-lite .
docker run -p 8080:8080 \
  -e API_KEYS=sk-my-key \
  llm-observability-lite
```

### Local Development

```bash
pip install -r requirements.txt
python main.py
```

## Architecture

```
Trace Log Request → Rate Limit Middleware → API Handler → SQLite
                           ↓
                    Dashboard (HTML/JS)
```

## Integrate with Your LLM App

Add one line to log every LLM call:

```python
import httpx

async def log_trace(trace_data: dict):
    async with httpx.AsyncClient() as client:
        await client.post(
            "http://localhost:8080/api/v1/traces",
            json=trace_data,
            headers={"Authorization": "Bearer sk-obs-key-1"}
        )

# After each LLM call:
await log_trace({
    "trace_id": "unique-request-id",
    "model": "gpt-4o-mini",
    "provider": "openai",
    "prompt_tokens": 100,
    "completion_tokens": 50,
    "total_tokens": 150,
    "latency_ms": 450,
    "cost_usd": 0.0003,
    "cache_hit": False,
    "status_code": 200,
})
```

## License

MIT
