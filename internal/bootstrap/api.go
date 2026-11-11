package bootstrap

import (
	"errors"
	"fmt"

	"github.com/arixbit/ginblade/config"
	"github.com/arixbit/ginblade/internal/taskqueue"
	"github.com/arixbit/ginblade/pkg/validator"
)

// InitAPI initializes resources required by the HTTP API process.
// Resources already opened are closed when a later step fails, because the
// caller never receives a registry it can close.
func InitAPI(cfg *config.Config) (*Registry, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}

	reg := &Registry{Cfg: cfg}
	var err error
	defer func() {
		if err != nil {
			_ = reg.Close()
		}
	}()

	dbMgr, err := openDatabase(cfg)
	if err != nil {
		err = fmt.Errorf("init database: %w", err)
		return nil, err
	}
	if dbMgr.DB() == nil {
		err = errors.New("postgres dsn is required for api")
		return nil, err
	}
	reg.DB = dbMgr

	cacheClient, err := openCache(cfg)
	if err != nil {
		err = fmt.Errorf("init cache: %w", err)
		return nil, err
	}
	reg.Cache = cacheClient

	authManager, err := openAuth(cfg)
	if err != nil {
		err = fmt.Errorf("init auth: %w", err)
		return nil, err
	}
	reg.Auth = authManager

	validator.InitValidator()
	queueClient := newAsynqClient(cfg)
	reg.Queue = taskqueue.NewQueue(queueClient)
	reg.queueClient = queueClient
	return reg, nil
}
