package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/arixbit/ginblade/internal/service"
	"github.com/arixbit/ginblade/pkg/response"
)

// WalletHandler handles HTTP requests for wallet transfers.
type WalletHandler struct {
	svc *service.WalletService
}

// NewWalletHandler creates a WalletHandler.
func NewWalletHandler(svc *service.WalletService) *WalletHandler {
	return &WalletHandler{svc: svc}
}

// Transfer handles POST /api/v1/wallets/transfers.
func (h *WalletHandler) Transfer(c *gin.Context) {
	var req service.TransferReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.WriteValidationError(c, err)
		return
	}

	res, err := h.svc.Transfer(c.Request.Context(), &req)
	if err != nil {
		response.WriteError(c, err)
		return
	}

	response.WriteSuccess(c, res)
}
