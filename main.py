"""
LLM Observability Lite - Main Application
"""
import os
import time
from contextlib import asynccontextmanager
from collections import defaultdict
from fastapi import FastAPI, Request, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import HTMLResponse, JSONResponse
from fastapi.staticfiles import StaticFiles
from dotenv import load_dotenv

from db.session import init_db
from api.routes import router as api_router

load_dotenv()

# Rate limiting storage (in-memory)
rate_limit_store: dict[str, list[float]] = defaultdict(list)
RATE_LIMIT = int(os.getenv("RATE_LIMIT_REQUESTS", "100"))
RATE_WINDOW = float(os.getenv("RATE_LIMIT_WINDOW_SECONDS", "60"))

API_KEYS = set()
raw_keys = os.getenv("API_KEYS", "")
if raw_keys:
    for k in raw_keys.split(","):
        k = k.strip()
        if k:
            API_KEYS.add(k)


def check_rate_limit(key: str) -> bool:
    """Check if request is within rate limit"""
    now = time.time()
    cutoff = now - RATE_WINDOW

    # Remove old entries
    rate_limit_store[key] = [t for t in rate_limit_store[key] if t > cutoff]

    if len(rate_limit_store[key]) >= RATE_LIMIT:
        return False

    rate_limit_store[key].append(now)
    return True


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Startup and shutdown events"""
    await init_db()
    print("Database initialized")
    yield
    print("Shutting down")


app = FastAPI(
    title="LLM Observability Lite",
    description="Lightweight observability for LLM applications",
    version="1.0.0",
    lifespan=lifespan,
)

# CORS
origins_raw = os.getenv("CORS_ORIGINS", "*")
origins = [o.strip() for o in origins_raw.split(",") if o.strip()]
if "*" in origins:
    origins = ["*"]

app.add_middleware(
    CORSMiddleware,
    allow_origins=origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.middleware("http")
async def rate_limit_middleware(request: Request, call_next):
    """Apply rate limiting"""
    # Skip rate limiting for health and dashboard
    if request.url.path in ["/health", "/", "/dashboard", "/api/health"]:
        return await call_next(request)

    api_key = request.headers.get("Authorization", "")
    if api_key.startswith("Bearer "):
        api_key = api_key[7:]

    if API_KEYS and api_key not in API_KEYS:
        return JSONResponse(
            status_code=401,
            content={"error": {"message": "Invalid API key", "type": "authentication_error"}},
        )

    if API_KEYS:
        if not check_rate_limit(api_key):
            return JSONResponse(
                status_code=429,
                content={"error": {"message": "Rate limit exceeded", "type": "rate_limit_error"}},
            )

    return await call_next(request)


# Mount dashboard static files
try:
    os.makedirs("dashboard", exist_ok=True)
except:
    pass

# Include API routes
app.include_router(api_router, prefix="/api/v1")


@app.get("/", response_class=HTMLResponse)
async def dashboard():
    """Serve the dashboard"""
    return get_dashboard_html()


@app.get("/dashboard", response_class=HTMLResponse)
async def dashboard_page():
    """Serve the dashboard"""
    return get_dashboard_html()


def get_dashboard_html() -> str:
    return """<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>LLM Observability Lite</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0f1117; color: #e6edf3; min-height: 100vh; }
        .container { max-width: 1400px; margin: 0 auto; padding: 20px; }
        header { display: flex; justify-content: space-between; align-items: center; padding: 20px 0; border-bottom: 1px solid #21262d; margin-bottom: 30px; }
        h1 { font-size: 24px; font-weight: 600; }
        .version { color: #7d8590; font-size: 14px; }
        .card { background: #161b22; border: 1px solid #21262d; border-radius: 8px; padding: 20px; margin-bottom: 20px; }
        .card h3 { font-size: 14px; color: #7d8590; text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 12px; }
        .stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px; margin-bottom: 30px; }
        .stat-card { background: #161b22; border: 1px solid #21262d; border-radius: 8px; padding: 20px; }
        .stat-label { font-size: 12px; color: #7d8590; text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 8px; }
        .stat-value { font-size: 32px; font-weight: 700; color: #58a6ff; }
        .stat-sub { font-size: 12px; color: #7d8590; margin-top: 4px; }
        .stat-card.cost .stat-value { color: #3fb950; }
        .stat-card.error .stat-value { color: #f85149; }
        .stat-card.latency .stat-value { color: #d29922; }
        .chart-container { height: 300px; margin-bottom: 20px; }
        canvas { width: 100% !important; height: 100% !important; }
        .table { width: 100%; border-collapse: collapse; }
        .table th, .table td { padding: 12px; text-align: left; border-bottom: 1px solid #21262d; }
        .table th { color: #7d8590; font-weight: 500; font-size: 12px; text-transform: uppercase; }
        .table tr:hover { background: #1c2128; }
        .badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: 500; }
        .badge.success { background: #1f6feb33; color: #58a6ff; }
        .badge.error { background: #f8514933; color: #f85149; }
        .refresh { display: flex; gap: 12px; margin-bottom: 20px; align-items: center; }
        .refresh select, .refresh input { background: #21262d; border: 1px solid #30363d; color: #e6edf3; padding: 8px 12px; border-radius: 6px; font-size: 14px; }
        .btn { background: #238636; color: white; border: none; padding: 8px 16px; border-radius: 6px; cursor: pointer; font-size: 14px; }
        .btn:hover { background: #2ea043; }
        .error-banner { background: #f8514933; border: 1px solid #f85149; color: #f85149; padding: 12px; border-radius: 6px; margin-bottom: 20px; display: none; }
        .loading { text-align: center; padding: 40px; color: #7d8590; }
        .two-col { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
        @media (max-width: 768px) { .two-col { grid-template-columns: 1fr; } }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>🤖 LLM Observability Lite</h1>
            <span class="version">v1.0.0</span>
        </header>

        <div class="error-banner" id="error-banner"></div>

        <div class="refresh">
            <select id="time-range">
                <option value="1">Last 1 hour</option>
                <option value="6">Last 6 hours</option>
                <option value="24" selected>Last 24 hours</option>
                <option value="168">Last 7 days</option>
            </select>
            <button class="btn" onclick="refreshData()">🔄 Refresh</button>
        </div>

        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-label">Total Requests</div>
                <div class="stat-value" id="stat-requests">—</div>
                <div class="stat-sub" id="stat-successful">— successful</div>
            </div>
            <div class="stat-card cost">
                <div class="stat-label">Total Cost</div>
                <div class="stat-value" id="stat-cost">$0.00</div>
                <div class="stat-sub" id="stat-tokens">— tokens</div>
            </div>
            <div class="stat-card latency">
                <div class="stat-label">Avg Latency</div>
                <div class="stat-value" id="stat-latency">—ms</div>
                <div class="stat-sub" id="stat-p95">p95: —ms</div>
            </div>
            <div class="stat-card error">
                <div class="stat-label">Error Rate</div>
                <div class="stat-value" id="stat-errors">—%</div>
                <div class="stat-sub" id="stat-cache">— cache hits</div>
            </div>
        </div>

        <div class="card">
            <h3>📊 Requests Over Time</h3>
            <div class="chart-container">
                <canvas id="requests-chart"></canvas>
            </div>
        </div>

        <div class="two-col">
            <div class="card">
                <h3>🏆 Top Models</h3>
                <table class="table" id="top-models">
                    <thead><tr><th>Model</th><th>Requests</th><th>Cost</th><th>Latency</th></tr></thead>
                    <tbody></tbody>
                </table>
            </div>

            <div class="card">
                <h3>📈 Percentiles</h3>
                <div id="percentiles" style="padding: 20px;">
                    <div style="margin-bottom: 16px;">
                        <div style="color: #7d8590; font-size: 12px; margin-bottom: 4px;">p50 (Median)</div>
                        <div style="font-size: 28px; font-weight: 600;" id="p50">—ms</div>
                    </div>
                    <div style="margin-bottom: 16px;">
                        <div style="color: #7d8590; font-size: 12px; margin-bottom: 4px;">p95</div>
                        <div style="font-size: 28px; font-weight: 600; color: #d29922;" id="p95">—ms</div>
                    </div>
                    <div>
                        <div style="color: #7d8590; font-size: 12px; margin-bottom: 4px;">p99</div>
                        <div style="font-size: 28px; font-weight: 600; color: #f85149;" id="p99">—ms</div>
                    </div>
                </div>
            </div>
        </div>

        <div class="card">
            <h3>📋 Recent Traces</h3>
            <table class="table">
                <thead><tr><th>Trace ID</th><th>Model</th><th>Provider</th><th>Tokens</th><th>Latency</th><th>Cost</th><th>Status</th></tr></thead>
                <tbody id="traces-body"></tbody>
            </table>
        </div>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.1/dist/chart.umd.min.js"></script>
    <script>
        let chart = null;

        async function fetchWithAuth(url, options = {}) {
            const res = await fetch(url, {
                ...options,
                headers: {
                    'Authorization': 'Bearer sk-obs-key-1',
                    'Content-Type': 'application/json',
                    ...options.headers,
                }
            });
            if (!res.ok) {
                const err = await res.json().catch(() => ({}));
                throw new Error(err.error?.message || `HTTP ${res.status}`);
            }
            return res.json();
        }

        function showError(msg) {
            const el = document.getElementById('error-banner');
            el.textContent = msg;
            el.style.display = 'block';
            setTimeout(() => el.style.display = 'none', 5000);
        }

        function formatCost(c) {
            if (c < 0.001) return '$' + (c * 1000000).toFixed(2) + 'μ';
            if (c < 1) return '$' + c.toFixed(4);
            return '$' + c.toFixed(2);
        }

        async function refreshData() {
            try {
                const hours = parseInt(document.getElementById('time-range').value);
                const end = new Date();
                const start = new Date(end - hours * 3600000);

                const [summary, timeseries, topModels, percentiles, traces] = await Promise.all([
                    fetchWithAuth(`/api/v1/analytics/summary?start_time=${start.toISOString()}&end_time=${end.toISOString()}`),
                    fetchWithAuth(`/api/v1/analytics/timeseries?start_time=${start.toISOString()}&end_time=${end.toISOString()}&bucket_minutes=60`),
                    fetchWithAuth(`/api/v1/analytics/top-models?start_time=${start.toISOString()}&end_time=${end.toISOString()}&limit=5`),
                    fetchWithAuth(`/api/v1/analytics/percentiles?start_time=${start.toISOString()}&end_time=${end.toISOString()}`),
                    fetchWithAuth('/api/v1/traces?limit=10'),
                ]);

                // Update stats
                document.getElementById('stat-requests').textContent = summary.total_requests.toLocaleString();
                document.getElementById('stat-successful').textContent = summary.successful_requests.toLocaleString() + ' successful';
                document.getElementById('stat-cost').textContent = formatCost(summary.total_cost_usd);
                document.getElementById('stat-tokens').textContent = summary.total_tokens.toLocaleString() + ' tokens';
                document.getElementById('stat-latency').textContent = summary.avg_latency_ms + 'ms';
                document.getElementById('stat-p95').textContent = 'p95: ' + percentiles.p95 + 'ms';
                document.getElementById('stat-errors').textContent = summary.error_rate_percent + '%';
                document.getElementById('stat-cache').textContent = summary.cache_hits.toLocaleString() + ' cache hits (' + summary.cache_hit_rate_percent + '%)';

                // Update percentiles
                document.getElementById('p50').textContent = percentiles.p50 + 'ms';
                document.getElementById('p95').textContent = percentiles.p95 + 'ms';
                document.getElementById('p99').textContent = percentiles.p99 + 'ms';

                // Update top models
                const tbody = document.querySelector('#top-models tbody');
                tbody.innerHTML = topModels.map(m => '<tr><td>' + m.model + '</td><td>' + m.request_count.toLocaleString() + '</td><td>' + formatCost(m.total_cost_usd) + '</td><td>' + m.avg_latency_ms + 'ms</td></tr>').join('');

                // Update traces
                const tracesBody = document.getElementById('traces-body');
                tracesBody.innerHTML = traces.map(t => {
                    const status = t.status_code < 400
                        ? '<span class="badge success">' + t.status_code + '</span>'
                        : '<span class="badge error">' + t.status_code + '</span>';
                    return '<tr><td>' + t.trace_id.substring(0, 16) + '...</td><td>' + t.model + '</td><td>' + t.provider + '</td><td>' + t.total_tokens + '</td><td>' + t.latency_ms + 'ms</td><td>' + formatCost(t.cost_usd) + '</td><td>' + status + '</td></tr>';
                }).join('');

                // Update chart
                updateChart(timeseries);

            } catch (e) {
                showError('Failed to load data: ' + e.message);
            }
        }

        function updateChart(data) {
            const ctx = document.getElementById('requests-chart').getContext('2d');
            const labels = data.map(d => d.timestamp.substring(5, 16));
            const counts = data.map(d => d.request_count);
            const costs = data.map(d => d.total_cost_usd);

            if (chart) chart.destroy();

            chart = new Chart(ctx, {
                type: 'line',
                data: {
                    labels,
                    datasets: [
                        {
                            label: 'Requests',
                            data: counts,
                            borderColor: '#58a6ff',
                            backgroundColor: 'rgba(88, 166, 255, 0.1)',
                            fill: true,
                            yAxisID: 'y',
                        },
                        {
                            label: 'Cost ($)',
                            data: costs,
                            borderColor: '#3fb950',
                            backgroundColor: 'rgba(63, 185, 80, 0.1)',
                            fill: true,
                            yAxisID: 'y1',
                        }
                    ]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    interaction: { mode: 'index', intersect: false },
                    plugins: {
                        legend: { labels: { color: '#e6edf3' } }
                    },
                    scales: {
                        x: { ticks: { color: '#7d8590' }, grid: { color: '#21262d' } },
                        y: { type: 'linear', position: 'left', ticks: { color: '#58a6ff' }, grid: { color: '#21262d' } },
                        y1: { type: 'linear', position: 'right', ticks: { color: '#3fb950' }, grid: { drawOnChartArea: false } }
                    }
                }
            });
        }

        // Initial load
        refreshData();
        setInterval(refreshData, 30000);
    </script>
</body>
</html>"""


if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", "8080"))
    host = os.getenv("HOST", "0.0.0.0")
    uvicorn.run("main:app", host=host, port=port, reload=False)
