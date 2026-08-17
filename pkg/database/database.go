package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	applog "github.com/arixbit/ginblade/pkg/log"
)

// DBManager holds the primary database connection.
type DBManager struct {
	primary *gorm.DB
}

// Config holds database connection settings.
type Config struct {
	DSN             string
	LogLevel        string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type poolSettings struct {
	maxIdleConns    int
	maxOpenConns    int
	connMaxLifetime time.Duration
	connMaxIdleTime time.Duration
}

// Init creates the primary database connection.
func Init(cfg Config) (*DBManager, error) {
	if strings.TrimSpace(cfg.DSN) == "" {
		return &DBManager{}, nil
	}

	pool := normalizePoolSettings(cfg)
	primary, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		Logger:                                   newGormLogger(cfg.LogLevel),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	if err := tunePool(primary, pool); err != nil {
		return nil, err
	}

	applog.L().Info("postgres connected")
	return &DBManager{primary: primary}, nil
}

// DB returns the GORM instance.
func (m *DBManager) DB() *gorm.DB {
	if m == nil {
		return nil
	}
	return m.primary
}

// Ping verifies that the database is reachable.
func (m *DBManager) Ping(ctx context.Context) error {
	if m == nil || m.primary == nil {
		return fmt.Errorf("database is not configured")
	}
	sqlDB, err := m.primary.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}

// Close closes the database connection pool.
func (m *DBManager) Close() error {
	if m == nil || m.primary == nil {
		return nil
	}
	sqlDB, err := m.primary.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// NewTestManager creates a DBManager from an existing GORM instance.
func NewTestManager(db *gorm.DB) *DBManager {
	return &DBManager{primary: db}
}

func tunePool(db *gorm.DB, pool poolSettings) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxIdleConns(pool.maxIdleConns)
	sqlDB.SetMaxOpenConns(pool.maxOpenConns)
	sqlDB.SetConnMaxLifetime(pool.connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(pool.connMaxIdleTime)
	return nil
}

func normalizePoolSettings(cfg Config) poolSettings {
	settings := poolSettings{
		maxIdleConns:    cfg.MaxIdleConns,
		maxOpenConns:    cfg.MaxOpenConns,
		connMaxLifetime: cfg.ConnMaxLifetime,
		connMaxIdleTime: cfg.ConnMaxIdleTime,
	}
	if settings.maxIdleConns <= 0 {
		settings.maxIdleConns = 15
	}
	if settings.maxOpenConns <= 0 {
		settings.maxOpenConns = 30
	}
	if settings.connMaxLifetime <= 0 {
		settings.connMaxLifetime = 30 * time.Minute
	}
	if settings.connMaxIdleTime <= 0 {
		settings.connMaxIdleTime = 5 * time.Minute
	}
	return settings
}

func parseLogLevel(level string) logger.LogLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "info":
		return logger.Info
	case "warn", "warning", "":
		return logger.Warn
	default:
		applog.L().Warn("unknown gorm log level; falling back to warn", zap.String("level", level))
		return logger.Warn
	}
}
