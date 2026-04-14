package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sj221097/llm-observability-lite/internal/models"
)

// Storage defines the interface for trace persistence.
type Storage interface {
	// Traces
	InsertTrace(ctx context.Context, t *models.Trace) error
	GetTrace(ctx context.Context, id uuid.UUID) (*models.Trace, error)
	ListTraces(ctx context.Context, workspaceID uuid.UUID, limit int, cursor string) ([]*models.Trace, string, error)
	GetTraceStats(ctx context.Context, workspaceID uuid.UUID, since time.Time) (*models.TraceStats, error)

	// Workspaces
	GetWorkspace(ctx context.Context, id uuid.UUID) (*models.Workspace, error)
	GetWorkspaceByAPIKey(ctx context.Context, keyHash string) (*models.Workspace, error)
	UpdateWorkspaceUsage(ctx context.Context, workspaceID uuid.UUID) error

	// API Keys
	CreateAPIKey(ctx context.Context, key *models.APIKey) (keyValue string, err error)
	RevokeAPIKey(ctx context.Context, id uuid.UUID) error
	ListAPIKeys(ctx context.Context, workspaceID uuid.UUID) ([]*models.APIKey, error)
}

// PostgresStorage implements Storage with PostgreSQL + Redis.
type PostgresStorage struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

func NewPostgresStorage(ctx context.Context, databaseURL string, redisURL string) (*PostgresStorage, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}
	rdb := redis.NewClient(opts)

	return &PostgresStorage{db: pool, redis: rdb}, nil
}

func (s *PostgresStorage) Close() {
	s.db.Close()
	s.redis.Close()
}

// InsertTrace stores a trace and increments workspace usage.
func (s *PostgresStorage) InsertTrace(ctx context.Context, t *models.Trace) error {
	metaJSON, _ := json.Marshal(t.Metadata)

	query := `
		INSERT INTO traces (id, workspace_id, trace_id, model, provider,
			prompt_tokens, output_tokens, latency_ms, cost_usd, status,
			status_code, error_msg, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err := s.db.Exec(ctx, query,
		t.ID, t.WorkspaceID, t.TraceID, t.Model, t.Provider,
		t.PromptTokens, t.OutputTokens, t.LatencyMs, t.CostUSD, t.Status,
		t.StatusCode, t.ErrorMsg, metaJSON, t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert trace: %w", err)
	}

	// Increment workspace usage counter in Redis for fast quota checking
	key := fmt.Sprintf("usage:%s:%d", t.WorkspaceID, time.Now().YearMonth())
	s.redis.IncrBy(ctx, key, 1)
	s.redis.Expire(ctx, key, 35*24*time.Hour)

	return nil
}

// GetTrace retrieves a single trace by ID.
func (s *PostgresStorage) GetTrace(ctx context.Context, id uuid.UUID) (*models.Trace, error) {
	var t models.Trace
	var metaJSON []byte

	query := `
		SELECT id, workspace_id, trace_id, model, provider,
			prompt_tokens, output_tokens, latency_ms, cost_usd, status,
			status_code, error_msg, metadata, created_at
		FROM traces WHERE id = $1
	`
	err := s.db.QueryRow(ctx, query, id).Scan(
		&t.ID, &t.WorkspaceID, &t.TraceID, &t.Model, &t.Provider,
		&t.PromptTokens, &t.OutputTokens, &t.LatencyMs, &t.CostUSD, &t.Status,
		&t.StatusCode, &t.ErrorMsg, &metaJSON, &t.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("trace not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get trace: %w", err)
	}

	if len(metaJSON) > 0 {
		json.Unmarshal(metaJSON, &t.Metadata)
	}

	return &t, nil
}

// ListTraces returns paginated traces for a workspace.
func (s *PostgresStorage) ListTraces(ctx context.Context, workspaceID uuid.UUID, limit int, cursor string) ([]*models.Trace, string, error) {
	args := []interface{}{workspaceID}
	paramIdx := 2

	query := `
		SELECT id, workspace_id, trace_id, model, provider,
			prompt_tokens, output_tokens, latency_ms, cost_usd, status,
			status_code, error_msg, metadata, created_at
		FROM traces WHERE workspace_id = $1
	`

	if cursor != "" {
		query += fmt.Sprintf(" AND created_at < $%d", paramIdx)
		args = append(args, cursor)
		paramIdx++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", paramIdx)
	args = append(args, limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list traces: %w", err)
	}
	defer rows.Close()

	var traces []*models.Trace
	for rows.Next() {
		var t models.Trace
		var metaJSON []byte
		err := rows.Scan(
			&t.ID, &t.WorkspaceID, &t.TraceID, &t.Model, &t.Provider,
			&t.PromptTokens, &t.OutputTokens, &t.LatencyMs, &t.CostUSD, &t.Status,
			&t.StatusCode, &t.ErrorMsg, &metaJSON, &t.CreatedAt,
		)
		if err != nil {
			return nil, "", fmt.Errorf("scan trace: %w", err)
		}
		if len(metaJSON) > 0 {
			json.Unmarshal(metaJSON, &t.Metadata)
		}
		traces = append(traces, &t)
	}

	var nextCursor string
	if len(traces) > limit {
		traces = traces[:limit]
		nextCursor = traces[len(traces)-1].CreatedAt.Format(time.RFC3339Nano)
	}

	return traces, nextCursor, nil
}

// GetTraceStats computes aggregated stats for a workspace.
func (s *PostgresStorage) GetTraceStats(ctx context.Context, workspaceID uuid.UUID, since time.Time) (*models.TraceStats, error) {
	var stats models.TraceStats

	query := `
		SELECT
			COUNT(*) as total_traces,
			COALESCE(SUM(cost_usd), 0) as total_cost,
			COALESCE(SUM(prompt_tokens), 0) as total_prompt_tokens,
			COALESCE(SUM(output_tokens), 0) as total_output_tokens,
			COALESCE(AVG(latency_ms), 0) as avg_latency,
			COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) as success_count,
			COALESCE(SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END), 0) as error_count,
			COALESCE(SUM(CASE WHEN metadata->>'cache_hit' = 'true' THEN 1 ELSE 0 END), 0) as cache_hit_count
		FROM traces
		WHERE workspace_id = $1 AND created_at >= $2
	`

	var totalTraces, successCount, errorCount, cacheHitCount int64
	err := s.db.QueryRow(ctx, query, workspaceID, since).Scan(
		&totalTraces,
		&stats.TotalCostUSD,
		&stats.TotalPromptTokens,
		&stats.TotalOutputTokens,
		&stats.AvgLatencyMs,
		&successCount,
		&errorCount,
		&cacheHitCount,
	)
	if err != nil {
		return nil, fmt.Errorf("get stats: %w", err)
	}

	stats.TotalTraces = totalTraces
	if totalTraces > 0 {
		stats.SuccessRate = float64(successCount) / float64(totalTraces)
		stats.ErrorRate = float64(errorCount) / float64(totalTraces)
		stats.CacheHitRate = float64(cacheHitCount) / float64(totalTraces)
	}

	latencies, err := s.getRecentLatencies(ctx, workspaceID, since, 1000)
	if err == nil && len(latencies) > 0 {
		stats.P50LatencyMs = percentile(latencies, 50)
		stats.P95LatencyMs = percentile(latencies, 95)
		stats.P99LatencyMs = percentile(latencies, 99)
	}

	return &stats, nil
}

func (s *PostgresStorage) getRecentLatencies(ctx context.Context, workspaceID uuid.UUID, since time.Time, limit int) ([]int, error) {
	query := `SELECT latency_ms FROM traces WHERE workspace_id = $1 AND created_at >= $2 ORDER BY created_at DESC LIMIT $3`
	rows, err := s.db.Query(ctx, query, workspaceID, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var latencies []int
	for rows.Next() {
		var l int
		if err := rows.Scan(&l); err == nil {
			latencies = append(latencies, l)
		}
	}
	sort.Ints(latencies)
	return latencies, nil
}

func percentile(sorted []int, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(float64(len(sorted))*float64(p)/100.0)) - 1
	if idx < 0 {
		idx = 0
	}
	return float64(sorted[idx])
}

func (s *PostgresStorage) GetWorkspace(ctx context.Context, id uuid.UUID) (*models.Workspace, error) {
	var w models.Workspace
	query := `
		SELECT id, name, plan, stripe_sub_id, trace_used, trace_limit, created_at
		FROM workspaces WHERE id = $1
	`
	err := s.db.QueryRow(ctx, query, id).Scan(
		&w.ID, &w.Name, &w.Plan, &w.StripeSubID, &w.TraceUsed, &w.TraceLimit, &w.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("workspace not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	return &w, nil
}

func (s *PostgresStorage) GetWorkspaceByAPIKey(ctx context.Context, keyHash string) (*models.Workspace, error) {
	var w models.Workspace
	query := `
		SELECT w.id, w.name, w.plan, w.stripe_sub_id, w.trace_used, w.trace_limit, w.created_at
		FROM workspaces w
		JOIN api_keys k ON k.workspace_id = w.id
		WHERE k.key_hash = $1 AND k.revoked_at IS NULL
	`
	err := s.db.QueryRow(ctx, query, keyHash).Scan(
		&w.ID, &w.Name, &w.Plan, &w.StripeSubID, &w.TraceUsed, &w.TraceLimit, &w.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("invalid API key")
	}
	if err != nil {
		return nil, fmt.Errorf("get workspace by key: %w", err)
	}
	return &w, nil
}

func (s *PostgresStorage) UpdateWorkspaceUsage(ctx context.Context, workspaceID uuid.UUID) error {
	key := fmt.Sprintf("usage:%s:%d", workspaceID, time.Now().YearMonth())
	count, err := s.redis.Get(ctx, key).Int64()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("get usage from redis: %w", err)
	}
	if err == redis.Nil {
		count = 0
	}

	_, err = s.db.Exec(ctx, `UPDATE workspaces SET trace_used = $1 WHERE id = $2`, count, workspaceID)
	return err
}

func (s *PostgresStorage) CreateAPIKey(ctx context.Context, key *models.APIKey) (string, error) {
	plainKey := fmt.Sprintf("llmobs_%s_%s", key.WorkspaceID.String()[:8], uuid.New().String()[:12])
	hashed := hashKeySHA256(plainKey)
	key.Key = hashed

	query := `
		INSERT INTO api_keys (id, workspace_id, key_hash, name, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := s.db.Exec(ctx, query, key.ID, key.WorkspaceID, key.Key, key.Name, key.CreatedAt)
	if err != nil {
		return "", fmt.Errorf("create api key: %w", err)
	}

	return plainKey, nil
}

func (s *PostgresStorage) RevokeAPIKey(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `UPDATE api_keys SET revoked_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *PostgresStorage) ListAPIKeys(ctx context.Context, workspaceID uuid.UUID) ([]*models.APIKey, error) {
	query := `
		SELECT id, workspace_id, name, created_at, revoked_at
		FROM api_keys WHERE workspace_id = $1 ORDER BY created_at DESC
	`
	rows, err := s.db.Query(ctx, query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.WorkspaceID, &k.Name, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		keys = append(keys, &k)
	}
	return keys, nil
}

func hashKeySHA256(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}
