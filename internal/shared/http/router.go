package httpserver

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	category "github.com/Mercer08572/stock-flow/internal/material/category"
	material "github.com/Mercer08572/stock-flow/internal/material/material"
	unit "github.com/Mercer08572/stock-flow/internal/material/unit"
	"github.com/Mercer08572/stock-flow/internal/shared/health"
	"github.com/Mercer08572/stock-flow/internal/shared/http/middleware"
)

type Dependencies struct {
	DB              *pgxpool.Pool
	MaterialService material.Service
	UnitService     unit.UnitService
	CategoryService category.CategoryService
}

func NewRouter(deps Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), middleware.TraceID())

	api := router.Group("/api/v1")
	health.NewHandler().RegisterRoutes(api)
	registerUnitRoutes(api, deps)
	registerCategoryRoutes(api, deps)
	registerMaterialRoutes(api, deps)

	return router
}

func registerUnitRoutes(router gin.IRouter, deps Dependencies) {
	service := deps.UnitService
	if service == nil && deps.DB != nil {
		service = unit.NewUnitService(unit.NewPostgresRepository(deps.DB))
	}

	if service == nil {
		return
	}

	unit.NewUnitHandler(service).RegisterRoutes(router)
}

func registerCategoryRoutes(router gin.IRouter, deps Dependencies) {
	service := deps.CategoryService
	if service == nil && deps.DB != nil {
		service = category.NewCategoryService(category.NewPostgresRepository(deps.DB))
	}

	if service == nil {
		return
	}

	category.NewCategoryHandler(service).RegisterRoutes(router)
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
