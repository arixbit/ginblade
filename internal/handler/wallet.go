package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/arixbit/ginblade/internal/service"
	"github.com/arixbit/ginblade/pkg/response"
)

// WalletHandler handles HTTP requests for wallets.
type WalletHandler struct {
	svc *service.WalletService
}

// NewWalletHandler creates a WalletHandler.
func NewWalletHandler(svc *service.WalletService) *WalletHandler {
	return &WalletHandler{svc: svc}
}

// Create handles POST /api/v1/wallets.
func (h *WalletHandler) Create(c *gin.Context) {
	var req service.CreateWalletReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.WriteValidationError(c, err)
		return
	}

	wallet, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		response.WriteError(c, err)
		return
	}

	response.WriteSuccess(c, wallet)
}

// List handles GET /api/v1/wallets.
func (h *WalletHandler) List(c *gin.Context) {
	var req service.ListWalletsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.WriteValidationError(c, err)
		return
	}

	res, err := h.svc.List(c.Request.Context(), &req)
	if err != nil {
		response.WriteError(c, err)
		return
	}

	response.WriteSuccess(c, res)
}

// Get handles GET /api/v1/wallets/:id.
func (h *WalletHandler) Get(c *gin.Context) {
	var req service.GetWalletReq
	if err := c.ShouldBindUri(&req); err != nil {
		response.WriteValidationError(c, err)
		return
	}

	wallet, err := h.svc.Get(c.Request.Context(), &req)
	if err != nil {
		response.WriteError(c, err)
		return
	}

	response.WriteSuccess(c, wallet)
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
