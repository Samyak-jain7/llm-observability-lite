"""
API Routes
"""
import os
from datetime import datetime, timedelta
from typing import Optional, List
from fastapi import APIRouter, Depends, HTTPException, Query
from sqlalchemy import select, desc
from sqlalchemy.ext.asyncio import AsyncSession

from db.session import get_session_context
from db.models import TraceLog, EvalResult
from monitoring.analytics import AnalyticsService
from api.schemas import (
    TraceCreate, TraceResponse,
    EvalCreate, SummaryStats, TimeSeriesPoint,
    TopModel, Percentiles, AlertRuleCreate, AlertRuleResponse,
    HealthResponse,
)

router = APIRouter()


# ===== TRACES =====

@router.post("/traces", response_model=TraceResponse, status_code=201)
async def create_trace(trace: TraceCreate, session: AsyncSession = Depends(get_session_context)):
    """Log a new trace"""
    db_trace = TraceLog(
        trace_id=trace.trace_id,
        timestamp=datetime.utcnow(),
        model=trace.model,
        provider=trace.provider,
        request_type=trace.request_type,
        prompt_tokens=trace.prompt_tokens,
        completion_tokens=trace.completion_tokens,
        total_tokens=trace.total_tokens,
        latency_ms=trace.latency_ms,
        cost_usd=trace.cost_usd,
        cache_hit=trace.cache_hit,
        user_id=trace.user_id,
        api_key_id=trace.api_key_id,
        status_code=trace.status_code,
        error_message=trace.error_message,
        request_snapshot=trace.request_snapshot,
        response_snapshot=trace.response_snapshot,
        tags=trace.tags,
    )
    session.add(db_trace)
    await session.commit()
    await session.refresh(db_trace)
    return db_trace


@router.get("/traces", response_model=List[TraceResponse])
async def list_traces(
    limit: int = Query(default=100, le=1000),
    offset: int = Query(default=0, ge=0),
    model: Optional[str] = None,
    provider: Optional[str] = None,
    api_key_id: Optional[str] = None,
    start_time: Optional[datetime] = None,
    end_time: Optional[datetime] = None,
    session: AsyncSession = Depends(get_session_context),
):
    """List traces with optional filtering"""
    filters = []
    if model:
        filters.append(TraceLog.model == model)
    if provider:
        filters.append(TraceLog.provider == provider)
    if api_key_id:
        filters.append(TraceLog.api_key_id == api_key_id)
    if start_time:
        filters.append(TraceLog.timestamp >= start_time)
    if end_time:
        filters.append(TraceLog.timestamp <= end_time)

    query = select(TraceLog).order_by(desc(TraceLog.timestamp)).offset(offset).limit(limit)
    if filters:
        query = query.where(*filters)

    result = await session.execute(query)
    traces = result.scalars().all()
    return traces


@router.get("/traces/{trace_id}", response_model=TraceResponse)
async def get_trace(trace_id: str, session: AsyncSession = Depends(get_session_context)):
    """Get a specific trace by trace_id"""
    result = await session.execute(
        select(TraceLog).where(TraceLog.trace_id == trace_id).limit(1)
    )
    trace = result.scalar_one_or_none()
    if not trace:
        raise HTTPException(status_code=404, detail="Trace not found")
    return trace


# ===== ANALYTICS =====

@router.get("/analytics/summary", response_model=SummaryStats)
async def get_summary(
    start_time: Optional[datetime] = None,
    end_time: Optional[datetime] = None,
    model: Optional[str] = None,
    provider: Optional[str] = None,
    api_key_id: Optional[str] = None,
    session: AsyncSession = Depends(get_session_context),
):
    """Get summary statistics"""
    if not end_time:
        end_time = datetime.utcnow()
    if not start_time:
        start_time = end_time - timedelta(hours=24)

    stats = await AnalyticsService.get_summary_stats(
        session, start_time, end_time, model, provider, api_key_id
    )
    return stats


@router.get("/analytics/timeseries", response_model=List[TimeSeriesPoint])
async def get_timeseries(
    start_time: datetime = Query(...),
    end_time: datetime = Query(...),
    bucket_minutes: int = Query(default=60, ge=1, le=1440),
    model: Optional[str] = None,
    provider: Optional[str] = None,
    session: AsyncSession = Depends(get_session_context),
):
    """Get time-series data"""
    data = await AnalyticsService.get_time_series(
        session, start_time, end_time, bucket_minutes, model, provider
    )
    return data


@router.get("/analytics/top-models", response_model=List[TopModel])
async def get_top_models(
    start_time: Optional[datetime] = None,
    end_time: Optional[datetime] = None,
    limit: int = Query(default=10, le=50),
    session: AsyncSession = Depends(get_session_context),
):
    """Get top models by request count"""
    models = await AnalyticsService.get_top_models(
        session, start_time, end_time, limit
    )
    return models


@router.get("/analytics/percentiles", response_model=Percentiles)
async def get_percentiles(
    start_time: Optional[datetime] = None,
    end_time: Optional[datetime] = None,
    model: Optional[str] = None,
    session: AsyncSession = Depends(get_session_context),
):
    """Get latency percentiles"""
    if not end_time:
        end_time = datetime.utcnow()
    if not start_time:
        start_time = end_time - timedelta(hours=24)

    percentiles = await AnalyticsService.get_percentiles(
        session, start_time, end_time, model
    )
    return percentiles


# ===== EVALS =====

@router.post("/evals", status_code=201)
async def create_eval(eval_data: EvalCreate, session: AsyncSession = Depends(get_session_context)):
    """Log an evaluation result"""
    db_eval = EvalResult(
        trace_id=eval_data.trace_id,
        timestamp=datetime.utcnow(),
        eval_name=eval_data.eval_name,
        score=eval_data.score,
        passed=eval_data.passed,
        details=eval_data.details,
        feedback=eval_data.feedback,
    )
    session.add(db_eval)
    await session.commit()
    await session.refresh(db_eval)
    return {"id": db_eval.id, "status": "created"}


@router.get("/evals")
async def list_evals(
    trace_id: Optional[str] = None,
    eval_name: Optional[str] = None,
    limit: int = Query(default=100, le=1000),
    session: AsyncSession = Depends(get_session_context),
):
    """List evaluation results"""
    filters = []
    if trace_id:
        filters.append(EvalResult.trace_id == trace_id)
    if eval_name:
        filters.append(EvalResult.eval_name == eval_name)

    query = select(EvalResult).order_by(desc(EvalResult.timestamp)).limit(limit)
    if filters:
        query = query.where(*filters)

    result = await session.execute(query)
    evals = result.scalars().all()
    return evals


# ===== HEALTH =====

@router.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint"""
    return HealthResponse(
        status="healthy",
        version="1.0.0",
        database="connected",
    )
