package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/sj221097/llm-observability-lite/internal/config"
)

var (
	ErrInvalidAPIKey = errors.New("invalid or revoked API key")
	ErrMissingAuth   = errors.New("missing authorization header")
)

// Claims is the JWT claims for dashboard users.
type Claims struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	jwt.RegisteredClaims
}

// APIKeyAuth is middleware that validates an API key from the Authorization header.
func APIKeyAuth(getKey func(ctx context.Context, key string) (uuid.UUID, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": ErrMissingAuth.Error()})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}

		key := parts[1]
		workspaceID, err := getKey(c.Request.Context(), hashKey(key))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": ErrInvalidAPIKey.Error()})
			return
		}

		c.Set("workspace_id", workspaceID)
		c.Set("api_key", key)
		c.Next()
	}
}

// JWTAuth validates JWTs for dashboard (non-API) endpoints.
func JWTAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid token"})
			return
		}

		token, err := jwt.ParseWithClaims(parts[1], &Claims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "malformed claims"})
			return
		}

		c.Set("workspace_id", claims.WorkspaceID)
		c.Next()
	}
}

// GenerateDashboardToken creates a short-lived JWT for the dashboard UI.
func GenerateDashboardToken(workspaceID uuid.UUID, secret string) (string, error) {
	claims := &Claims{
		WorkspaceID: workspaceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "llm-observability-lite",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// hashKey hashes an API key for storage comparison.
// Keys are stored as SHA-256 hashes.
func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// HashKey exports the hash function for use in key creation.
func HashKey(key string) string {
	return hashKey(key)
}
