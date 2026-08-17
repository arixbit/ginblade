package bootstrap

import (
	"errors"
	"fmt"

	"github.com/arixbit/ginblade/config"
	"github.com/arixbit/ginblade/internal/taskqueue"
	"github.com/arixbit/ginblade/pkg/validator"
)

// InitAPI initializes resources required by the HTTP API process.
func InitAPI(cfg *config.Config) (*Registry, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}

	dbMgr, err := initDatabase(cfg)
	if err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}
	if dbMgr.DB() == nil {
		return nil, errors.New("postgres dsn is required for api")
	}

	cacheClient, err := initCache(cfg)
	if err != nil {
		return nil, fmt.Errorf("init cache: %w", err)
	}

	authManager, err := initAuth(cfg)
	if err != nil {
		return nil, fmt.Errorf("init auth: %w", err)
	}

	validator.InitValidator()
	queueClient := newAsynqClient(cfg)

	return &Registry{
		Cfg:         cfg,
		DB:          dbMgr,
		Cache:       cacheClient,
		Auth:        authManager,
		Queue:       taskqueue.NewQueue(queueClient),
		queueClient: queueClient,
	}, nil
}
