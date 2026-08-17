package bootstrap

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/arixbit/ginblade/config"
	applog "github.com/arixbit/ginblade/pkg/log"
)

// InitRuntime initializes process-wide runtime settings.
func InitRuntime(cfg *config.Config, service ...string) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	gin.SetMode(cfg.Server.GinMode)
	serviceName := ""
	if len(service) > 0 {
		serviceName = service[0]
	}
	if _, err := applog.Init(applog.Config{
		Level:           cfg.Log.Level,
		Format:          cfg.Log.Format,
		StacktraceLevel: cfg.Log.StacktraceLevel,
		Service:         serviceName,
	}); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	return nil
}
