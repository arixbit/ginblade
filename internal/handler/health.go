package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/arixbit/ginblade/pkg/cache"
	"github.com/arixbit/ginblade/pkg/database"
)

// HealthHandler checks infrastructure dependencies.
type HealthHandler struct {
	db    *database.DBManager
	cache *cache.Client
}

// NewHealthHandler creates a HealthHandler.
func NewHealthHandler(db *database.DBManager, cache *cache.Client) *HealthHandler {
	return &HealthHandler{db: db, cache: cache}
}

// Health returns database and cache health status.
func (h *HealthHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	checks := map[string]string{}
	healthy := true

	if h.db == nil {
		checks["postgres"] = "not_configured"
		healthy = false
	} else if err := h.db.Ping(ctx); err != nil {
		checks["postgres"] = "unavailable"
		healthy = false
	} else {
		checks["postgres"] = "ok"
	}

	if h.cache == nil {
		checks["redis"] = "not_configured"
	} else if err := h.cache.Ping(ctx); err != nil {
		checks["redis"] = "unavailable"
		healthy = false
	} else {
		checks["redis"] = "ok"
	}

	if !healthy {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"checks": checks,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"checks": checks,
	})
}
