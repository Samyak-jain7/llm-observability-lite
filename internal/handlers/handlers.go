package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sj221097/llm-observability-lite/internal/auth"
	"github.com/sj221097/llm-observability-lite/internal/billing"
	"github.com/sj221097/llm-observability-lite/internal/models"
	"github.com/sj221097/llm-observability-lite/internal/storage"
	"github.com/sj221097/llm-observability-lite/internal/tracing"
)

type GatewayHandler struct {
	store        storage.Storage
	tracer       *tracing.Tracer
	billing      *billing.StripeService
	getKeyHashFn func(key string) string
}

func NewGatewayHandler(s storage.Storage, t *tracing.Tracer, b *billing.StripeService) *GatewayHandler {
	return &GatewayHandler{
		store:   s,
		tracer:  t,
		billing: b,
		getKeyHashFn: auth.HashKey,
	}
}

// RegisterRoutes wires up all HTTP routes.
func (h *GatewayHandler) RegisterRoutes(r *gin.Engine) {
	// Health — no auth
	r.GET("/health", h.Health)

	// Stripe webhook — raw body, special middleware
	r.POST("/v1/webhooks/stripe", h.HandleStripeWebhook)

	api := r.Group("/v1")
	{
		// API key auth for trace ingestion
		api.Use(auth.APIKeyAuth(h.getWorkspaceIDFromKey))
		{
			// Ingest
			api.POST("/trace", h.IngestTrace)
			api.POST("/traces/batch", h.IngestBatch)

			// Read
			api.GET("/traces", h.ListTraces)
			api.GET("/traces/:id", h.GetTrace)
			api.GET("/stats", h.GetStats)
			api.GET("/stats/summary", h.GetStatsSummary)
		}

		// Dashboard auth (JWT)
		dash := api.Group("/workspace")
		dash.Use(auth.JWTAuth(loadConfig()))
		{
			dash.GET("/me", h.GetWorkspace)
			dash.POST("/api-keys", h.CreateAPIKey)
			dash.DELETE("/api-keys/:id", h.RevokeAPIKey)
			dash.GET("/api-keys", h.ListAPIKeys)
		}
	}
}

func loadConfig() *struct{} {
	// Placeholder — real impl passes *config.Config through the handler struct
	return nil
}

// Health returns service status.
func (h *GatewayHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "llm-observability-lite",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// IngestTrace records a single trace.
func (h *GatewayHandler) IngestTrace(c *gin.Context) {
	workspaceID := c.MustGet("workspace_id").(uuid.UUID)

	var req models.IngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}

	// Check quota
	if err := h.checkQuota(c, workspaceID); err != nil {
		return
	}

	trace := h.tracer.ToTrace(workspaceID, &req)
	if err := h.store.InsertTrace(c.Request.Context(), trace); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to store trace", Code: "STORAGE_ERROR"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         trace.ID,
		"trace_id":   trace.TraceID,
		"workspace":  workspaceID,
		"cost_usd":   trace.CostUSD,
		"status":     "recorded",
	})
}

// IngestBatch records up to 100 traces at once.
func (h *GatewayHandler) IngestBatch(c *gin.Context) {
	workspaceID := c.MustGet("workspace_id").(uuid.UUID)

	var reqs []models.IngestRequest
	if err := c.ShouldBindJSON(&reqs); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}

	if len(reqs) > 100 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "max 100 traces per batch", Code: "BATCH_TOO_LARGE"})
		return
	}

	if err := h.checkQuota(c, workspaceID); err != nil {
		return
	}

	results := make([]gin.H, 0, len(reqs))
	ctx := c.Request.Context()

	for _, r := range reqs {
		trace := h.tracer.ToTrace(workspaceID, &r)
		if err := h.store.InsertTrace(ctx, trace); err != nil {
			results = append(results, gin.H{"trace_id": r.TraceID, "status": "error", "error": err.Error()})
			continue
		}
		results = append(results, gin.H{"id": trace.ID, "trace_id": trace.TraceID, "status": "recorded"})
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

// ListTraces returns paginated traces.
func (h *GatewayHandler) ListTraces(c *gin.Context) {
	workspaceID := c.MustGet("workspace_id").(uuid.UUID)

	var p models.Pagination
	if err := c.ShouldBindQuery(&p); err != nil {
		p.Limit = 50
	}
	if p.Limit > 200 {
		p.Limit = 200
	}

	traces, nextCursor, err := h.store.ListTraces(c.Request.Context(), workspaceID, p.Limit, p.Cursor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list traces"})
		return
	}

	response := gin.H{"traces": traces}
	if nextCursor != "" {
		response["next_cursor"] = nextCursor
	}

	c.JSON(http.StatusOK, response)
}

// GetTrace returns a single trace by ID.
func (h *GatewayHandler) GetTrace(c *gin.Context) {
	workspaceID := c.MustGet("workspace_id").(uuid.UUID)
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid trace ID"})
		return
	}

	trace, err := h.store.GetTrace(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "trace not found"})
		return
	}

	if trace.WorkspaceID != workspaceID {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "trace not found"})
		return
	}

	c.JSON(http.StatusOK, trace)
}

// GetStats returns detailed stats with optional time range.
func (h *GatewayHandler) GetStats(c *gin.Context) {
	workspaceID := c.MustGet("workspace_id").(uuid.UUID)

	since := time.Now().Add(-30 * 24 * time.Hour) // default: last 30 days
	if days := c.Query("days"); days != "" {
		if d := parseDays(days); d > 0 {
			since = time.Now().Add(-time.Duration(d) * 24 * time.Hour)
		}
	}

	stats, err := h.store.GetTraceStats(c.Request.Context(), workspaceID, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to compute stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats":      stats,
		"since":      since.Format(time.RFC3339),
		"until":      time.Now().Format(time.RFC3339),
	})
}

// GetStatsSummary returns a lightweight summary for dashboard quick-view.
func (h *GatewayHandler) GetStatsSummary(c *gin.Context) {
	workspaceID := c.MustGet("workspace_id").(uuid.UUID)

	stats, err := h.store.GetTraceStats(c.Request.Context(), workspaceID, time.Now().Add(-24*time.Hour))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to compute stats"})
		return
	}

	workspace, _ := h.store.GetWorkspace(c.Request.Context(), workspaceID)

	c.JSON(http.StatusOK, gin.H{
		"traces_24h":     stats.TotalTraces,
		"cost_24h_usd":   stats.TotalCostUSD,
		"avg_latency_ms": stats.AvgLatencyMs,
		"success_rate":   stats.SuccessRate,
		"quota": gin.H{
			"used":  workspace.TraceUsed,
			"limit": workspace.TraceLimit,
		},
	})
}

// GetWorkspace returns the current workspace details.
func (h *GatewayHandler) GetWorkspace(c *gin.Context) {
	workspaceID := c.MustGet("workspace_id").(uuid.UUID)

	workspace, err := h.store.GetWorkspace(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "workspace not found"})
		return
	}

	c.JSON(http.StatusOK, workspace)
}

// CreateAPIKey creates a new API key for the workspace.
func (h *GatewayHandler) CreateAPIKey(c *gin.Context) {
	workspaceID := c.MustGet("workspace_id").(uuid.UUID)

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "name is required"})
		return
	}

	key := &models.APIKey{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		Name:        req.Name,
		CreatedAt:   time.Now().UTC(),
	}

	plainKey, err := h.store.CreateAPIKey(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create API key"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":        key.ID,
		"name":      key.Name,
		"api_key":   plainKey, // shown only once
		"created":   key.CreatedAt,
		"warning":   "Save this API key now. It will not be shown again.",
	})
}

// RevokeAPIKey revokes an API key.
func (h *GatewayHandler) RevokeAPIKey(c *gin.Context) {
	workspaceID := c.MustGet("workspace_id").(uuid.UUID)
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid key ID"})
		return
	}

	if err := h.store.RevokeAPIKey(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to revoke key"})
		return
	}

	_ = workspaceID // suppress unused warning
	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

// ListAPIKeys returns all API keys for the workspace.
func (h *GatewayHandler) ListAPIKeys(c *gin.Context) {
	workspaceID := c.MustGet("workspace_id").(uuid.UUID)

	keys, err := h.store.ListAPIKeys(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list keys"})
		return
	}

	// Mask key values for security
	masked := make([]gin.H, len(keys))
	for i, k := range keys {
		masked[i] = gin.H{
			"id":         k.ID,
			"name":       k.Name,
			"created_at": k.CreatedAt,
			"revoked_at": k.RevokedAt,
			"key":        "llmobs_••••••••••••",
		}
	}

	c.JSON(http.StatusOK, gin.H{"api_keys": masked})
}

// HandleStripeWebhook processes Stripe billing webhooks.
func (h *GatewayHandler) HandleStripeWebhook(c *gin.Context) {
	payload, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "failed to read body"})
		return
	}

	sig := c.GetHeader("Stripe-Signature")
	if err := h.billing.HandleWebhook(c.Request.Context(), payload, sig); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

// checkQuota verifies the workspace has not exceeded its trace limit.
func (h *GatewayHandler) checkQuota(c *gin.Context, workspaceID uuid.UUID) error {
	workspace, err := h.store.GetWorkspace(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "workspace lookup failed"})
		return err
	}

	limit := billing.GetPlanLimit(workspace.Plan)
	if workspace.TraceUsed >= limit {
		c.JSON(http.StatusTooManyRequests, models.ErrorResponse{
			Error: "quota exceeded",
			Code:  "QUOTA_EXCEEDED",
		})
		return err
	}
	return nil
}

func (h *GatewayHandler) getWorkspaceIDFromKey(ctx interface{}, key string) (uuid.UUID, error) {
	// This is passed to APIKeyAuth middleware — real impl uses the store
	ginCtx := ctx.(gin.Context)
	store := h.store
	hash := auth.HashKey(key)
	return store.GetWorkspaceByAPIKey(ginCtx.Request.Context(), hash)
}

func parseDays(s string) int {
	var d int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			d = d*10 + int(c-'0')
		} else {
			return 0
		}
	}
	return d
}
