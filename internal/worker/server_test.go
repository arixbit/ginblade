package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"github.com/arixbit/ginblade/internal/task"
	applog "github.com/arixbit/ginblade/pkg/log"
)

func init() {
	applog.SetLogger(zap.NewNop())
}

func TestNewRedisOpt(t *testing.T) {
	opt := NewRedisOpt("127.0.0.1:6379", "secret", 3)
	if opt.Addr != "127.0.0.1:6379" {
		t.Fatalf("Addr = %q", opt.Addr)
	}
	if opt.Password != "secret" {
		t.Fatalf("Password = %q", opt.Password)
	}
	if opt.DB != 3 {
		t.Fatalf("DB = %d, want 3", opt.DB)
	}
}

func TestRetryDelayFuncBackoff(t *testing.T) {
	delayFunc := exampleRetryDelay

	newTask := func(payload []byte) *asynq.Task {
		return asynq.NewTask("test:task", payload)
	}

	cases := []struct {
		n    int
		want time.Duration
	}{
		{0, 5 * time.Second},
		{1, 10 * time.Second},
		{2, 20 * time.Second},
		{3, 40 * time.Second},
	}
	for _, tc := range cases {
		got := delayFunc(tc.n, errors.New("boom"), newTask([]byte(`{"name":"x"}`)))
		if got != tc.want {
			t.Errorf("delay(n=%d) = %v, want %v", tc.n, got, tc.want)
		}
	}
}

func TestRetryDelayFuncCapsAtHour(t *testing.T) {
	got := exampleRetryDelay(30, errors.New("boom"), &asynq.Task{})
	if got != time.Hour {
		t.Fatalf("delay(30) = %v, want 1h cap", got)
	}
}

func TestTraceMiddlewareRestoresTrace(t *testing.T) {
	payload, _ := json.Marshal(task.ExamplePayload{Name: "demo", TraceID: "req-trace-1"})
	tk := asynq.NewTask(task.TypeExampleTask, payload)

	var captured string
	next := asynq.HandlerFunc(func(ctx context.Context, _ *asynq.Task) error {
		captured = applog.TraceIDFrom(ctx)
		return nil
	})

	meta := func(context.Context) taskRuntimeMetadata {
		return taskRuntimeMetadata{TaskID: "task-1", Queue: "default"}
	}

	handler := traceMiddleware(next, meta)
	if err := handler.ProcessTask(context.Background(), tk); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}
	if captured != "req-trace-1" {
		t.Fatalf("trace = %q, want req-trace-1 (restored from payload)", captured)
	}
}

func TestTraceMiddlewareGeneratesTraceForTask(t *testing.T) {
	payload, _ := json.Marshal(task.ExamplePayload{Name: "demo"})
	tk := asynq.NewTask(task.TypeExampleTask, payload)

	var captured string
	next := asynq.HandlerFunc(func(ctx context.Context, _ *asynq.Task) error {
		captured = applog.TraceIDFrom(ctx)
		return nil
	})

	meta := func(context.Context) taskRuntimeMetadata {
		return taskRuntimeMetadata{TaskID: "task-9"}
	}

	handler := traceMiddleware(next, meta)
	if err := handler.ProcessTask(context.Background(), tk); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}
	if captured != "asynq:task-9" {
		t.Fatalf("trace = %q, want asynq:task-9", captured)
	}
}

func TestTraceMiddlewarePropagatesError(t *testing.T) {
	tk := asynq.NewTask(task.TypeExampleTask, []byte(`{"name":"demo"}`))
	next := asynq.HandlerFunc(func(context.Context, *asynq.Task) error {
		return errors.New("handler failed")
	})

	handler := traceMiddleware(next, func(context.Context) taskRuntimeMetadata { return taskRuntimeMetadata{} })
	err := handler.ProcessTask(context.Background(), tk)
	if err == nil || err.Error() != "handler failed" {
		t.Fatalf("expected handler error to propagate, got %v", err)
	}
}

func TestTaskLogContextFields(t *testing.T) {
	tk := asynq.NewTask("example:run", []byte(`{"name":"demo","trace_id":"t-1"}`))
	ctx, fields := taskLogContext(context.Background(), tk, taskRuntimeMetadata{
		TaskID:             "task-1",
		Queue:              "default",
		RetryCount:         2,
		RetryCountRecorded: true,
	})

	if got := applog.TraceIDFrom(ctx); got != "t-1" {
		t.Fatalf("trace = %q, want t-1", got)
	}

	seen := map[string]string{}
	for _, f := range fields {
		seen[f.Key] = f.String
	}
	if seen["task"] != "example:run" {
		t.Errorf("task field = %q", seen["task"])
	}
	if seen["queue"] != "default" {
		t.Errorf("queue field = %q", seen["queue"])
	}
	if seen["task_id"] != "task-1" {
		t.Errorf("task_id field = %q", seen["task_id"])
	}
	if seen["trace_source"] != "request" {
		t.Errorf("trace_source field = %q", seen["trace_source"])
	}

	var retryCount int64 = -1
	for _, f := range fields {
		if f.Key == "retry_count" {
			retryCount = f.Integer
		}
	}
	if retryCount != 2 {
		t.Errorf("retry_count = %d, want 2", retryCount)
	}
}

func TestHandleExampleTask(t *testing.T) {
	deps := &Deps{}
	payload, _ := json.Marshal(task.ExamplePayload{Name: "demo"})
	tk := asynq.NewTask(task.TypeExampleTask, payload)

	if err := deps.HandleExampleTask(context.Background(), tk); err != nil {
		t.Fatalf("HandleExampleTask: %v", err)
	}
}

func TestHandleExampleTaskInvalidPayload(t *testing.T) {
	deps := &Deps{}
	tk := asynq.NewTask(task.TypeExampleTask, []byte("{not json"))

	if err := deps.HandleExampleTask(context.Background(), tk); err == nil {
		t.Fatal("expected error for invalid payload")
	}
}

func TestRegisterHandlersNilMux(t *testing.T) {
	RegisterHandlers(nil, &Deps{}) // should not panic
}

func TestRegisterHandlersNilDeps(t *testing.T) {
	mux := asynq.NewServeMux()
	RegisterHandlers(mux, nil) // should not panic
	if mux == nil {
		t.Fatal("mux should remain non-nil")
	}
}
