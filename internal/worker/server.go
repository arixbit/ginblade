package worker

import (
	"context"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"github.com/arixbit/ginblade/internal/task"
	applog "github.com/arixbit/ginblade/pkg/log"
)

// NewRedisOpt creates the Redis connection config used by asynq.
func NewRedisOpt(addr, password string, db int) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     addr,
		Password: password,
		DB:       db,
	}
}

// NewServer creates an asynq worker server.
func NewServer(redisOpt asynq.RedisClientOpt) *asynq.Server {
	return asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 10,
		Queues: map[string]int{
			"critical": 6,
			"default":  3,
			"low":      1,
		},
		RetryDelayFunc: exampleRetryDelay,
		ErrorHandler:   asynq.ErrorHandlerFunc(logTaskFailed),
	})
}

// exampleRetryDelay returns the backoff delay for a failed task: 5s × 2^n,
// capped at one hour. It is a named function so it can be unit tested.
func exampleRetryDelay(n int, e error, t *asynq.Task) time.Duration {
	delay := time.Duration(1<<uint(n)) * 5 * time.Second
	if delay > time.Hour {
		delay = time.Hour
	}
	logger := applog.L()
	if traceID := task.TraceIDFromPayload(t.Payload()); traceID != "" {
		logger = logger.With(zap.String("trace_id", traceID))
	}
	logger.Warn("asynq task will retry",
		zap.String("task", t.Type()),
		zap.Int("attempt", n+1),
		zap.Duration("retry_after", delay),
		zap.Error(e),
	)
	return delay
}

type taskRuntimeMetadata struct {
	TaskID             string
	Queue              string
	RetryCount         int
	RetryCountRecorded bool
}

type taskRuntimeMetadataProvider func(context.Context) taskRuntimeMetadata

func taskRuntimeMetadataFromContext(ctx context.Context) taskRuntimeMetadata {
	meta := taskRuntimeMetadata{}
	if taskID, ok := asynq.GetTaskID(ctx); ok {
		meta.TaskID = taskID
	}
	if queue, ok := asynq.GetQueueName(ctx); ok {
		meta.Queue = queue
	}
	if retryCount, ok := asynq.GetRetryCount(ctx); ok {
		meta.RetryCount = retryCount
		meta.RetryCountRecorded = true
	}
	return meta
}

// TraceMiddleware restores trace_id from task payloads and logs task lifecycle events.
func TraceMiddleware(next asynq.Handler) asynq.Handler {
	return traceMiddleware(next, taskRuntimeMetadataFromContext)
}

func traceMiddleware(next asynq.Handler, metaProvider taskRuntimeMetadataProvider) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		ctx, fields := taskLogContext(ctx, t, metaProvider(ctx))
		applog.FromContext(ctx).Info("asynq task started", fields...)
		err := next.ProcessTask(ctx, t)
		if err != nil {
			return err
		}
		applog.FromContext(ctx).Info("asynq task finished", fields...)
		return nil
	})
}

func logTaskFailed(ctx context.Context, t *asynq.Task, err error) {
	ctx, fields := taskLogContext(ctx, t, taskRuntimeMetadataFromContext(ctx))
	fields = append(fields, zap.Error(err))
	applog.FromContext(ctx).Error("asynq task failed", fields...)
}

func taskLogContext(ctx context.Context, t *asynq.Task, meta taskRuntimeMetadata) (context.Context, []zap.Field) {
	traceID := task.TraceIDFromPayload(t.Payload())
	traceSource := "request"
	if traceID == "" {
		traceID = applog.NewTraceID("asynq", meta.TaskID)
		traceSource = "asynq_task"
	}
	ctx = applog.EnsureTraceID(ctx, traceID)

	fields := []zap.Field{
		zap.String("task", t.Type()),
		zap.String("trace_source", traceSource),
	}
	if strings.TrimSpace(meta.TaskID) != "" {
		fields = append(fields, zap.String("task_id", meta.TaskID))
	}
	if strings.TrimSpace(meta.Queue) != "" {
		fields = append(fields, zap.String("queue", meta.Queue))
	}
	if meta.RetryCountRecorded {
		fields = append(fields, zap.Int("retry_count", meta.RetryCount))
	}
	return ctx, fields
}

func registerTraceMiddleware(mux *asynq.ServeMux) {
	if mux != nil {
		mux.Use(TraceMiddleware)
	}
}
