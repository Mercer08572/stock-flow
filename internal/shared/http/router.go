package httpserver

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	inventory "github.com/Mercer08572/stock-flow/internal/inventory"
	category "github.com/Mercer08572/stock-flow/internal/material/category"
	conversion "github.com/Mercer08572/stock-flow/internal/material/conversion"
	material "github.com/Mercer08572/stock-flow/internal/material/material"
	unit "github.com/Mercer08572/stock-flow/internal/material/unit"
	"github.com/Mercer08572/stock-flow/internal/shared/health"
	"github.com/Mercer08572/stock-flow/internal/shared/http/middleware"
	sku "github.com/Mercer08572/stock-flow/internal/sku"
	warehouse "github.com/Mercer08572/stock-flow/internal/warehouse"
)

type Dependencies struct {
	DB                *pgxpool.Pool
	MaterialService   material.Service
	UnitService       unit.UnitService
	CategoryService   category.CategoryService
	ConversionService conversion.Service
	SKUService        sku.Service
	InventoryService  inventory.Service
	WarehouseService  warehouse.Service
}

func NewRouter(deps Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), middleware.TraceID())
	router.GET("/api-docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := router.Group("/api/v1")
	health.NewHandler().RegisterRoutes(api)
	registerUnitRoutes(api, deps)
	registerCategoryRoutes(api, deps)
	registerConversionRoutes(api, deps)
	registerMaterialRoutes(api, deps)
	registerSKURoutes(api, deps)
	registerWarehouseRoutes(api, deps)
	registerInventoryRoutes(api, deps)

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

func registerConversionRoutes(router gin.IRouter, deps Dependencies) {
	service := deps.ConversionService
	if service == nil && deps.DB != nil {
		service = conversion.NewService(conversion.NewPostgresRepository(deps.DB))
	}

	if service == nil {
		return
	}

	conversion.NewHandler(service).RegisterRoutes(router)
}

func registerWarehouseRoutes(router gin.IRouter, deps Dependencies) {
	service := deps.WarehouseService
	if service == nil && deps.DB != nil {
		service = warehouse.NewService(warehouse.NewPostgresRepository(deps.DB))
	}

	if service == nil {
		return
	}

	warehouse.NewHandler(service).RegisterRoutes(router)
}

func registerInventoryRoutes(router gin.IRouter, deps Dependencies) {
	service := deps.InventoryService
	if service == nil && deps.DB != nil {
		warehouseReader := deps.WarehouseService
		if warehouseReader == nil {
			warehouseReader = warehouse.NewService(warehouse.NewPostgresRepository(deps.DB))
		}

		skuReader := deps.SKUService
		if skuReader == nil {
			materialValidator := deps.MaterialService
			if materialValidator == nil {
				materialValidator = material.NewService(material.NewPostgresRepository(deps.DB))
			}
			skuReader = sku.NewService(sku.NewPostgresRepository(deps.DB), materialValidator)
		}

		service = inventory.NewService(inventory.NewPostgresRepository(deps.DB), warehouseReader, skuReader)
	}

	if service == nil {
		return
	}

	inventory.NewHandler(service).RegisterRoutes(router)
}

func registerSKURoutes(router gin.IRouter, deps Dependencies) {
	service := deps.SKUService
	if service == nil && deps.DB != nil {
		materialValidator := deps.MaterialService
		if materialValidator == nil {
			materialValidator = material.NewService(material.NewPostgresRepository(deps.DB))
		}
		service = sku.NewService(sku.NewPostgresRepository(deps.DB), materialValidator)
	}

	if service == nil {
		return
	}

	sku.NewHandler(service).RegisterRoutes(router)
}
