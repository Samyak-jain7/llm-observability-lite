package tracing

import (
	"time"

	"github.com/google/uuid"
	"github.com/sj221097/llm-observability-lite/internal/models"
)

// Tracer handles trace creation and enrichment.
type Tracer struct{}

func NewTracer() *Tracer {
	return &Tracer{}
}

// ToTrace converts an ingest request to a Trace model.
func (t *Tracer) ToTrace(workspaceID uuid.UUID, req *models.IngestRequest) *models.Trace {
	return &models.Trace{
		ID:           uuid.New(),
		WorkspaceID:  workspaceID,
		TraceID:      req.TraceID,
		Model:        req.Model,
		Provider:     req.Provider,
		PromptTokens: req.PromptTokens,
		OutputTokens: req.OutputTokens,
		LatencyMs:   req.LatencyMs,
		CostUSD:      req.CostUSD,
		Status:       req.Status,
		StatusCode:   req.StatusCode,
		ErrorMsg:     req.ErrorMsg,
		Metadata:     req.Metadata,
		CreatedAt:    time.Now().UTC(),
	}
}

// Span represents a nested span within a trace.
type Span struct {
	ID         uuid.UUID              `json:"id"`
	TraceID    uuid.UUID              `json:"trace_id"`
	ParentID   *uuid.UUID             `json:"parent_id,omitempty"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"` // "llm" | "embedding" | "tool" | "custom"
	StartMs    int64                  `json:"start_ms"`
	DurationMs int                    `json:"duration_ms"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}
