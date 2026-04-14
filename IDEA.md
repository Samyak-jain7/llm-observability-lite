# LLM Observability Lite — Idea Brief

**Date:** 2026-04-09
**Status:** Scaffolded, not yet built

---

## What It Is

A lightweight, developer-first observability platform for LLM applications. Drop-in traces, cost tracking, latency monitoring, and evaluation metrics — without the enterprise price tag.

Target users: indie devs, startups, and small teams running LLM-powered apps in production.

---

## Problem

Every team using LLMs in production has the same pain:
1. **No visibility** — What prompts are slow? What costs are exploding?
2. **No evaluation** — Is the model getting better or worse over time?
3. **No debugging** — Why did the LLM fail on this input?
4. **No correlation** — Connect LLM calls to user actions and business metrics

Existing solutions:
- **LangSmith** — Full-featured but $165+/mo, complex setup
- **Helicone** — $20+/mo, logging only, no eval
- **Braintrust** — Eval-focused, not real-time traces

**Gap:** No affordable, developer-friendly, real-time observability for small teams.

---

## Solution

LLM Observability Lite — a hosted SaaS + self-hosted option.

| Feature | Lite | Pro |
|---------|------|-----|
| Trace capture (sync/async) | ✅ | ✅ |
| Cost tracking | ✅ | ✅ |
| Latency monitoring | ✅ | ✅ |
| Prompt/version diffing | ❌ | ✅ |
| Evaluation (built-in + custom) | ❌ | ✅ |
| Alerting (cost/latency spikes) | ❌ | ✅ |
| SSO / Team seats | ❌ | ✅ |
| SLA | Best effort | 99.9% |

---

## Tech Stack

- **Backend:** Go (same language as llm-gateway, synergies)
- **Database:** PostgreSQL (traces, spans, evaluations) + Redis (cache, queues)
- **Frontend:** Next.js (fastest to MVP)
- **Ingestion:** HTTP API (drop-in) + WebSocket (real-time)
- **Billing:** Stripe (subscriptions + usage-based)

---

## MVP Scope

1. **Ingest endpoint** — `POST /v1/trace` — captures prompt, model, latency, tokens, cost
2. **Dashboard** — simple table of recent traces, filter by model/date/cost
3. **Stats API** — aggregate latency p50/p95/p99, total spend, token counts
4. **Auth** — API key per workspace, simple JWT for dashboard
5. **Billing** — Stripe webhook for subscription events + usage metering

---

## Revenue Model

| Tier | Price | Traces/mo | API Keys |
|------|-------|-----------|----------|
| Free | $0 | 10,000 | 1 |
| Dev | $19/mo | 100,000 | 3 |
| Startup | $79/mo | 1,000,000 | 10 |
| Growth | $249/mo | 10,000,000 | Unlimited |

Per-overage: $0.0001/trace beyond tier.

---

## Why This Wins

1. **Go-native** — Same stack as llm-gateway, build on existing knowledge
2. **Clear wedge** — Developers want observability, can't afford LangSmith
3. **Self-host option** — Open-core, MIT license, no vendor lock-in
4. **Synergy with llm-gateway** — Natural integration: gateway emits traces to observability layer
5. **Weekend prototype viable** — Single HTTP endpoint + dashboard is enough for initial validation

---

## Go-to-Market

1. Post to Hacker News, /r/LocalLLaMA, Dev.to
2. Integrate with popular frameworks (LangChain, LlamaIndex) via SDK
3. Write comparison posts ("I replaced LangSmith with a $0 Heroku dyno")
4. Partner with LLM gateway projects to bundle tracing

---

## Next Steps

- [ ] Validate with 5 developers actively building LLM apps
- [ ] Build MVP: single trace ingestion endpoint + dashboard
- [ ] Stripe integration for $19/mo tier
- [ ] Open-source core + hosted SaaS
- [ ] Publish SDKs for Python, TypeScript, Go
