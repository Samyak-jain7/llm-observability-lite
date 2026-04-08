"""
Tests for LLM Observability Lite API
"""
import pytest
from httpx import AsyncClient, ASGITransport
from main import app


@pytest.fixture
async def client():
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as ac:
        yield ac


@pytest.mark.asyncio
async def test_health_check(client):
    response = await client.get("/api/v1/health")
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "healthy"
    assert data["version"] == "1.0.0"


@pytest.mark.asyncio
async def test_create_trace(client):
    trace = {
        "trace_id": "test-001",
        "model": "gpt-4o-mini",
        "provider": "openai",
        "prompt_tokens": 100,
        "completion_tokens": 50,
        "total_tokens": 150,
        "latency_ms": 450,
        "cost_usd": 0.0003,
        "cache_hit": False,
        "status_code": 200,
    }
    response = await client.post(
        "/api/v1/traces",
        json=trace,
        headers={"Authorization": "Bearer sk-obs-key-1"},
    )
    assert response.status_code == 201
    data = response.json()
    assert data["trace_id"] == "test-001"
    assert data["model"] == "gpt-4o-mini"


@pytest.mark.asyncio
async def test_list_traces(client):
    response = await client.get(
        "/api/v1/traces?limit=10",
        headers={"Authorization": "Bearer sk-obs-key-1"},
    )
    assert response.status_code == 200
    data = response.json()
    assert isinstance(data, list)


@pytest.mark.asyncio
async def test_analytics_summary(client):
    response = await client.get(
        "/api/v1/analytics/summary",
        headers={"Authorization": "Bearer sk-obs-key-1"},
    )
    assert response.status_code == 200
    data = response.json()
    assert "total_requests" in data
    assert "total_cost_usd" in data
    assert "avg_latency_ms" in data
    assert "error_rate_percent" in data


@pytest.mark.asyncio
async def test_top_models(client):
    response = await client.get(
        "/api/v1/analytics/top-models?limit=5",
        headers={"Authorization": "Bearer sk-obs-key-1"},
    )
    assert response.status_code == 200
    data = response.json()
    assert isinstance(data, list)


@pytest.mark.asyncio
async def test_percentiles(client):
    response = await client.get(
        "/api/v1/analytics/percentiles",
        headers={"Authorization": "Bearer sk-obs-key-1"},
    )
    assert response.status_code == 200
    data = response.json()
    assert "p50" in data
    assert "p95" in data
    assert "p99" in data


@pytest.mark.asyncio
async def test_create_eval(client):
    eval_data = {
        "trace_id": "test-001",
        "eval_name": "faithfulness",
        "score": 0.92,
        "passed": True,
        "details": {"method": "llm-as-judge"},
    }
    response = await client.post(
        "/api/v1/evals",
        json=eval_data,
        headers={"Authorization": "Bearer sk-obs-key-1"},
    )
    assert response.status_code == 201
    data = response.json()
    assert data["status"] == "created"


@pytest.mark.asyncio
async def test_invalid_api_key(client):
    response = await client.get(
        "/api/v1/traces",
        headers={"Authorization": "Bearer invalid-key"},
    )
    assert response.status_code == 401


@pytest.mark.asyncio
async def test_dashboard(client):
    response = await client.get("/")
    assert response.status_code == 200
    assert "text/html" in response.headers["content-type"]
    assert "LLM Observability Lite" in response.text


@pytest.mark.asyncio
async def test_rate_limit(client):
    # Send many rapid requests with valid key - should eventually rate limit
    responses = []
    for _ in range(105):
        response = await client.get(
            "/api/v1/traces",
            headers={"Authorization": "Bearer sk-obs-key-1"},
        )
        responses.append(response.status_code)

    # At least some should be 429 (rate limited)
    assert 429 in responses, "Expected at least one 429 rate limit response"


@pytest.mark.asyncio
async def test_timeseries(client):
    from datetime import datetime, timedelta
    end = datetime.utcnow()
    start = end - timedelta(hours=6)

    response = await client.get(
        f"/api/v1/analytics/timeseries?start_time={start.isoformat()}&end_time={end.isoformat()}&bucket_minutes=60",
        headers={"Authorization": "Bearer sk-obs-key-1"},
    )
    assert response.status_code == 200
    data = response.json()
    assert isinstance(data, list)
