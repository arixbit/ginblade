package app

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/arixbit/ginblade/config"
	"github.com/arixbit/ginblade/internal/bootstrap"
	"github.com/arixbit/ginblade/pkg/database"
	applog "github.com/arixbit/ginblade/pkg/log"
)

func init() {
	gin.SetMode(gin.TestMode)
	applog.SetLogger(zap.NewNop())
}

func TestValidateHTTPRegistry(t *testing.T) {
	if err := validateHTTPRegistry(nil); err != errNilRegistry {
		t.Fatalf("nil registry = %v, want errNilRegistry", err)
	}
	if err := validateHTTPRegistry(&bootstrap.Registry{}); err != errNilConfig {
		t.Fatalf("nil config = %v, want errNilConfig", err)
	}
	if err := validateHTTPRegistry(&bootstrap.Registry{Cfg: &config.Config{}}); err != errMissingDB {
		t.Fatalf("missing db = %v, want errMissingDB", err)
	}
	reg := &bootstrap.Registry{
		Cfg: &config.Config{},
		DB:  database.NewTestManager(nil),
	}
	if err := validateHTTPRegistry(reg); err != errMissingDB {
		t.Fatalf("nil db handle = %v, want errMissingDB", err)
	}
}

func TestValidateWorkerRegistry(t *testing.T) {
	if err := validateWorkerRegistry(nil); err != errNilRegistry {
		t.Fatalf("nil registry = %v, want errNilRegistry", err)
	}
	if err := validateWorkerRegistry(&bootstrap.Registry{}); err != errNilConfig {
		t.Fatalf("nil config = %v, want errNilConfig", err)
	}
	if err := validateWorkerRegistry(&bootstrap.Registry{Cfg: &config.Config{}}); err == nil {
		t.Fatal("expected error for missing redis address")
	}
	reg := &bootstrap.Registry{Cfg: &config.Config{Redis: config.RedisConfig{Addr: "redis:6379"}}}
	if err := validateWorkerRegistry(reg); err != nil {
		t.Fatalf("valid registry: %v", err)
	}
}

func TestNewHTTPHandlersWithNilResources(t *testing.T) {
	reg := &bootstrap.Registry{
		Cfg: &config.Config{},
		DB:  database.NewTestManager(nil),
	}
	handlers := newHTTPHandlers(reg)
	if handlers == nil {
		t.Fatal("expected non-nil handlers")
	}
	if handlers.Example == nil {
		t.Error("expected example handler")
	}
	if handlers.Health == nil {
		t.Error("expected health handler")
	}
	if handlers.Auth != nil {
		t.Error("expected nil auth handler without JWT manager")
	}
}

func TestNewHTTPServer(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:           ":9999",
			RequestTimeout: 5 * time.Second,
		},
	}
	srv := newHTTPServer(cfg, gin.New())
	if srv == nil {
		t.Fatal("expected non-nil http server")
	}
	if srv.Addr != ":9999" {
		t.Fatalf("Addr = %q, want :9999", srv.Addr)
	}
	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v, want 5s", srv.ReadHeaderTimeout)
	}
	if srv.Handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestBuildWorkerDeps(t *testing.T) {
	reg := &bootstrap.Registry{
		Cfg:   &config.Config{Redis: config.RedisConfig{Addr: "redis:6379"}},
		DB:    database.NewTestManager(nil),
		Cache: nil,
		Queue: nil,
	}
	deps := buildWorkerDeps(reg)
	if deps == nil {
		t.Fatal("expected non-nil deps")
	}
	if deps.DB != nil {
		t.Error("expected nil DB when manager has no handle")
	}
}

func TestServerRunNilHTTP(t *testing.T) {
	s := &Server{}
	if err := s.Run(); err != errNilHTTPServer {
		t.Fatalf("Run = %v, want errNilHTTPServer", err)
	}
}

func TestServerShutdownNilHTTP(t *testing.T) {
	s := &Server{}
	if err := s.Shutdown(t.Context()); err != errNilHTTPServer {
		t.Fatalf("Shutdown = %v, want errNilHTTPServer", err)
	}
}

func TestServerCloseNilHTTP(t *testing.T) {
	s := &Server{}
	if err := s.Close(); err != errNilHTTPServer {
		t.Fatalf("Close = %v, want errNilHTTPServer", err)
	}
}

func TestNewServerNilRegistry(t *testing.T) {
	if _, err := NewServer(nil); err != errNilRegistry {
		t.Fatalf("NewServer(nil) = %v, want errNilRegistry", err)
	}
}

func TestWorkerRunNilServer(t *testing.T) {
	w := &Worker{}
	if err := w.Run(t.Context()); err != errNilWorker {
		t.Fatalf("Run = %v, want errNilWorker", err)
	}
}

func TestNewWorkerNilRegistry(t *testing.T) {
	if _, err := NewWorker(nil); err != errNilRegistry {
		t.Fatalf("NewWorker(nil) = %v, want errNilRegistry", err)
	}
}

func TestHTTPServerUsesConfigPort(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Port: ":1234"}}
	srv := newHTTPServer(cfg, gin.New())
	if srv.Addr != ":1234" {
		t.Fatalf("Addr = %q, want :1234", srv.Addr)
	}
	if srv.Handler == nil || srv.Handler == http.NotFoundHandler() {
		t.Fatal("expected engine handler")
	}
}
