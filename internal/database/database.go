package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/stackit/enterprise-vm-manager/internal/config"
	"github.com/stackit/enterprise-vm-manager/internal/models"
	"github.com/stackit/enterprise-vm-manager/pkg/logger"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Database represents the database connection and operations
type Database struct {
	DB     *gorm.DB
	config *config.DatabaseConfig
	logger *logger.Logger
}

// New creates a new database connection
func New(cfg *config.DatabaseConfig, log *logger.Logger) (*Database, error) {
	// Configure GORM logger (use default writer)
	gormLog := gormLogger.Default.LogMode(gormLogger.Info)

	// Open database connection
	db, err := gorm.Open(gormpostgres.Open(cfg.DSN()), &gorm.Config{
		Logger: gormLog,
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "",
			SingularTable: false,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	database := &Database{
		DB:     db,
		config: cfg,
		logger: log.WithComponent("database"),
	}

	return database, nil
}

// Connect establishes database connection with retries
func Connect(cfg *config.DatabaseConfig, log *logger.Logger) (*Database, error) {
	var database *Database
	var err error

	// Retry connection with exponential backoff
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		database, err = New(cfg, log)
		if err == nil {
			break
		}

		if i == maxRetries-1 {
			return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
		}

		log.Warnf("Database connection attempt %d/%d failed: %v", i+1, maxRetries, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := database.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Database connection established successfully")
	return database, nil
}

// Ping tests the database connection
func (d *Database) Ping(ctx context.Context) error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Close closes the database connection
func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// AutoMigrate runs GORM auto-migrations
func (d *Database) AutoMigrate() error {
	d.logger.Info("Running database auto-migrations...")

	err := d.DB.AutoMigrate(
		&models.VM{},
	)
	if err != nil {
		return fmt.Errorf("failed to run auto-migrations: %w", err)
	}

	d.logger.Info("Database auto-migrations completed successfully")
	return nil
}

// RunMigrations runs database migrations using golang-migrate
func (d *Database) RunMigrations() error {
	if d.config.MigrationsPath == "" {
		d.logger.Info("No migrations path configured, skipping migrations")
		return nil
	}

	d.logger.Info("Running database migrations...")

	// Open a separate sql.DB for migrations to avoid interfering with GORM connection
	sqlDB, err := sql.Open("postgres", d.config.DSN())
	if err != nil {
		return fmt.Errorf("failed to open sql.DB for migrations: %w", err)
	}
	defer sqlDB.Close()

	// Create postgres driver for migrate
	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %w", err)
	}

	// Create migrator
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", d.config.MigrationsPath),
		"postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer m.Close()

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	d.logger.Info("Database migrations completed successfully")
	return nil
}

// Seed populates the database with initial data
func (d *Database) Seed() error {
	d.logger.Info("Seeding database with initial data...")

	// Check if data already exists
	var count int64
	if err := d.DB.Model(&models.VM{}).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check existing data: %w", err)
	}

	if count > 0 {
		d.logger.Info("Database already contains data, skipping seeding")
		return nil
	}

	// Create sample VMs
	sampleVMs := d.createSampleVMs()

	// Insert sample data in transaction
	err := d.DB.Transaction(func(tx *gorm.DB) error {
		for _, vm := range sampleVMs {
			if err := tx.Create(vm).Error; err != nil {
				return fmt.Errorf("failed to create sample VM %s: %w", vm.Name, err)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to seed database: %w", err)
	}

	d.logger.Infof("Database seeded successfully with %d sample VMs", len(sampleVMs))
	return nil
}

// createSampleVMs creates sample VM data
func (d *Database) createSampleVMs() []*models.VM {
	now := time.Now()

	return []*models.VM{
		{
			Name:        "web-server-01",
			Description: "Production web server for main application",
			Spec: models.VMSpec{
				CPUCores:    4,
				RAMMb:       8192,
				DiskGb:      100,
				ImageName:   "ubuntu:22.04",
				NetworkType: models.NetworkTypeNAT,
			},
			Status:    models.VMStatusRunning,
			NodeID:    "node-01",
			CreatedBy: "system",
			UpdatedBy: "system",
			StartedAt: &now,
			Stats: models.VMStats{
				CPUUsagePercent:  45.2,
				RAMUsagePercent:  67.8,
				DiskUsagePercent: 23.1,
				UptimeSeconds:    3600,
				LastStatsUpdate:  now,
			},
		},
		{
			Name:        "database-primary",
			Description: "Primary PostgreSQL database server",
			Spec: models.VMSpec{
				CPUCores:    8,
				RAMMb:       16384,
				DiskGb:      500,
				ImageName:   "postgres:15",
				NetworkType: models.NetworkTypeBridge,
			},
			Status:    models.VMStatusRunning,
			NodeID:    "node-02",
			CreatedBy: "system",
			UpdatedBy: "system",
			StartedAt: &now,
			Stats: models.VMStats{
				CPUUsagePercent:  25.6,
				RAMUsagePercent:  78.9,
				DiskUsagePercent: 45.3,
				UptimeSeconds:    7200,
				LastStatsUpdate:  now,
			},
		},
		{
			Name:        "cache-server",
			Description: "Redis cache server",
			Spec: models.VMSpec{
				CPUCores:    2,
				RAMMb:       4096,
				DiskGb:      50,
				ImageName:   "redis:7",
				NetworkType: models.NetworkTypeNAT,
			},
			Status:    models.VMStatusRunning,
			NodeID:    "node-01",
			CreatedBy: "system",
			UpdatedBy: "system",
			StartedAt: &now,
			Stats: models.VMStats{
				CPUUsagePercent:  12.3,
				RAMUsagePercent:  34.5,
				DiskUsagePercent: 15.2,
				UptimeSeconds:    5400,
				LastStatsUpdate:  now,
			},
		},
		{
			Name:        "api-server-01",
			Description: "REST API server instance 1",
			Spec: models.VMSpec{
				CPUCores:    2,
				RAMMb:       4096,
				DiskGb:      50,
				ImageName:   "golang:1.21-alpine",
				NetworkType: models.NetworkTypeNAT,
			},
			Status:    models.VMStatusStopped,
			NodeID:    "node-03",
			CreatedBy: "system",
			UpdatedBy: "system",
		},
		{
			Name:        "monitoring-server",
			Description: "Prometheus monitoring server",
			Spec: models.VMSpec{
				CPUCores:    4,
				RAMMb:       8192,
				DiskGb:      200,
				ImageName:   "prom/prometheus:latest",
				NetworkType: models.NetworkTypeBridge,
			},
			Status:    models.VMStatusStopped,
			NodeID:    "node-04",
			CreatedBy: "system",
			UpdatedBy: "system",
		},
		{
			Name:        "test-environment",
			Description: "Development and testing environment",
			Spec: models.VMSpec{
				CPUCores:    1,
				RAMMb:       2048,
				DiskGb:      30,
				ImageName:   "alpine:latest",
				NetworkType: models.NetworkTypeNAT,
			},
			Status:    models.VMStatusStopped,
			NodeID:    "node-01",
			CreatedBy: "developer",
			UpdatedBy: "developer",
		},
	}
}

// HealthCheck checks database health
func (d *Database) HealthCheck(ctx context.Context) error {
	// Check connection
	if err := d.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check if we can query
	var result int
	if err := d.DB.WithContext(ctx).Raw("SELECT 1").Scan(&result).Error; err != nil {
		return fmt.Errorf("database query failed: %w", err)
	}

	return nil
}

// GetStats returns database statistics
func (d *Database) GetStats() (*DatabaseStats, error) {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return nil, err
	}

	stats := sqlDB.Stats()

	return &DatabaseStats{
		OpenConnections:   stats.OpenConnections,
		InUse:             stats.InUse,
		Idle:              stats.Idle,
		WaitCount:         stats.WaitCount,
		WaitDuration:      stats.WaitDuration,
		MaxIdleClosed:     stats.MaxIdleClosed,
		MaxLifetimeClosed: stats.MaxLifetimeClosed,
	}, nil
}

// DatabaseStats represents database connection statistics
type DatabaseStats struct {
	OpenConnections   int           `json:"open_connections"`
	InUse             int           `json:"in_use"`
	Idle              int           `json:"idle"`
	WaitCount         int64         `json:"wait_count"`
	WaitDuration      time.Duration `json:"wait_duration"`
	MaxIdleClosed     int64         `json:"max_idle_closed"`
	MaxLifetimeClosed int64         `json:"max_lifetime_closed"`
}

// Truncate removes all data from tables (for testing)
func (d *Database) Truncate() error {
	d.logger.Warn("Truncating all database tables")

	tables := []string{
		"virtual_machines",
	}

	return d.DB.Transaction(func(tx *gorm.DB) error {
		// Disable foreign key checks
		if err := tx.Exec("SET session_replication_role = replica").Error; err != nil {
			return err
		}

		// Truncate tables
		for _, table := range tables {
			if err := tx.Exec(fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table)).Error; err != nil {
				return fmt.Errorf("failed to truncate table %s: %w", table, err)
			}
		}

		// Re-enable foreign key checks
		if err := tx.Exec("SET session_replication_role = DEFAULT").Error; err != nil {
			return err
		}

		return nil
	})
}

// BeginTx starts a new transaction
func (d *Database) BeginTx(ctx context.Context) *gorm.DB {
	return d.DB.WithContext(ctx).Begin()
}

// WithContext returns a new DB instance with context
func (d *Database) WithContext(ctx context.Context) *gorm.DB {
	return d.DB.WithContext(ctx)
}
