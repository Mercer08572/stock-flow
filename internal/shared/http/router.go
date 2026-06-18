package httpserver

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Mercer08572/stock-flow/internal/material"
	"github.com/Mercer08572/stock-flow/internal/shared/health"
	"github.com/Mercer08572/stock-flow/internal/shared/http/middleware"
)

type Dependencies struct {
	DB              *pgxpool.Pool
	MaterialService material.Service
}

func NewRouter(deps Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), middleware.TraceID())

	api := router.Group("/api/v1")
	health.NewHandler().RegisterRoutes(api)
	registerMaterialRoutes(api, deps)

	return router
}

func registerMaterialRoutes(router gin.IRouter, deps Dependencies) {
	service := deps.MaterialService
	if service == nil && deps.DB != nil {
		service = material.NewService(material.NewPostgresRepository(deps.DB))
	}

	if service == nil {
		return
	}

	material.NewHandler(service).RegisterRoutes(router)
}
