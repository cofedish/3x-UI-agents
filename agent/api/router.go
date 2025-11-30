// Package api provides HTTP routing for the agent API.
package api

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/agent/config"
	"github.com/mhsanaei/3x-ui/v2/agent/middleware"
	"github.com/mhsanaei/3x-ui/v2/logger"
)

// SetupRouter creates and configures the Gin router for agent API.
func SetupRouter(cfg *config.AgentConfig) *gin.Engine {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.TraceID())
	router.Use(middleware.RequestLogger())

	// Rate limiting
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimit)
	router.Use(rateLimiter.Middleware())

	// Max body size (10MB)
	router.Use(middleware.MaxBodySize(10 * 1024 * 1024))

	// Authentication middleware
	var authMiddleware gin.HandlerFunc
	if cfg.AuthType == "mtls" {
		authMiddleware = middleware.MTLSAuth(cfg.CAFile)
	} else if cfg.AuthType == "jwt" {
		authMiddleware = middleware.JWTAuth(cfg.JWTSecret)
	}

	// Create handlers
	handlers := NewAgentHandlers()

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Public endpoints (no auth required for health check)
		v1.GET("/health", handlers.Health)

		// Protected endpoints
		protected := v1.Group("")
		protected.Use(authMiddleware)
		{
			// Server info
			protected.GET("/info", handlers.Info)

			// Inbound management
			inbounds := protected.Group("/inbounds")
			{
				inbounds.GET("", handlers.ListInbounds)
				inbounds.GET("/:id", handlers.GetInbound)
				inbounds.POST("", handlers.AddInbound)
				inbounds.PUT("/:id", handlers.UpdateInbound)
				inbounds.DELETE("/:id", handlers.DeleteInbound)

				// Client management
				inbounds.POST("/:id/clients", handlers.AddClient)
				inbounds.DELETE("/:id/clients/:email", handlers.DeleteClient)
			}

			// Traffic and stats
			protected.GET("/traffic", handlers.GetTraffic)
			protected.GET("/traffic/clients", handlers.GetClientTraffics)
			protected.GET("/clients/online", handlers.GetOnlineClients)

			// Xray control
			xrayGroup := protected.Group("/xray")
			{
				xrayGroup.POST("/start", handlers.StartXray)
				xrayGroup.POST("/stop", handlers.StopXray)
				xrayGroup.POST("/restart", handlers.RestartXray)
				xrayGroup.GET("/version", handlers.GetXrayVersion)
				xrayGroup.GET("/config", handlers.GetXrayConfig)
			}

			// System operations
			protected.GET("/system/stats", handlers.GetSystemStats)
			protected.GET("/logs", handlers.GetLogs)
			protected.POST("/geofiles/update", handlers.UpdateGeoFiles)
		}
	}

	// Root endpoint
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":    "3x-ui Agent",
			"version": "2.0.0",
			"status":  "running",
		})
	})

	return router
}

// StartServer starts the agent API server with TLS.
func StartServer(cfg *config.AgentConfig, router *gin.Engine) error {
	logger.Info(fmt.Sprintf("Starting 3x-ui Agent API on %s", cfg.ListenAddr))
	logger.Info(fmt.Sprintf("Auth type: %s", cfg.AuthType))

	if cfg.AuthType == "mtls" {
		// Start with mTLS
		return startTLSServer(cfg, router)
	}

	// Start with regular HTTPS (JWT auth)
	return startHTTPSServer(cfg, router)
}

// startTLSServer starts server with mTLS.
func startTLSServer(cfg *config.AgentConfig, router *gin.Engine) error {
	// Load server certificate
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}

	// Create server
	server := &http.Server{
		Addr:      cfg.ListenAddr,
		Handler:   router,
		TLSConfig: tlsConfig,
	}

	logger.Info("Starting mTLS server...")
	return server.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile)
}

// startHTTPSServer starts server with HTTPS (for JWT auth).
func startHTTPSServer(cfg *config.AgentConfig, router *gin.Engine) error {
	logger.Info("Starting HTTPS server...")

	// For JWT, we still use TLS but without client cert verification
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to load server certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	}

	server := &http.Server{
		Addr:      cfg.ListenAddr,
		Handler:   router,
		TLSConfig: tlsConfig,
	}

	return server.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile)
}
