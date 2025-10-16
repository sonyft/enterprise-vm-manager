package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stackit/enterprise-vm-manager/internal/api/handlers"
	"github.com/stackit/enterprise-vm-manager/internal/api/middleware"
	"github.com/stackit/enterprise-vm-manager/internal/api/routes"
	"github.com/stackit/enterprise-vm-manager/internal/config"
	"github.com/stackit/enterprise-vm-manager/internal/database"
	"github.com/stackit/enterprise-vm-manager/internal/repositories"
	"github.com/stackit/enterprise-vm-manager/internal/services"
	"github.com/stackit/enterprise-vm-manager/pkg/logger"
)

// Build-time variables (set via ldflags)
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

// Application represents the main application
type Application struct {
	cfg    *config.Config
	logger *logger.Logger
	db     *database.Database
	server *http.Server
	router *routes.Router

	// Services
	vmService services.VMService

	// Repositories
	vmRepo repositories.VMRepository

	// Handlers
	vmHandler *handlers.VMHandler

	// Middleware
	middleware *middleware.MiddlewareManager
}

// @title VM Manager API
// @version 1.0
// @description Enterprise Virtual Machine Management API
// @termsOfService https://github.com/stackit/enterprise-vm-manager

// @contact.name API Support
// @contact.url https://github.com/stackit/enterprise-vm-manager/issues
// @contact.email support@stackit.example.com

// @license.name MIT
// @license.url https://github.com/stackit/enterprise-vm-manager/blob/main/LICENSE

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter 'Bearer' [space] and then your token

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description Enter your API key

func main() {
	// Parse command line flags
	var configPath = flag.String("config", "", "Path to configuration file")
	flag.Parse()

	// Print version information
	fmt.Printf("VM Manager API %s (built %s, commit %s)\n", version, buildTime, gitCommit)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logConfig := logger.Config{
		Level:      cfg.Logging.Level,
		Format:     cfg.Logging.Format,
		Output:     cfg.Logging.Output,
		Filename:   cfg.Logging.Filename,
		MaxSize:    cfg.Logging.MaxSize,
		MaxBackups: cfg.Logging.MaxBackups,
		MaxAge:     cfg.Logging.MaxAge,
		Compress:   cfg.Logging.Compress,
	}

	log, err := logger.New(logConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Close()

	// Set global logger
	if err := logger.Init(logConfig); err != nil {
		log.Fatalf("Failed to set global logger: %v", err)
	}

	log.Infof("Starting VM Manager API %s", version)
	log.Infof("Environment: %s", cfg.Server.Mode)
	log.Infof("Configuration loaded successfully")

	// Create application instance
	app := &Application{
		cfg:    cfg,
		logger: log,
	}

	// Initialize application components
	if err := app.initializeComponents(); err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Start HTTP server
	if err := app.start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Wait for shutdown signal
	app.waitForShutdown()

	// Graceful shutdown
	if err := app.shutdown(); err != nil {
		log.Errorf("Error during shutdown: %v", err)
	}

	log.Info("VM Manager API stopped successfully")
}

// initializeComponents initializes all application components
func (app *Application) initializeComponents() error {
	var err error

	// Initialize database
	app.logger.Info("Connecting to database...")
	app.db, err = database.Connect(&app.cfg.Database, app.logger)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run database migrations from SQL files
	if err := app.db.RunMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Seed database in development mode
	if app.cfg.IsDevelopment() {
		if err := app.db.Seed(); err != nil {
			app.logger.Warnf("Failed to seed database: %v", err)
		}
	}

	// Initialize repositories
	app.vmRepo = repositories.NewVMRepository(app.db.DB)

	// Initialize services
	app.vmService = services.NewVMService(app.vmRepo, app.cfg, app.logger)

	// Initialize handlers
	app.vmHandler = handlers.NewVMHandler(app.vmService, app.logger)

	// Initialize middleware
	app.middleware = middleware.NewMiddlewareManager(app.cfg, app.logger)

	// Initialize router
	app.router = routes.NewRouter(app.cfg, app.logger, app.vmHandler, app.middleware)

	app.logger.Info("All components initialized successfully")
	return nil
}

// start starts the HTTP server
func (app *Application) start() error {
	// Set Gin mode
	gin.SetMode(app.cfg.Server.Mode)

	// Create Gin engine
	engine := gin.New()

	// Setup routes
	app.router.SetupRoutes(engine)

	// Print routes in development mode
	if app.cfg.IsDevelopment() {
		app.router.PrintRoutes(engine)
	}

	// Create HTTP server
	app.server = &http.Server{
		Addr:         app.cfg.Address(),
		Handler:      engine,
		ReadTimeout:  app.cfg.Server.ReadTimeout,
		WriteTimeout: app.cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		app.logger.Infof("Starting HTTP server on %s", app.cfg.Address())
		app.logger.Infof("Swagger documentation: http://%s/swagger/index.html", app.cfg.Address())
		app.logger.Infof("Health check: http://%s/health", app.cfg.Address())

		if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.logger.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	app.logger.Info("VM Manager API started successfully")
	return nil
}

// waitForShutdown waits for termination signals
func (app *Application) waitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	app.logger.Infof("Received signal %v, initiating graceful shutdown...", sig)
}

// shutdown performs graceful shutdown
func (app *Application) shutdown() error {
	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), app.cfg.Server.ShutdownTimeout)
	defer cancel()

	// Shutdown HTTP server
	app.logger.Info("Shutting down HTTP server...")
	if err := app.server.Shutdown(ctx); err != nil {
		app.logger.Errorf("HTTP server shutdown error: %v", err)
		return err
	}
	app.logger.Info("HTTP server stopped")

	// Close database connection
	if app.db != nil {
		app.logger.Info("Closing database connection...")
		if err := app.db.Close(); err != nil {
			app.logger.Errorf("Database close error: %v", err)
			return err
		}
		app.logger.Info("Database connection closed")
	}

	// Close logger
	if err := app.logger.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Logger close error: %v\n", err)
		return err
	}

	return nil
}

// healthCheck performs application health check
func (app *Application) healthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check database health
	if err := app.db.HealthCheck(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	return nil
}
