package bootstrap

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"

	"github.com/arixbit/ginblade/config"
	"github.com/arixbit/ginblade/internal/taskqueue"
	"github.com/arixbit/ginblade/pkg/auth"
	"github.com/arixbit/ginblade/pkg/cache"
	"github.com/arixbit/ginblade/pkg/database"
)

// Registry holds shared runtime resources initialized at process startup.
type Registry struct {
	Cfg   *config.Config
	DB    *database.DBManager
	Cache *cache.Client
	Auth  *auth.JWTManager
	Queue *taskqueue.Queue

	queueClient *asynq.Client
}

// Close releases resources owned by the registry.
func (r *Registry) Close() error {
	if r == nil {
		return nil
	}

	var errs []error
	if r.queueClient != nil {
		if err := r.queueClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close queue client: %w", err))
		}
	}
	if r.Cache != nil {
		if err := r.Cache.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close cache: %w", err))
		}
	}
	if r.DB != nil {
		if err := r.DB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close database: %w", err))
		}
	}
	return errors.Join(errs...)
}

func initDatabase(cfg *config.Config) (*database.DBManager, error) {
	return database.Init(database.Config{
		DSN:             cfg.Postgres.DSN,
		LogLevel:        cfg.Postgres.LogLevel,
		MaxIdleConns:    cfg.Postgres.MaxIdleConns,
		MaxOpenConns:    cfg.Postgres.MaxOpenConns,
		ConnMaxLifetime: cfg.Postgres.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Postgres.ConnMaxIdleTime,
	})
}

func initCache(cfg *config.Config) (*cache.Client, error) {
	if strings.TrimSpace(cfg.Redis.Addr) == "" {
		return nil, nil
	}
	return cache.NewClient(cache.RedisConfig{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.CacheDB,
	})
}

func initAuth(cfg *config.Config) (*auth.JWTManager, error) {
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		return nil, nil
	}
	return auth.NewJWTManager(auth.JWTConfig{
		Secret: cfg.Auth.JWTSecret,
		Issuer: cfg.Auth.JWTIssuer,
		TTL:    cfg.Auth.JWTTTL,
	})
}

func newAsynqClient(cfg *config.Config) *asynq.Client {
	if cfg == nil || strings.TrimSpace(cfg.Redis.Addr) == "" {
		return nil
	}
	return asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.QueueDB,
	})
}
