package models

import (
	"time"

	"github.com/google/uuid"
)

// Trace represents a single LLM call trace.
type Trace struct {
	ID            uuid.UUID              `json:"id"`
	WorkspaceID   uuid.UUID              `json:"workspace_id"`
	TraceID       string                 `json:"trace_id"`        // client-supplied idempotency key
	Model         string                 `json:"model"`
	Provider      string                 `json:"provider"`        // "openai" | "anthropic" | "google"
	PromptTokens  int                    `json:"prompt_tokens"`
	OutputTokens  int                    `json:"output_tokens"`
	LatencyMs     int                    `json:"latency_ms"`
	CostUSD       float64                `json:"cost_usd"`
	Status        string                 `json:"status"`          // "success" | "error" | "timeout"
	StatusCode    int                    `json:"status_code,omitempty"`
	ErrorMsg      string                 `json:"error_msg,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

// TraceStats are aggregated statistics for a workspace.
type TraceStats struct {
	TotalTraces     int64   `json:"total_traces"`
	TotalCostUSD    float64 `json:"total_cost_usd"`
	TotalPromptTokens  int64 `json:"total_prompt_tokens"`
	TotalOutputTokens int64 `json:"total_output_tokens"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	P50LatencyMs    float64 `json:"p50_latency_ms"`
	P95LatencyMs    float64 `json:"p95_latency_ms"`
	P99LatencyMs    float64 `json:"p99_latency_ms"`
	SuccessRate      float64 `json:"success_rate"` // 0-1
	ErrorRate        float64 `json:"error_rate"`
	CacheHitRate     float64 `json:"cache_hit_rate"`
}

// Workspace represents a customer workspace (tenant).
type Workspace struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Plan         string    `json:"plan"` // "free" | "dev" | "startup" | "growth"
	StripeSubID  string    `json:"stripe_sub_id,omitempty"`
	TraceUsed    int64     `json:"trace_used"`
	TraceLimit   int64     `json:"trace_limit"`
	CreatedAt    time.Time `json:"created_at"`
}

// APIKey represents an API key for a workspace.
type APIKey struct {
	ID          uuid.UUID `json:"id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Key         string    `json:"key"` // hashed in DB, shown once on creation
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

// IngestRequest is the payload sent by clients to record a trace.
type IngestRequest struct {
	TraceID        string                 `json:"trace_id" binding:"required"`
	Model          string                 `json:"model" binding:"required"`
	Provider       string                 `json:"provider" binding:"required"`
	PromptTokens   int                    `json:"prompt_tokens"`
	OutputTokens   int                    `json:"output_tokens"`
	LatencyMs      int                    `json:"latency_ms"`
	CostUSD        float64                `json:"cost_usd"`
	Status         string                 `json:"status" binding:"required"`
	StatusCode     int                    `json:"status_code"`
	ErrorMsg       string                 `json:"error_msg"`
	Metadata       map[string]interface{} `json:"metadata"`
	CacheHit       bool                   `json:"cache_hit"`
}

// ErrorResponse is a standard error payload.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// Pagination holds cursor-based pagination state.
type Pagination struct {
	Cursor string `form:"cursor"`
	Limit  int    `form:"limit,default=50"`
}
