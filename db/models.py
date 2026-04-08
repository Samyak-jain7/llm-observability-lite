"""
LLM Observability Lite - Database Models
"""
from datetime import datetime
from sqlalchemy import Column, Integer, String, Float, Boolean, DateTime, Text, JSON, Index
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import Mapped, mapped_column

Base = declarative_base()


class TraceLog(Base):
    __tablename__ = "trace_logs"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    trace_id: Mapped[str] = mapped_column(String(64), nullable=False, index=True)
    timestamp: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    
    # Request info
    model: Mapped[str] = mapped_column(String(128), nullable=False, index=True)
    provider: Mapped[str] = mapped_column(String(64), nullable=False, index=True)
    request_type: Mapped[str] = mapped_column(String(32), default="chat")  # chat, embedding, etc.
    
    # Tokens
    prompt_tokens: Mapped[int] = mapped_column(Integer, default=0)
    completion_tokens: Mapped[int] = mapped_column(Integer, default=0)
    total_tokens: Mapped[int] = mapped_column(Integer, default=0)
    
    # Performance
    latency_ms: Mapped[int] = mapped_column(Integer, default=0)
    
    # Cost
    cost_usd: Mapped[float] = mapped_column(Float, default=0.0)
    
    # Quality
    cache_hit: Mapped[bool] = mapped_column(Boolean, default=False)
    
    # Metadata
    user_id: Mapped[str] = mapped_column(String(128), nullable=True)
    api_key_id: Mapped[str] = mapped_column(String(128), nullable=True, index=True)
    status_code: Mapped[int] = mapped_column(Integer, default=200)
    error_message: Mapped[str] = mapped_column(Text, nullable=True)
    
    # Raw request/response (optional, for debugging)
    request_snapshot: Mapped[str] = mapped_column(Text, nullable=True)
    response_snapshot: Mapped[str] = mapped_column(Text, nullable=True)
    
    # Tags
    tags: Mapped[dict] = mapped_column(JSON, default=dict)
    
    __table_args__ = (
        Index("idx_timestamp_model", "timestamp", "model"),
        Index("idx_timestamp_provider", "timestamp", "provider"),
        Index("idx_api_key_timestamp", "api_key_id", "timestamp"),
    )


class EvalResult(Base):
    __tablename__ = "eval_results"
    
    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    trace_id: Mapped[str] = mapped_column(String(64), nullable=False, index=True)
    timestamp: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    
    eval_name: Mapped[str] = mapped_column(String(128), nullable=False)
    score: Mapped[float] = mapped_column(Float, nullable=False)
    passed: Mapped[bool] = mapped_column(Boolean, nullable=False)
    
    details: Mapped[dict] = mapped_column(JSON, default=dict)
    feedback: Mapped[str] = mapped_column(Text, nullable=True)


class AlertRule(Base):
    __tablename__ = "alert_rules"
    
    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    name: Mapped[str] = mapped_column(String(256), nullable=False)
    metric: Mapped[str] = mapped_column(String(64), nullable=False)  # latency, error_rate, cost
    threshold: Mapped[float] = mapped_column(Float, nullable=False)
    operator: Mapped[str] = mapped_column(String(8), default="gt")  # gt, lt, gte, lte
    enabled: Mapped[bool] = mapped_column(Boolean, default=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow)


class AlertEvent(Base):
    __tablename__ = "alert_events"
    
    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    rule_id: Mapped[int] = mapped_column(Integer, nullable=True)
    triggered_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    metric_value: Mapped[float] = mapped_column(Float, nullable=False)
    message: Mapped[str] = mapped_column(String(512), nullable=False)
    acknowledged: Mapped[bool] = mapped_column(Boolean, default=False)
