package bootstrap

import (
	"errors"
	"fmt"
	"strings"

	"github.com/arixbit/ginblade/config"
	"github.com/arixbit/ginblade/internal/taskqueue"
)

// InitWorker initializes resources required by the async worker process.
// Resources already opened are closed when a later step fails, because the
// caller never receives a registry it can close.
func InitWorker(cfg *config.Config) (*Registry, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	if strings.TrimSpace(cfg.Redis.Addr) == "" {
		return nil, errors.New("redis address is required for worker")
	}

	reg := &Registry{Cfg: cfg}
	var err error
	defer func() {
		if err != nil {
			_ = reg.Close()
		}
	}()

	cacheClient, err := openCache(cfg)
	if err != nil {
		err = fmt.Errorf("init worker cache: %w", err)
		return nil, err
	}
	reg.Cache = cacheClient

	if strings.TrimSpace(cfg.Postgres.DSN) != "" {
		dbMgr, dbErr := openDatabase(cfg)
		if dbErr != nil {
			err = fmt.Errorf("init worker database: %w", dbErr)
			return nil, err
		}
		reg.DB = dbMgr
	}

	queueClient := newAsynqClient(cfg)
	reg.Queue = taskqueue.NewQueue(queueClient)
	reg.queueClient = queueClient
	return reg, nil
}
