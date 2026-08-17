package bootstrap

import (
	"testing"

	"go.uber.org/zap"

	"github.com/arixbit/ginblade/config"
	applog "github.com/arixbit/ginblade/pkg/log"
)

func init() {
	applog.SetLogger(zap.NewNop())
}

func TestInitAPINilConfig(t *testing.T) {
	if _, err := InitAPI(nil); err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestInitAPIMissingDSN(t *testing.T) {
	if _, err := InitAPI(&config.Config{}); err == nil {
		t.Fatal("expected error for missing postgres DSN")
	}
}

func TestInitWorkerNilConfig(t *testing.T) {
	if _, err := InitWorker(nil); err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestInitWorkerMissingRedis(t *testing.T) {
	if _, err := InitWorker(&config.Config{}); err == nil {
		t.Fatal("expected error for missing redis address")
	}
}

func TestRegistryCloseNil(t *testing.T) {
	var r *Registry
	if err := r.Close(); err != nil {
		t.Fatalf("Close on nil registry: %v", err)
	}
}

func TestRegistryCloseEmpty(t *testing.T) {
	r := &Registry{}
	if err := r.Close(); err != nil {
		t.Fatalf("Close on empty registry: %v", err)
	}
}

func TestInitCacheEmptyAddr(t *testing.T) {
	got, err := initCache(&config.Config{})
	if err != nil {
		t.Fatalf("initCache: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil cache for empty addr")
	}
}

func TestInitAuthEmptySecret(t *testing.T) {
	got, err := initAuth(&config.Config{})
	if err != nil {
		t.Fatalf("initAuth: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil auth for empty secret")
	}
}

func TestNewAsynqClientNilConfig(t *testing.T) {
	if got := newAsynqClient(nil); got != nil {
		t.Fatal("expected nil client for nil config")
	}
}

func TestNewAsynqClientEmptyAddr(t *testing.T) {
	if got := newAsynqClient(&config.Config{}); got != nil {
		t.Fatal("expected nil client for empty redis addr")
	}
}

func TestInitRuntimeNilConfig(t *testing.T) {
	if err := InitRuntime(nil); err == nil {
		t.Fatal("expected error for nil config")
	}
}
