package router

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"

	"github.com/arixbit/ginblade/internal/handler"
	"github.com/arixbit/ginblade/internal/middleware"
	"github.com/arixbit/ginblade/internal/model"
	"github.com/arixbit/ginblade/internal/service"
	"github.com/arixbit/ginblade/pkg/auth"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type mockExampleRepo struct{}

func (m *mockExampleRepo) Create(ctx context.Context, example *model.Example) error {
	return nil
}

func (m *mockExampleRepo) List(ctx context.Context, limit, offset int) ([]model.Example, int64, error) {
	return nil, 0, nil
}

type mockExampleQueue struct{}

func (m *mockExampleQueue) Available() bool {
	return true
}

func (m *mockExampleQueue) Enqueue(ctx context.Context, t *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	return &asynq.TaskInfo{}, nil
}

func newExampleHandler() *handler.ExampleHandler {
	svc := service.NewExampleService(&mockExampleRepo{}, &mockExampleQueue{})
	return handler.NewExampleHandler(svc)
}

func newAuthHandler(t *testing.T) *auth.JWTManager {
	t.Helper()
	mgr, err := auth.NewJWTManager(auth.JWTConfig{Secret: "test-secret", Issuer: "test"})
	if err != nil {
		t.Fatalf("NewJWTManager: %v", err)
	}
	return mgr
}

func routePaths(t *testing.T, deps Dependencies) map[string]string {
	t.Helper()
	engine := gin.New()
	api := engine.Group("/api/v1")
	if err := RegisterRoutes(api, deps); err != nil {
		t.Fatalf("RegisterRoutes: %v", err)
	}
	paths := make(map[string]string)
	for _, r := range engine.Routes() {
		paths[r.Method+" "+r.Path] = r.Handler
	}
	return paths
}

func TestRegisterRoutesNilDeps(t *testing.T) {
	engine := gin.New()
	api := engine.Group("/api/v1")
	if err := RegisterRoutes(api, Dependencies{}); err != nil {
		t.Fatalf("RegisterRoutes with nil deps: %v", err)
	}
	if len(engine.Routes()) != 0 {
		t.Fatalf("expected no routes, got %d", len(engine.Routes()))
	}
}

func TestRegisterRoutesExampleWithoutAuth(t *testing.T) {
	paths := routePaths(t, Dependencies{Example: newExampleHandler()})

	for _, p := range []string{
		"GET /api/v1/examples",
		"POST /api/v1/examples",
		"POST /api/v1/examples/tasks",
	} {
		if _, ok := paths[p]; !ok {
			t.Errorf("missing route %s; got %v", p, keys(paths))
		}
	}
	if _, ok := paths["POST /api/v1/auth/token"]; ok {
		t.Error("auth routes should not be registered without an auth handler")
	}
}

func TestRegisterRoutesWithAuth(t *testing.T) {
	mgr := newAuthHandler(t)
	authHandler := handler.NewAuthHandler(mgr)

	paths := routePaths(t, Dependencies{
		Auth:         authHandler,
		AuthRequired: middleware.BearerAuth(mgr),
		Example:      newExampleHandler(),
	})

	for _, p := range []string{
		"POST /api/v1/auth/token",
		"GET /api/v1/auth/me",
		"GET /api/v1/examples",
	} {
		if _, ok := paths[p]; !ok {
			t.Errorf("missing route %s; got %v", p, keys(paths))
		}
	}
}

func TestRegisterRoutesAuthWithoutRequired(t *testing.T) {
	mgr := newAuthHandler(t)
	authHandler := handler.NewAuthHandler(mgr)

	paths := routePaths(t, Dependencies{Auth: authHandler})

	if _, ok := paths["POST /api/v1/auth/token"]; !ok {
		t.Error("token route should be registered")
	}
	if _, ok := paths["GET /api/v1/auth/me"]; ok {
		t.Error("/auth/me should not be registered without AuthRequired")
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
