// Package middleware provides HTTP middleware for the agent API.
package middleware

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/cofedish/3xui-agents/logger"
)

// MTLSAuth middleware verifies client certificate.
func MTLSAuth(caFile string) gin.HandlerFunc {
	// Load CA certificate
	caCert, err := tls.LoadX509KeyPair(caFile, caFile)
	if err != nil {
		logger.Error("Failed to load CA certificate:", err)
		return func(c *gin.Context) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "MTLS_SETUP_ERROR",
					"message": "mTLS configuration error",
				},
			})
		}
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(caCert.Leaf)

	return func(c *gin.Context) {
		// Check if TLS is used
		if c.Request.TLS == nil {
			logger.Warning("Non-TLS request to agent API")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "TLS_REQUIRED",
					"message": "TLS is required for agent API",
				},
			})
			return
		}

		// Verify client certificate
		if len(c.Request.TLS.PeerCertificates) == 0 {
			logger.Warning("No client certificate provided")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "CLIENT_CERT_REQUIRED",
					"message": "Client certificate is required",
				},
			})
			return
		}

		// Get client certificate
		clientCert := c.Request.TLS.PeerCertificates[0]

		// Verify against CA
		opts := x509.VerifyOptions{
			Roots: caCertPool,
		}

		if _, err := clientCert.Verify(opts); err != nil {
			logger.Warning("Client certificate verification failed:", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "CERT_VERIFICATION_FAILED",
					"message": "Client certificate verification failed",
				},
			})
			return
		}

		// Certificate is valid
		c.Set("client_cn", clientCert.Subject.CommonName)
		c.Next()
	}
}

// JWTAuth middleware verifies JWT token (simplified implementation).
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "AUTH_REQUIRED",
					"message": "Authorization header is required",
				},
			})
			return
		}

		// Check Bearer token format
		if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_AUTH_FORMAT",
					"message": "Authorization header must be 'Bearer <token>'",
				},
			})
			return
		}

		token := authHeader[7:]

		// TODO: Implement proper JWT validation
		// For now, simple secret comparison (NOT PRODUCTION READY)
		if token != secret {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_TOKEN",
					"message": "Invalid authentication token",
				},
			})
			return
		}

		c.Next()
	}
}

// TraceID middleware adds a unique trace ID to each request.
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate or use existing trace ID
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}

		c.Set("trace_id", traceID)
		c.Header("X-Trace-ID", traceID)

		c.Next()
	}
}

// RequestLogger middleware logs all requests.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Log after processing
		duration := time.Since(start)
		logger.Info(fmt.Sprintf(
			"[Agent API] %s %s | Status: %d | Duration: %v | TraceID: %s",
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			duration,
			c.GetString("trace_id"),
		))
	}
}

// RateLimiter implements a simple token bucket rate limiter.
type RateLimiter struct {
	limit    int                    // requests per minute
	buckets  map[string]*tokenBucket
	mu       sync.RWMutex
	cleanupTicker *time.Ticker
}

type tokenBucket struct {
	tokens    int
	lastRefill time.Time
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	rl := &RateLimiter{
		limit:   requestsPerMinute,
		buckets: make(map[string]*tokenBucket),
	}

	// Cleanup old buckets every 5 minutes
	rl.cleanupTicker = time.NewTicker(5 * time.Minute)
	go rl.cleanup()

	return rl
}

// cleanup removes old buckets.
func (rl *RateLimiter) cleanup() {
	for range rl.cleanupTicker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, bucket := range rl.buckets {
			if now.Sub(bucket.lastRefill) > 10*time.Minute {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware returns a Gin middleware function.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Use client IP as key
		clientIP := c.ClientIP()

		if !rl.allow(clientIP) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": fmt.Sprintf("Rate limit exceeded: %d requests per minute", rl.limit),
				},
			})
			return
		}

		c.Next()
	}
}

// allow checks if a request is allowed.
func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	bucket, exists := rl.buckets[key]
	if !exists {
		bucket = &tokenBucket{
			tokens:    rl.limit,
			lastRefill: now,
		}
		rl.buckets[key] = bucket
	}

	// Refill tokens based on time elapsed
	elapsed := now.Sub(bucket.lastRefill)
	tokensToAdd := int(elapsed.Minutes() * float64(rl.limit))

	if tokensToAdd > 0 {
		bucket.tokens += tokensToAdd
		if bucket.tokens > rl.limit {
			bucket.tokens = rl.limit
		}
		bucket.lastRefill = now
	}

	// Check if request is allowed
	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}

	return false
}

// Stop stops the rate limiter cleanup goroutine.
func (rl *RateLimiter) Stop() {
	rl.cleanupTicker.Stop()
}

// MaxBodySize middleware limits request body size.
func MaxBodySize(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
		c.Next()
	}
}
