package router

import (
	"github.com/gin-gonic/gin"

	"github.com/arixbit/ginblade/internal/handler"
)

// Dependencies collects handlers and middleware needed during route registration.
type Dependencies struct {
	Auth         *handler.AuthHandler
	AuthRequired gin.HandlerFunc
	Example      *handler.ExampleHandler
	Wallet       *handler.WalletHandler
}

// RegisterRoutes registers API routes under the given router group.
func RegisterRoutes(r *gin.RouterGroup, deps Dependencies) error {
	registerAuthRoutes(r, deps)
	registerExampleRoutes(r, deps)
	registerWalletRoutes(r, deps)
	return nil
}

func registerAuthRoutes(r *gin.RouterGroup, deps Dependencies) {
	if deps.Auth == nil {
		return
	}

	authRoutes := r.Group("/auth")
	authRoutes.POST("/token", deps.Auth.CreateToken)
	if deps.AuthRequired != nil {
		authRoutes.GET("/me", deps.AuthRequired, deps.Auth.Me)
	}
}

func registerExampleRoutes(r *gin.RouterGroup, deps Dependencies) {
	if deps.Example == nil {
		return
	}

	examples := r.Group("/examples")
	examples.GET("", deps.Example.List)
	examples.POST("", deps.Example.Create)
	examples.POST("/tasks", deps.Example.EnqueueTask)
}

func registerWalletRoutes(r *gin.RouterGroup, deps Dependencies) {
	if deps.Wallet == nil {
		return
	}

	wallets := r.Group("/wallets")
	wallets.GET("", deps.Wallet.List)
	wallets.POST("", deps.Wallet.Create)
	wallets.POST("/transfer", deps.Wallet.Transfer)
}
