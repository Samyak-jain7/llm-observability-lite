package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/sj221097/llm-observability-lite/internal/auth"
	"github.com/sj221097/llm-observability-lite/internal/billing"
	"github.com/sj221097/llm-observability-lite/internal/config"
	"github.com/sj221097/llm-observability-lite/internal/handlers"
	"github.com/sj221097/llm-observability-lite/internal/middleware"
	"github.com/sj221097/llm-observability-lite/internal/storage"
	"github.com/sj221097/llm-observability-lite/internal/tracing"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()

	// Initialize storage
	store, err := storage.NewPostgresStorage(ctx, cfg.DatabaseURL, cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to connect to storage: %v", err)
	}
	defer store.Close()

	// Initialize services
	tracer := tracing.NewTracer()
	billingSvc := billing.NewStripeService(cfg.StripeSecretKey, cfg.StripeWebhookSecret)

	// Initialize handler
	h := handlers.NewGatewayHandler(store, tracer, billingSvc)

	// Setup Gin
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.RecoveryMiddleware())
	r.Use(middleware.LoggingMiddleware())
	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.CORSMiddleware("*"))

	// Health always available
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "llm-observability-lite"})
	})

	// Register all routes
	h.RegisterRoutes(r)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		port := cfg.Port
		if port == "" {
			port = "8080"
		}
		log.Printf("LLM Observability Lite starting on :%s", port)
		if err := r.Run(":" + port); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-quit
	log.Println("Shutting down...")

	_ = auth.GenerateDashboardToken // reference to avoid unused import warning
}
