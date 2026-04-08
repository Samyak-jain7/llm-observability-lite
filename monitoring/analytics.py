"""
Analytics and aggregation service
"""
from datetime import datetime, timedelta
from typing import Optional
from sqlalchemy import func, select, and_
from sqlalchemy.ext.asyncio import AsyncSession
from db.models import TraceLog, EvalResult
from db.session import get_session_context


class AnalyticsService:
    """Service for computing aggregated analytics"""

    @staticmethod
    async def get_summary_stats(
        session: AsyncSession,
        start_time: Optional[datetime] = None,
        end_time: Optional[datetime] = None,
        model: Optional[str] = None,
        provider: Optional[str] = None,
        api_key_id: Optional[str] = None,
    ) -> dict:
        """Get summary statistics"""
        filters = []
        if start_time:
            filters.append(TraceLog.timestamp >= start_time)
        if end_time:
            filters.append(TraceLog.timestamp <= end_time)
        if model:
            filters.append(TraceLog.model == model)
        if provider:
            filters.append(TraceLog.provider == provider)
        if api_key_id:
            filters.append(TraceLog.api_key_id == api_key_id)

        where_clause = and_(*filters) if filters else True

        # Total requests
        total_req = await session.scalar(
            select(func.count(TraceLog.id)).where(where_clause)
        )

        # Total tokens
        tokens_result = await session.execute(
            select(
                func.sum(TraceLog.prompt_tokens),
                func.sum(TraceLog.completion_tokens),
                func.sum(TraceLog.total_tokens),
            ).where(where_clause)
        )
        row = tokens_result.one()
        total_prompt = row[0] or 0
        total_completion = row[1] or 0
        total_tokens_sum = row[2] or 0

        # Total cost
        total_cost = await session.scalar(
            select(func.sum(TraceLog.cost_usd)).where(where_clause)
        ) or 0.0

        # Avg latency
        avg_latency = await session.scalar(
            select(func.avg(TraceLog.latency_ms)).where(where_clause)
        ) or 0.0

        # Error count
        error_count = await session.scalar(
            select(func.count(TraceLog.id)).where(
                and_(where_clause, TraceLog.status_code >= 400)
            )
        )

        # Cache hits
        cache_hits = await session.scalar(
            select(func.count(TraceLog.id)).where(
                and_(where_clause, TraceLog.cache_hit == True)
            )
        )

        total_successful = await session.scalar(
            select(func.count(TraceLog.id)).where(
                and_(where_clause, TraceLog.status_code < 400)
            )
        )

        error_rate = 0.0
        if total_req and total_req > 0:
            error_rate = round((error_count / total_req) * 100, 2)

        cache_hit_rate = 0.0
        if total_req and total_req > 0:
            cache_hit_rate = round((cache_hits / total_req) * 100, 2)

        return {
            "total_requests": total_req or 0,
            "total_prompt_tokens": total_prompt,
            "total_completion_tokens": total_completion,
            "total_tokens": total_tokens_sum,
            "total_cost_usd": round(total_cost, 6),
            "avg_latency_ms": round(avg_latency, 2),
            "error_count": error_count or 0,
            "error_rate_percent": error_rate,
            "cache_hits": cache_hits or 0,
            "cache_hit_rate_percent": cache_hit_rate,
            "successful_requests": total_successful or 0,
        }

    @staticmethod
    async def get_time_series(
        session: AsyncSession,
        start_time: datetime,
        end_time: datetime,
        bucket_minutes: int = 60,
        model: Optional[str] = None,
        provider: Optional[str] = None,
    ) -> list:
        """Get time-series aggregated data"""
        filters = [
            TraceLog.timestamp >= start_time,
            TraceLog.timestamp <= end_time,
        ]
        if model:
            filters.append(TraceLog.model == model)
        if provider:
            filters.append(TraceLog.provider == provider)

        # Use raw SQL for bucket grouping (SQLite compatible)
        # bucket_minutes determines the granularity
        query = select(
            func.strftime(
                "%Y-%m-%d %H:00:00",
                TraceLog.timestamp
            ).label("bucket"),
            func.count(TraceLog.id).label("count"),
            func.sum(TraceLog.total_tokens).label("tokens"),
            func.sum(TraceLog.cost_usd).label("cost"),
            func.avg(TraceLog.latency_ms).label("latency"),
        ).where(and_(*filters)).group_by("bucket").order_by("bucket")

        result = await session.execute(query)
        rows = result.all()

        return [
            {
                "timestamp": row[0],
                "request_count": row[1],
                "total_tokens": row[2] or 0,
                "total_cost_usd": round(row[3] or 0.0, 6),
                "avg_latency_ms": round(row[4] or 0.0, 2),
            }
            for row in rows
        ]

    @staticmethod
    async def get_top_models(
        session: AsyncSession,
        start_time: Optional[datetime] = None,
        end_time: Optional[datetime] = None,
        limit: int = 10,
    ) -> list:
        """Get top models by request count"""
        filters = []
        if start_time:
            filters.append(TraceLog.timestamp >= start_time)
        if end_time:
            filters.append(TraceLog.timestamp <= end_time)

        where_clause = and_(*filters) if filters else True

        query = (
            select(
                TraceLog.model,
                TraceLog.provider,
                func.count(TraceLog.id).label("count"),
                func.sum(TraceLog.total_tokens).label("tokens"),
                func.sum(TraceLog.cost_usd).label("cost"),
                func.avg(TraceLog.latency_ms).label("avg_latency"),
            )
            .where(where_clause)
            .group_by(TraceLog.model, TraceLog.provider)
            .order_by(func.count(TraceLog.id).desc())
            .limit(limit)
        )

        result = await session.execute(query)
        rows = result.all()

        return [
            {
                "model": row[0],
                "provider": row[1],
                "request_count": row[2],
                "total_tokens": row[3] or 0,
                "total_cost_usd": round(row[4] or 0.0, 6),
                "avg_latency_ms": round(row[5] or 0.0, 2),
            }
            for row in rows
        ]

    @staticmethod
    async def get_percentiles(
        session: AsyncSession,
        start_time: Optional[datetime] = None,
        end_time: Optional[datetime] = None,
        model: Optional[str] = None,
    ) -> dict:
        """Get latency percentiles (p50, p95, p99)"""
        filters = [TraceLog.status_code < 400]
        if start_time:
            filters.append(TraceLog.timestamp >= start_time)
        if end_time:
            filters.append(TraceLog.timestamp <= end_time)
        if model:
            filters.append(TraceLog.model == model)

        # SQLite doesn't have percentile functions, use ordered subquery approach
        result = await session.execute(
            select(TraceLog.latency_ms)
            .where(and_(*filters))
            .order_by(TraceLog.latency_ms)
        )
        rows = result.scalars().all()

        if not rows:
            return {"p50": 0, "p95": 0, "p99": 0}

        n = len(rows)
        p50_idx = int(n * 0.50)
        p95_idx = int(n * 0.95)
        p99_idx = int(n * 0.99)

        return {
            "p50": rows[p50_idx] if p50_idx < n else rows[-1],
            "p95": rows[p95_idx] if p95_idx < n else rows[-1],
            "p99": rows[p99_idx] if p99_idx < n else rows[-1],
        }
