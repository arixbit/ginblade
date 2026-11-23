package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/arixbit/ginblade/config"
	"github.com/arixbit/ginblade/internal/bootstrap"
	"github.com/arixbit/ginblade/internal/model"
	"github.com/arixbit/ginblade/pkg/database"
	applog "github.com/arixbit/ginblade/pkg/log"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	config.LoadEnv("cmd/migrate/.env")
	cfg := config.Load()
	if err := bootstrap.InitRuntime(cfg, "migrate"); err != nil {
		fmt.Fprintf(os.Stderr, "init runtime: %v\n", err)
		return err
	}
	defer func() { _ = applog.Sync() }()

	dbMgr, err := database.Init(database.Config{
		DSN:             cfg.Postgres.DSN,
		LogLevel:        cfg.Postgres.LogLevel,
		MaxIdleConns:    cfg.Postgres.MaxIdleConns,
		MaxOpenConns:    cfg.Postgres.MaxOpenConns,
		ConnMaxLifetime: cfg.Postgres.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Postgres.ConnMaxIdleTime,
	})
	if err != nil {
		applog.L().Error("initialize database", zap.Error(err))
		return err
	}
	defer func() {
		if err := dbMgr.Close(); err != nil {
			applog.L().Warn("close database", zap.Error(err))
		}
	}()
	if dbMgr.DB() == nil {
		err := errors.New("database is not configured")
		applog.L().Error("database is not configured")
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := dbMgr.DB().WithContext(ctx).AutoMigrate(&model.Example{}); err != nil {
		applog.L().Error("run migrations", zap.Error(err))
		return err
	}
	applog.L().Info("migrations completed")
	return nil
}
