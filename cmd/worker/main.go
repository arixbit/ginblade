package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/arixbit/ginblade/config"
	app "github.com/arixbit/ginblade/internal"
	"github.com/arixbit/ginblade/internal/bootstrap"
	applog "github.com/arixbit/ginblade/pkg/log"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	config.LoadEnv("cmd/worker/.env")
	cfg := config.Load()
	if err := bootstrap.InitRuntime(cfg, "worker"); err != nil {
		fmt.Fprintf(os.Stderr, "init runtime: %v\n", err)
		return err
	}
	defer func() { _ = applog.Sync() }()

	registry, err := bootstrap.InitWorker(cfg)
	if err != nil {
		applog.L().Error("initialize worker", zap.Error(err))
		return err
	}
	defer func() {
		if err := registry.Close(); err != nil {
			applog.L().Warn("close registry", zap.Error(err))
		}
	}()

	worker, err := app.NewWorker(registry)
	if err != nil {
		applog.L().Error("assemble worker", zap.Error(err))
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	applog.L().Info("worker started", zap.String("component", "asynq"))
	if err := worker.Run(ctx); err != nil {
		applog.L().Error("worker runtime error", zap.Error(err))
		return err
	}
	applog.L().Info("worker shutdown completed")
	return nil
}
