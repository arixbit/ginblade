package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/arixbit/ginblade/internal/task"
	"github.com/arixbit/ginblade/internal/taskqueue"
	"github.com/arixbit/ginblade/pkg/cache"
	applog "github.com/arixbit/ginblade/pkg/log"
)

// Deps collects shared dependencies for async task handlers.
type Deps struct {
	DB    *gorm.DB
	Cache *cache.Client
	RDB   *redis.Client
	Queue *taskqueue.Queue
}

// HandleExampleTask processes the example async task.
func (d *Deps) HandleExampleTask(ctx context.Context, t *asynq.Task) error {
	var p task.ExamplePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal example payload: %w", err)
	}

	applog.FromContext(ctx).Info("example task executed",
		zap.String("name", p.Name),
		zap.Bool("db_available", d != nil && d.DB != nil),
	)
	return nil
}

// RegisterHandlers registers all async task handlers on mux.
func RegisterHandlers(mux *asynq.ServeMux, deps *Deps) {
	if mux == nil {
		return
	}
	registerTraceMiddleware(mux)
	if deps == nil {
		deps = &Deps{}
	}
	mux.HandleFunc(task.TypeExampleTask, deps.HandleExampleTask)
}
