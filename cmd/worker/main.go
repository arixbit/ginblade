package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/arixbit/ginblade/config"
	app "github.com/arixbit/ginblade/internal"
	"github.com/arixbit/ginblade/internal/bootstrap"
	applog "github.com/arixbit/ginblade/pkg/log"
)

func main() {
	config.LoadEnv("cmd/worker/.env")
	cfg := config.Load()
	if err := bootstrap.InitRuntime(cfg, "worker"); err != nil {
		panic(fmt.Sprintf("init runtime: %v", err))
	}
	defer func() { _ = applog.Sync() }()

	registry, err := bootstrap.InitWorker(cfg)
	if err != nil {
		applog.L().Fatal("initialize worker", zap.Error(err))
	}
	defer func() {
		if err := registry.Close(); err != nil {
			applog.L().Warn("close registry", zap.Error(err))
		}
	}()

	worker, err := app.NewWorker(registry)
	if err != nil {
		applog.L().Fatal("assemble worker", zap.Error(err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	applog.L().Info("worker started", zap.String("component", "asynq"))
	if err := worker.Run(ctx); err != nil {
		applog.L().Fatal("worker runtime error", zap.Error(err))
	}
	applog.L().Info("worker shutdown completed")
}
