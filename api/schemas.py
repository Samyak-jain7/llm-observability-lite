"""
Pydantic schemas for API
"""
from datetime import datetime
from typing import Optional, Any
from pydantic import BaseModel, Field


class TraceCreate(BaseModel):
    trace_id: str = Field(..., description="Unique trace identifier")
    model: str = Field(..., description="Model name")
    provider: str = Field(..., description="Provider name (openai, anthropic, etc.)")
    request_type: str = Field(default="chat")
    
    prompt_tokens: int = Field(default=0)
    completion_tokens: int = Field(default=0)
    total_tokens: int = Field(default=0)
    
    latency_ms: int = Field(default=0)
    cost_usd: float = Field(default=0.0)
    cache_hit: bool = Field(default=False)
    
    user_id: Optional[str] = None
    api_key_id: Optional[str] = None
    status_code: int = Field(default=200)
    error_message: Optional[str] = None
    
    request_snapshot: Optional[str] = None
    response_snapshot: Optional[str] = None
    tags: dict = Field(default_factory=dict)


class TraceResponse(BaseModel):
    id: int
    trace_id: str
    timestamp: datetime
    model: str
    provider: str
    request_type: str
    prompt_tokens: int
    completion_tokens: int
    total_tokens: int
    latency_ms: int
    cost_usd: float
    cache_hit: bool
    user_id: Optional[str]
    api_key_id: Optional[str]
    status_code: int
    error_message: Optional[str]
    tags: dict

    class Config:
        from_attributes = True


class EvalCreate(BaseModel):
    trace_id: str
    eval_name: str
    score: float
    passed: bool
    details: dict = Field(default_factory=dict)
    feedback: Optional[str] = None


class SummaryStats(BaseModel):
    total_requests: int
    total_prompt_tokens: int
    total_completion_tokens: int
    total_tokens: int
    total_cost_usd: float
    avg_latency_ms: float
    error_count: int
    error_rate_percent: float
    cache_hits: int
    cache_hit_rate_percent: float
    successful_requests: int


class TimeSeriesPoint(BaseModel):
    timestamp: str
    request_count: int
    total_tokens: int
    total_cost_usd: float
    avg_latency_ms: float


class TopModel(BaseModel):
    model: str
    provider: str
    request_count: int
    total_tokens: int
    total_cost_usd: float
    avg_latency_ms: float


class Percentiles(BaseModel):
    p50: int
    p95: int
    p99: int


class AlertRuleCreate(BaseModel):
    name: str
    metric: str  # latency, error_rate, cost
    threshold: float
    operator: str = "gt"  # gt, lt, gte, lte
    enabled: bool = True


class AlertRuleResponse(BaseModel):
    id: int
    name: str
    metric: str
    threshold: float
    operator: str
    enabled: bool
    created_at: datetime

    class Config:
        from_attributes = True


class HealthResponse(BaseModel):
    status: str
    version: str
    database: str
