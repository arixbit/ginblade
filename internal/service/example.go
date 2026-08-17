package service

import (
	"context"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"github.com/arixbit/ginblade/internal/errcode"
	"github.com/arixbit/ginblade/internal/model"
	"github.com/arixbit/ginblade/internal/task"
	applog "github.com/arixbit/ginblade/pkg/log"
)

// ExampleRepository is the persistence dependency used by ExampleService.
type ExampleRepository interface {
	Create(ctx context.Context, example *model.Example) error
	List(ctx context.Context, limit, offset int) ([]model.Example, int64, error)
}

// ExampleQueue is the async task queue boundary used by ExampleService.
type ExampleQueue interface {
	Available() bool
	Enqueue(ctx context.Context, t *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// ExampleService handles the example application flow.
type ExampleService struct {
	repo  ExampleRepository
	queue ExampleQueue
}

// NewExampleService creates an ExampleService with the given repository.
func NewExampleService(repo ExampleRepository, queue ...ExampleQueue) *ExampleService {
	var q ExampleQueue
	if len(queue) > 0 {
		q = queue[0]
	}
	return &ExampleService{repo: repo, queue: q}
}

// CreateExampleReq is the request body for creating an example.
type CreateExampleReq struct {
	Name string `json:"name" binding:"required"`
}

// Create creates a new example.
func (s *ExampleService) Create(ctx context.Context, req *CreateExampleReq) (*model.Example, error) {
	example := model.Example{Name: req.Name}
	if err := s.repo.Create(ctx, &example); err != nil {
		applog.FromContext(ctx).Error("failed to create example", zap.Error(err))
		return nil, errcode.DatabaseError
	}
	return &example, nil
}

// EnqueueExampleTaskReq is the request body for publishing an example task.
type EnqueueExampleTaskReq struct {
	Name string `json:"name" binding:"required"`
}

// EnqueueExampleTaskRes is the response for publishing an example task.
type EnqueueExampleTaskRes struct {
	Queued bool `json:"queued"`
}

// EnqueueTask publishes an example async task.
func (s *ExampleService) EnqueueTask(ctx context.Context, req *EnqueueExampleTaskReq) (*EnqueueExampleTaskRes, error) {
	if s.queue == nil || !s.queue.Available() {
		return nil, errcode.QueueUnavailable
	}

	t, err := task.NewExampleTask(req.Name, applog.TraceIDFrom(ctx))
	if err != nil {
		applog.FromContext(ctx).Error("failed to create example task", zap.Error(err))
		return nil, errcode.QueueError
	}
	if _, err := s.queue.Enqueue(ctx, t); err != nil {
		applog.FromContext(ctx).Error("failed to enqueue example task", zap.Error(err))
		return nil, errcode.QueueError
	}
	return &EnqueueExampleTaskRes{Queued: true}, nil
}

// ListExamplesReq is the request query for listing examples.
type ListExamplesReq struct {
	Limit  int `form:"limit" binding:"omitempty,min=1,max=100"`
	Offset int `form:"offset" binding:"omitempty,min=0"`
}

// ListExamplesRes is the response for listing examples.
type ListExamplesRes struct {
	Examples []model.Example `json:"examples"`
	Total    int64           `json:"total"`
}

// List returns a paginated list of examples.
func (s *ExampleService) List(ctx context.Context, req *ListExamplesReq) (*ListExamplesRes, error) {
	if req.Limit == 0 {
		req.Limit = 20
	}
	examples, total, err := s.repo.List(ctx, req.Limit, req.Offset)
	if err != nil {
		applog.FromContext(ctx).Error("failed to list examples", zap.Error(err))
		return nil, errcode.DatabaseError
	}

	return &ListExamplesRes{Examples: examples, Total: total}, nil
}
