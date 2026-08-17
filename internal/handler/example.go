package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/arixbit/ginblade/internal/service"
	"github.com/arixbit/ginblade/pkg/response"
)

// ExampleHandler handles HTTP requests for examples.
type ExampleHandler struct {
	svc *service.ExampleService
}

// NewExampleHandler creates an ExampleHandler.
func NewExampleHandler(svc *service.ExampleService) *ExampleHandler {
	return &ExampleHandler{svc: svc}
}

// Create handles POST /api/v1/examples.
func (h *ExampleHandler) Create(c *gin.Context) {
	var req service.CreateExampleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.WriteValidationError(c, err)
		return
	}

	example, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		response.WriteError(c, err)
		return
	}

	response.WriteSuccess(c, example)
}

// List handles GET /api/v1/examples.
func (h *ExampleHandler) List(c *gin.Context) {
	var req service.ListExamplesReq
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

// EnqueueTask handles POST /api/v1/examples/tasks.
func (h *ExampleHandler) EnqueueTask(c *gin.Context) {
	var req service.EnqueueExampleTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.WriteValidationError(c, err)
		return
	}

	res, err := h.svc.EnqueueTask(c.Request.Context(), &req)
	if err != nil {
		response.WriteError(c, err)
		return
	}

	response.WriteSuccess(c, res)
}
