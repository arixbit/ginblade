package main

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/arixbit/ginblade/config"
	app "github.com/arixbit/ginblade/internal"
	"github.com/arixbit/ginblade/internal/bootstrap"
	applog "github.com/arixbit/ginblade/pkg/log"
)

const (
	gracefulShutdownTimeout = 10 * time.Second
	serverExitWaitTimeout   = 2 * time.Second
)

var errServerExitTimeout = errors.New("web server exit wait timeout")

type httpServerLifecycle interface {
	Shutdown(context.Context) error
	Close() error
}

func main() {
	config.LoadEnv("cmd/api/.env")
	cfg := config.Load()
	if err := bootstrap.InitRuntime(cfg, "api"); err != nil {
		panic(fmt.Sprintf("init runtime: %v", err))
	}
	defer func() { _ = applog.Sync() }()

	registry, err := bootstrap.InitAPI(cfg)
	if err != nil {
		applog.L().Fatal("initialize application", zap.Error(err))
	}
	defer func() {
		if err := registry.Close(); err != nil {
			applog.L().Warn("close registry", zap.Error(err))
		}
	}()

	server, err := app.NewServer(registry)
	if err != nil {
		applog.L().Fatal("assemble http server", zap.Error(err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run()
	}()

	applog.L().Info("starting web server", zap.String("addr", cfg.Server.Port))

	select {
	case err := <-errCh:
		if err != nil {
			applog.L().Fatal("web server error", zap.Error(err))
		}
		return
	case <-ctx.Done():
	}

	if _, err := shutdownWebServer(server, errCh, gracefulShutdownTimeout, serverExitWaitTimeout); err != nil {
		applog.L().Error("web server shutdown failed", zap.Error(err))
	}
}

func shutdownWebServer(server httpServerLifecycle, errCh <-chan error, shutdownTimeout, waitTimeout time.Duration) (bool, error) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	forced := false
	if err := server.Shutdown(shutdownCtx); err != nil {
		forced = true
		applog.L().Error("web server shutdown error", zap.Error(err))
		if closeErr := server.Close(); closeErr != nil {
			applog.L().Error("web server force close error", zap.Error(closeErr))
		}
	}

	exitErr := waitForServerExit(errCh, waitTimeout)
	if exitErr != nil {
		if errors.Is(exitErr, errServerExitTimeout) {
			applog.L().Warn("web server exit wait timed out", zap.Error(exitErr))
		} else {
			applog.L().Error("web server exited with error", zap.Error(exitErr))
		}
		return forced, exitErr
	}

	if forced {
		applog.L().Info("forced shutdown completed")
	} else {
		applog.L().Info("graceful shutdown completed")
	}
	return forced, nil
}

func waitForServerExit(errCh <-chan error, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-errCh:
		return err
	case <-timer.C:
		return fmt.Errorf("%w after %s", errServerExitTimeout, timeout)
	}
}
