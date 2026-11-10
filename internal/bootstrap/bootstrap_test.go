package bootstrap

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/arixbit/ginblade/config"
	"github.com/arixbit/ginblade/pkg/auth"
	"github.com/arixbit/ginblade/pkg/cache"
	"github.com/arixbit/ginblade/pkg/database"
	applog "github.com/arixbit/ginblade/pkg/log"
)

const stubSQLDriverName = "ginblade_bootstrap_stub"

func init() {
	sql.Register(stubSQLDriverName, stubDriver{})
}

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

type stubDriver struct{}

func (stubDriver) Open(string) (driver.Conn, error) { return stubConn{}, nil }

type stubConn struct{}

func (stubConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("stub: prepare not supported")
}

func (stubConn) Close() error { return nil }

func (stubConn) Begin() (driver.Tx, error) {
	return nil, errors.New("stub: begin not supported")
}

func (stubConn) Ping(context.Context) error { return nil }

func swapOpeners(
	t *testing.T,
	db func(*config.Config) (*database.DBManager, error),
	cacheFn func(*config.Config) (*cache.Client, error),
	authFn func(*config.Config) (*auth.JWTManager, error),
) {
	t.Helper()
	prevDB, prevCache, prevAuth := openDatabase, openCache, openAuth
	if db != nil {
		openDatabase = db
	}
	if cacheFn != nil {
		openCache = cacheFn
	}
	if authFn != nil {
		openAuth = authFn
	}
	t.Cleanup(func() {
		openDatabase, openCache, openAuth = prevDB, prevCache, prevAuth
	})
}

func newStubDB(t *testing.T) *database.DBManager {
	t.Helper()
	sqlDB, err := sql.Open(stubSQLDriverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	mgr := database.NewTestManager(gdb)
	t.Cleanup(func() { _ = mgr.Close() })
	return mgr
}

func newStubCache() *cache.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:1",
		Dialer: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("dial disabled")
		},
	})
	return cache.NewTestClient(rdb)
}

func dbClosed(t *testing.T, mgr *database.DBManager) bool {
	t.Helper()
	sqlDB, err := mgr.DB().DB()
	if err != nil {
		t.Fatalf("sql DB: %v", err)
	}
	err = sqlDB.Ping()
	return err != nil && strings.Contains(err.Error(), "closed")
}

func cacheClosed(client *cache.Client) bool {
	if client == nil || client.Underlying() == nil {
		return false
	}
	err := client.Underlying().Ping(context.Background()).Err()
	return err != nil && strings.Contains(err.Error(), "client is closed")
}

func TestInitAPIClosesDatabaseWhenCacheInitFails(t *testing.T) {
	db := newStubDB(t)
	swapOpeners(t,
		func(*config.Config) (*database.DBManager, error) { return db, nil },
		func(*config.Config) (*cache.Client, error) { return nil, errors.New("cache down") },
		func(*config.Config) (*auth.JWTManager, error) {
			return nil, errors.New("auth should not run")
		},
	)

	reg, err := InitAPI(&config.Config{Postgres: config.PostgresConfig{DSN: "postgres://stub"}})
	if err == nil || reg != nil {
		t.Fatalf("InitAPI = (%v, %v), want nil registry and error", reg, err)
	}
	if !strings.Contains(err.Error(), "init cache") {
		t.Fatalf("error = %v, want init cache", err)
	}
	if !dbClosed(t, db) {
		t.Fatal("database left open after cache init failure")
	}
}

func TestInitAPIClosesOpenedResourcesWhenAuthInitFails(t *testing.T) {
	db := newStubDB(t)
	cacheClient := newStubCache()
	swapOpeners(t,
		func(*config.Config) (*database.DBManager, error) { return db, nil },
		func(*config.Config) (*cache.Client, error) { return cacheClient, nil },
		func(*config.Config) (*auth.JWTManager, error) { return nil, errors.New("auth down") },
	)

	reg, err := InitAPI(&config.Config{
		Postgres: config.PostgresConfig{DSN: "postgres://stub"},
		Redis:    config.RedisConfig{Addr: "127.0.0.1:1"},
		Auth:     config.AuthConfig{JWTSecret: "secret"},
	})
	if err == nil || reg != nil {
		t.Fatalf("InitAPI = (%v, %v), want nil registry and error", reg, err)
	}
	if !strings.Contains(err.Error(), "init auth") {
		t.Fatalf("error = %v, want init auth", err)
	}
	if !dbClosed(t, db) {
		t.Fatal("database left open after auth init failure")
	}
	if !cacheClosed(cacheClient) {
		t.Fatal("cache left open after auth init failure")
	}
}

func TestInitAPILeavesDatabaseOpenOnSuccess(t *testing.T) {
	db := newStubDB(t)
	swapOpeners(t,
		func(*config.Config) (*database.DBManager, error) { return db, nil },
		func(*config.Config) (*cache.Client, error) { return nil, nil },
		func(*config.Config) (*auth.JWTManager, error) { return nil, nil },
	)

	reg, err := InitAPI(&config.Config{Postgres: config.PostgresConfig{DSN: "postgres://stub"}})
	if err != nil {
		t.Fatalf("InitAPI: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	if dbClosed(t, db) {
		t.Fatal("database closed after successful init")
	}
}

func TestInitWorkerClosesCacheWhenDatabaseInitFails(t *testing.T) {
	cacheClient := newStubCache()
	swapOpeners(t,
		func(*config.Config) (*database.DBManager, error) { return nil, errors.New("db down") },
		func(*config.Config) (*cache.Client, error) { return cacheClient, nil },
		nil,
	)

	reg, err := InitWorker(&config.Config{
		Redis:    config.RedisConfig{Addr: "127.0.0.1:1"},
		Postgres: config.PostgresConfig{DSN: "postgres://stub"},
	})
	if err == nil || reg != nil {
		t.Fatalf("InitWorker = (%v, %v), want nil registry and error", reg, err)
	}
	if !strings.Contains(err.Error(), "init worker database") {
		t.Fatalf("error = %v, want init worker database", err)
	}
	if !cacheClosed(cacheClient) {
		t.Fatal("cache left open after database init failure")
	}
}

func TestInitWorkerLeavesCacheOpenOnSuccess(t *testing.T) {
	cacheClient := newStubCache()
	swapOpeners(t,
		func(*config.Config) (*database.DBManager, error) {
			return nil, errors.New("database should not open")
		},
		func(*config.Config) (*cache.Client, error) { return cacheClient, nil },
		nil,
	)

	reg, err := InitWorker(&config.Config{Redis: config.RedisConfig{Addr: "127.0.0.1:1"}})
	if err != nil {
		t.Fatalf("InitWorker: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	if cacheClosed(cacheClient) {
		t.Fatal("cache closed after successful init")
	}
}
