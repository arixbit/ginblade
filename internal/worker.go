package app

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"golang.org/x/sync/errgroup"

	"github.com/arixbit/ginblade/internal/bootstrap"
	"github.com/arixbit/ginblade/internal/worker"
)

// Worker owns the async task runtime created from application dependencies.
type Worker struct {
	server *asynq.Server
	mux    *asynq.ServeMux
}

// NewWorker wires async task handlers and the worker runtime.
func NewWorker(reg *bootstrap.Registry) (*Worker, error) {
	if err := validateWorkerRegistry(reg); err != nil {
		return nil, err
	}

	deps := buildWorkerDeps(reg)
	mux := asynq.NewServeMux()
	worker.RegisterHandlers(mux, deps)

	return &Worker{
		server: worker.NewServer(worker.NewRedisOpt(reg.Cfg.Redis.Addr, reg.Cfg.Redis.Password, reg.Cfg.Redis.QueueDB)),
		mux:    mux,
	}, nil
}

// Run starts the task server until the context is canceled.
func (w *Worker) Run(ctx context.Context) error {
	if w == nil || w.server == nil || w.mux == nil {
		return errNilWorker
	}
	if ctx == nil {
		ctx = context.Background()
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		if err := w.server.Run(w.mux); err != nil {
			return fmt.Errorf("run worker server: %w", err)
		}
		return nil
	})
	group.Go(func() error {
		<-groupCtx.Done()
		w.server.Shutdown()
		return nil
	})

	return group.Wait()
}

func validateWorkerRegistry(reg *bootstrap.Registry) error {
	switch {
	case reg == nil:
		return errNilRegistry
	case reg.Cfg == nil:
		return errNilConfig
	case reg.Cfg.Redis.Addr == "":
		return fmt.Errorf("app: missing redis address")
	default:
		return nil
	}
}

func buildWorkerDeps(reg *bootstrap.Registry) *worker.Deps {
	deps := &worker.Deps{
		Cache: reg.Cache,
		Queue: reg.Queue,
	}
	if reg.DB != nil {
		deps.DB = reg.DB.DB()
	}
	return deps
}
