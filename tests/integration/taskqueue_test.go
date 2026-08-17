//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/arixbit/ginblade/internal/task"
	"github.com/arixbit/ginblade/internal/taskqueue"
)

func TestRedisTaskQueueIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	redisOpt := asynq.RedisClientOpt{
		Addr:     requireEnv(t, "TEST_REDIS_ADDR"),
		Password: os.Getenv("TEST_REDIS_PASSWORD"),
		DB:       requireIntEnv(t, "TEST_REDIS_QUEUE_DB"),
	}
	client := asynq.NewClient(redisOpt)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close asynq client: %v", err)
		}
	})

	inspector := asynq.NewInspector(redisOpt)
	t.Cleanup(func() {
		if err := inspector.Close(); err != nil {
			t.Errorf("close asynq inspector: %v", err)
		}
	})

	queueName := "integration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	t.Cleanup(func() {
		if err := inspector.DeleteQueue(queueName, true); err != nil && !errors.Is(err, asynq.ErrQueueNotFound) {
			t.Errorf("delete asynq integration queue: %v", err)
		}
	})

	queuedTask, err := task.NewExampleTask("integration", "integration-trace")
	if err != nil {
		t.Fatalf("create example task: %v", err)
	}
	taskID := uuid.NewString()
	queue := taskqueue.NewQueue(client)
	if queue == nil || !queue.Available() {
		t.Fatal("asynq queue is unavailable")
	}

	info, err := queue.Enqueue(ctx, queuedTask, asynq.Queue(queueName), asynq.TaskID(taskID))
	if err != nil {
		t.Fatalf("enqueue task in redis: %v", err)
	}
	assertTaskInfo(t, info, taskID, queueName, queuedTask)

	stored, err := inspector.GetTaskInfo(queueName, taskID)
	if err != nil {
		t.Fatalf("inspect task in redis: %v", err)
	}
	assertTaskInfo(t, stored, taskID, queueName, queuedTask)
}

func assertTaskInfo(t *testing.T, info *asynq.TaskInfo, taskID, queueName string, want *asynq.Task) {
	t.Helper()

	if info == nil {
		t.Fatal("asynq task info is nil")
	}
	if info.ID != taskID {
		t.Fatalf("asynq task ID = %q, want %q", info.ID, taskID)
	}
	if info.Queue != queueName {
		t.Fatalf("asynq queue = %q, want %q", info.Queue, queueName)
	}
	if info.Type != want.Type() {
		t.Fatalf("asynq task type = %q, want %q", info.Type, want.Type())
	}
	if !bytes.Equal(info.Payload, want.Payload()) {
		t.Fatalf("asynq payload = %q, want %q", info.Payload, want.Payload())
	}
	if info.State != asynq.TaskStatePending {
		t.Fatalf("asynq task state = %v, want pending", info.State)
	}
}
