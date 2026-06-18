package health

import (
	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/pkg/response"
)

type Handler interface {
	RegisterRoutes(router gin.IRouter)
}

type handler struct{}

type statusResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func NewHandler() Handler {
	return &handler{}
}

func (h *handler) RegisterRoutes(router gin.IRouter) {
	router.GET("/health", h.Get)
}

func (h *handler) Get(c *gin.Context) {
	response.Success(c, statusResponse{
		Status:  "ok",
		Service: "stock-flow",
	})
}
