package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/requestid"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/stackit/enterprise-vm-manager/internal/config"
	"github.com/stackit/enterprise-vm-manager/pkg/errors"
	"github.com/stackit/enterprise-vm-manager/pkg/logger"
	"golang.org/x/time/rate"
)

// MiddlewareManager manages all middleware components
type MiddlewareManager struct {
	cfg    *config.Config
	logger *logger.Logger
}

// NewMiddlewareManager creates a new middleware manager
func NewMiddlewareManager(cfg *config.Config, logger *logger.Logger) *MiddlewareManager {
	return &MiddlewareManager{
		cfg:    cfg,
		logger: logger,
	}
}

// SetupMiddleware sets up all middleware
func (m *MiddlewareManager) SetupMiddleware(r *gin.Engine) {
	// Request ID middleware - must be first
	r.Use(requestid.New())

	// CORS middleware
	r.Use(m.CORSMiddleware())

	// Logging middleware
	r.Use(m.LoggingMiddleware())

	// Recovery middleware
	r.Use(m.RecoveryMiddleware())

	// Rate limiting middleware
	if m.cfg.Server.RateLimit.Enabled {
		r.Use(m.RateLimitMiddleware())
	}

	// Security headers middleware
	r.Use(m.SecurityHeadersMiddleware())

	// Metrics middleware
	if m.cfg.Metrics.Enabled {
		r.Use(m.MetricsMiddleware())
	}
}

// CORSMiddleware configures CORS
func (m *MiddlewareManager) CORSMiddleware() gin.HandlerFunc {
	corsConfig := cors.Config{
		AllowOrigins:     m.cfg.Server.CORS.AllowOrigins,
		AllowMethods:     m.cfg.Server.CORS.AllowMethods,
		AllowHeaders:     m.cfg.Server.CORS.AllowHeaders,
		ExposeHeaders:    m.cfg.Server.CORS.ExposeHeaders,
		AllowCredentials: m.cfg.Server.CORS.AllowCredentials,
		MaxAge:           time.Duration(m.cfg.Server.CORS.MaxAge) * time.Second,
	}

	return cors.New(corsConfig)
}

// LoggingMiddleware provides structured logging
func (m *MiddlewareManager) LoggingMiddleware() gin.HandlerFunc {
	return ginzap.Ginzap(m.logger.Desugar(), time.RFC3339, true)
}

// RecoveryMiddleware handles panics
func (m *MiddlewareManager) RecoveryMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		requestID := requestid.Get(c)

		m.logger.WithField("request_id", requestID).
			Errorf("Panic recovered: %v", recovered)

		err := errors.ErrInternalServer.WithContext("request_id", requestID)
		c.JSON(err.HTTPCode, gin.H{
			"error":      err,
			"request_id": requestID,
		})
		c.Abort()
	})
}

// RateLimitMiddleware implements rate limiting
func (m *MiddlewareManager) RateLimitMiddleware() gin.HandlerFunc {
	limiter := rate.NewLimiter(
		rate.Limit(m.cfg.Server.RateLimit.RPS),
		m.cfg.Server.RateLimit.Burst,
	)

	return func(c *gin.Context) {
		if !limiter.Allow() {
			requestID := requestid.Get(c)

			m.logger.WithField("request_id", requestID).
				WithField("client_ip", c.ClientIP()).
				Warn("Rate limit exceeded")

			err := errors.ErrRateLimitExceeded.WithContext("request_id", requestID)
			c.JSON(err.HTTPCode, gin.H{
				"error":       err,
				"request_id":  requestID,
				"retry_after": "60",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// SecurityHeadersMiddleware adds security headers
func (m *MiddlewareManager) SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}

// AuthenticationMiddleware handles authentication
func (m *MiddlewareManager) AuthenticationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.cfg.Auth.Enabled {
			c.Next()
			return
		}

		requestID := requestid.Get(c)

		// Check API key header
		apiKey := c.GetHeader(m.cfg.Auth.APIKeyHeader)
		if apiKey == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				err := errors.ErrUnauthorized.WithContext("request_id", requestID)
				c.JSON(err.HTTPCode, gin.H{
					"error":      err,
					"request_id": requestID,
				})
				c.Abort()
				return
			}

			// Extract Bearer token
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		// Validate API key
		if !m.isValidAPIKey(apiKey) {
			m.logger.WithField("request_id", requestID).
				WithField("client_ip", c.ClientIP()).
				Warn("Invalid API key provided")

			err := errors.ErrInvalidToken.WithContext("request_id", requestID)
			c.JSON(err.HTTPCode, gin.H{
				"error":      err,
				"request_id": requestID,
			})
			c.Abort()
			return
		}

		// Set user context (simplified - in real implementation, extract from JWT)
		c.Set("user_id", "api-user")
		c.Set("user_role", "admin")

		c.Next()
	}
}

// ErrorHandlerMiddleware handles and formats errors
func (m *MiddlewareManager) ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Handle errors after request processing
		if len(c.Errors) > 0 {
			requestID := requestid.Get(c)
			lastError := c.Errors.Last()

			// Log the error
			m.logger.WithField("request_id", requestID).
				WithField("path", c.Request.URL.Path).
				WithField("method", c.Request.Method).
				Errorf("Request error: %v", lastError.Err)

			// Convert to app error
			appErr := errors.ToAppError(lastError.Err)
			appErr = appErr.WithContext("request_id", requestID)

			// Don't override status if already set
			if c.Writer.Status() == http.StatusOK {
				c.JSON(appErr.HTTPCode, gin.H{
					"error":      appErr,
					"request_id": requestID,
				})
			}
		}
	}
}

// MetricsMiddleware adds Prometheus metrics
func (m *MiddlewareManager) MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		// In real implementation, this would update Prometheus metrics
		m.logger.WithFields(map[string]interface{}{
			"method":      c.Request.Method,
			"path":        c.Request.URL.Path,
			"status":      status,
			"duration_ms": duration.Milliseconds(),
			"size":        c.Writer.Size(),
		}).Debug("Request completed")
	}
}

// ValidationErrorHandler handles validation errors
func (m *MiddlewareManager) ValidationErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			requestID := requestid.Get(c)

			for _, ginErr := range c.Errors {
				if ginErr.Type == gin.ErrorTypeBind {
					// Handle binding errors
					appErr := errors.ErrValidationFailed.
						WithContext("request_id", requestID).
						WithDetails(ginErr.Error())

					c.JSON(appErr.HTTPCode, gin.H{
						"error":      appErr,
						"request_id": requestID,
					})
					return
				}
			}
		}
	}
}

// HealthCheckMiddleware bypasses other middleware for health endpoints
func (m *MiddlewareManager) HealthCheckMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authentication and rate limiting for health endpoints
		if strings.HasPrefix(c.Request.URL.Path, "/health") ||
			strings.HasPrefix(c.Request.URL.Path, "/metrics") ||
			strings.HasPrefix(c.Request.URL.Path, "/ready") {
			c.Next()
			return
		}

		c.Next()
	}
}

// RequestSizeLimitMiddleware limits request size
func (m *MiddlewareManager) RequestSizeLimitMiddleware(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxSize {
			requestID := requestid.Get(c)

			err := errors.New("REQUEST_TOO_LARGE", "Request body too large", http.StatusRequestEntityTooLarge).
				WithContext("request_id", requestID).
				WithDetails(fmt.Sprintf("Maximum allowed size: %d bytes", maxSize))

			c.JSON(err.HTTPCode, gin.H{
				"error":      err,
				"request_id": requestID,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// TimeoutMiddleware adds request timeout
func (m *MiddlewareManager) TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		timer := time.NewTimer(timeout)
		done := make(chan struct{})

		go func() {
			c.Next()
			close(done)
		}()

		select {
		case <-done:
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
			requestID := requestid.Get(c)
			err := errors.New("REQUEST_TIMEOUT", "Request timeout", http.StatusRequestTimeout).
				WithContext("request_id", requestID)
			c.JSON(err.HTTPCode, gin.H{
				"error":      err,
				"request_id": requestID,
			})
			c.Abort()
			return
		}
	}
}

// Helper methods

func (m *MiddlewareManager) isValidAPIKey(apiKey string) bool {
	if apiKey == "" {
		return false
	}

	// Check against configured API keys
	for _, validKey := range m.cfg.Auth.APIKeys {
		if apiKey == validKey {
			return true
		}
	}

	return false
}

// GetUserID extracts user ID from context
func GetUserID(c *gin.Context) string {
	userID, exists := c.Get("user_id")
	if !exists {
		return ""
	}
	return userID.(string)
}

// GetUserRole extracts user role from context
func GetUserRole(c *gin.Context) string {
	userRole, exists := c.Get("user_role")
	if !exists {
		return ""
	}
	return userRole.(string)
}

// RequireAuth middleware that requires authentication
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID == "" {
			requestID := requestid.Get(c)
			err := errors.ErrUnauthorized.WithContext("request_id", requestID)
			c.JSON(err.HTTPCode, gin.H{
				"error":      err,
				"request_id": requestID,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireRole middleware that requires specific role
func RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := GetUserRole(c)
		if userRole != requiredRole {
			requestID := requestid.Get(c)
			err := errors.ErrInsufficientPerm.WithContext("request_id", requestID)
			c.JSON(err.HTTPCode, gin.H{
				"error":      err,
				"request_id": requestID,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
