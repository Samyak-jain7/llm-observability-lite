package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDMiddleware adds a unique request ID to every request.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		c.Set("request_id", reqID)
		c.Header("X-Request-ID", reqID)
		c.Next()
	}
}

// CORSMiddleware sets appropriate CORS headers.
func CORSMiddleware(allowedOrigins string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowedOrigins)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, Stripe-Signature")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// LoggingMiddleware logs request method, path, status, and latency.
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		reqID, _ := c.Get("request_id")

		log.Printf("[%s] %s %s %d %v request_id=%s",
			time.Now().Format("2006-01-02T15:04:05Z"),
			method,
			path,
			status,
			latency,
			reqID,
		)
	}
}

// RecoveryMiddleware recovers from panics and returns 500.
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[PANIC] %v", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
					"code":  "INTERNAL_ERROR",
				})
			}
		}()
		c.Next()
	}
}

// RateLimitMiddleware is a placeholder for token-bucket rate limiting.
// Production: use Redis + sliding window, or a dedicated rate-limit library.
func RateLimitMiddleware(requestsPerMinute int) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement token bucket with Redis
		// For MVP: no-op, add later with a proper rate-limit lib
		_ = requestsPerMinute
		c.Next()
	}
}
