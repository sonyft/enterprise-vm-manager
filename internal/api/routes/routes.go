package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/stackit/enterprise-vm-manager/internal/api/handlers"
	"github.com/stackit/enterprise-vm-manager/internal/api/middleware"
	"github.com/stackit/enterprise-vm-manager/internal/config"
	"github.com/stackit/enterprise-vm-manager/pkg/logger"
	"github.com/swaggo/files"
	"github.com/swaggo/gin-swagger"
)

// Router manages API routes
type Router struct {
	cfg        *config.Config
	logger     *logger.Logger
	vmHandler  *handlers.VMHandler
	middleware *middleware.MiddlewareManager
}

// NewRouter creates a new router
func NewRouter(
	cfg *config.Config,
	logger *logger.Logger,
	vmHandler *handlers.VMHandler,
	middlewareManager *middleware.MiddlewareManager,
) *Router {
	return &Router{
		cfg:        cfg,
		logger:     logger,
		vmHandler:  vmHandler,
		middleware: middlewareManager,
	}
}

// SetupRoutes sets up all API routes
func (r *Router) SetupRoutes(engine *gin.Engine) {
	// Setup global middleware
	r.middleware.SetupMiddleware(engine)

	// Health and system routes (no auth required)
	r.setupSystemRoutes(engine)

	// API routes
	r.setupAPIRoutes(engine)

	// Documentation routes
	r.setupDocumentationRoutes(engine)
}

// setupSystemRoutes sets up system/health routes
func (r *Router) setupSystemRoutes(engine *gin.Engine) {
	// Health check endpoint
	engine.GET("/health", r.healthCheck)

	// Readiness probe
	engine.GET("/ready", r.readinessCheck)

	// Liveness probe
	engine.GET("/live", r.livenessCheck)

	// Version endpoint
	engine.GET("/version", r.versionInfo)

	// Metrics endpoint (if enabled)
	if r.cfg.Metrics.Enabled {
		engine.GET(r.cfg.Metrics.Path, r.metricsHandler)
	}
}

// setupAPIRoutes sets up main API routes
func (r *Router) setupAPIRoutes(engine *gin.Engine) {
	// API v1 group
	v1 := engine.Group("/api/v1")

	// Apply authentication middleware if enabled
	if r.cfg.Auth.Enabled {
		v1.Use(r.middleware.AuthenticationMiddleware())
	}

	// Apply error handling middleware
	v1.Use(r.middleware.ErrorHandlerMiddleware())
	v1.Use(r.middleware.ValidationErrorHandler())

	// VM management routes
	r.setupVMRoutes(v1)

	// System statistics routes
	r.setupStatsRoutes(v1)
}

// setupVMRoutes sets up VM-related routes
func (r *Router) setupVMRoutes(rg *gin.RouterGroup) {
	vms := rg.Group("/vms")

	// CRUD operations
	vms.POST("", r.vmHandler.CreateVM)
	vms.GET("", r.vmHandler.ListVMs)
	vms.GET("/:id", r.vmHandler.GetVM)
	vms.PUT("/:id", r.vmHandler.UpdateVM)
	vms.DELETE("/:id", r.vmHandler.DeleteVM)

	// State management operations
	vms.POST("/:id/start", r.vmHandler.StartVM)
	vms.POST("/:id/stop", r.vmHandler.StopVM)
	vms.POST("/:id/restart", r.vmHandler.RestartVM)
	vms.POST("/:id/suspend", r.vmHandler.SuspendVM)
	vms.POST("/:id/resume", r.vmHandler.ResumeVM)

	// Statistics and monitoring
	vms.GET("/:id/stats", r.vmHandler.GetVMStats)
}

// setupStatsRoutes sets up statistics routes
func (r *Router) setupStatsRoutes(rg *gin.RouterGroup) {
	stats := rg.Group("/stats")

	stats.GET("/summary", r.vmHandler.GetResourceSummary)
}

// setupDocumentationRoutes sets up API documentation
func (r *Router) setupDocumentationRoutes(engine *gin.Engine) {
	// Swagger documentation
	if r.cfg.IsDevelopment() {
		engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
		engine.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

		// Redirect /docs to /swagger
		engine.GET("/docs", func(c *gin.Context) {
			c.Redirect(302, "/swagger/index.html")
		})
	}
}

// System endpoint handlers

// healthCheck returns the health status of the application
func (r *Router) healthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":    "ok",
		"service":   "enterprise-vm-manager",
		"version":   "1.0.0",
		"timestamp": "2025-10-15T22:00:00Z",
	})
}

// readinessCheck checks if the application is ready to serve traffic
func (r *Router) readinessCheck(c *gin.Context) {
	// In a real implementation, this would check:
	// - Database connectivity
	// - External service dependencies
	// - Cache availability

	c.JSON(200, gin.H{
		"status": "ready",
		"checks": gin.H{
			"database": "ok",
			"cache":    "ok",
		},
	})
}

// livenessCheck checks if the application is alive
func (r *Router) livenessCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"status": "alive",
	})
}

// versionInfo returns version information
func (r *Router) versionInfo(c *gin.Context) {
	c.JSON(200, gin.H{
		"service":     "enterprise-vm-manager",
		"version":     "1.0.0",
		"build_time":  "2025-10-15T22:00:00Z",
		"git_commit":  "abc123def",
		"go_version":  "1.21.0",
		"environment": r.cfg.Server.Mode,
	})
}

// metricsHandler handles Prometheus metrics
func (r *Router) metricsHandler(c *gin.Context) {
	// In a real implementation, this would serve Prometheus metrics
	c.String(200, `# HELP vm_manager_requests_total Total number of requests
# TYPE vm_manager_requests_total counter
vm_manager_requests_total 42

# HELP vm_manager_vms_total Total number of VMs
# TYPE vm_manager_vms_total gauge
vm_manager_vms_total 10

# HELP vm_manager_vms_running Number of running VMs
# TYPE vm_manager_vms_running gauge
vm_manager_vms_running 7
`)
}

// RouteInfo represents route information for debugging
type RouteInfo struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler"`
}

// GetRoutes returns all registered routes (for debugging)
func (r *Router) GetRoutes(engine *gin.Engine) []RouteInfo {
	routes := engine.Routes()
	routeInfos := make([]RouteInfo, len(routes))

	for i, route := range routes {
		routeInfos[i] = RouteInfo{
			Method:  route.Method,
			Path:    route.Path,
			Handler: route.Handler,
		}
	}

	return routeInfos
}

// PrintRoutes prints all routes for debugging
func (r *Router) PrintRoutes(engine *gin.Engine) {
	routes := r.GetRoutes(engine)

	r.logger.Info("Registered routes:")
	for _, route := range routes {
		r.logger.Infof("  %s %s", route.Method, route.Path)
	}
}
