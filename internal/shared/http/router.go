package httpserver

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	audit "github.com/Mercer08572/stock-flow/internal/audit"
	auth "github.com/Mercer08572/stock-flow/internal/auth"
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
	DB                     *pgxpool.Pool
	AuthService            auth.Service
	Authenticator          auth.Authenticator
	AuthSessionTTL         time.Duration
	AuthCookieSecure       bool
	AuthCookieSameSite     http.SameSite
	AuthLoginFailurePolicy auth.RateLimitPolicy
	AuthLoginIPPolicy      auth.RateLimitPolicy
	HTTPTrustedProxies     []string
	AuditSink              audit.EntrySink
	AuditService           audit.Service
	MaterialService        material.Service
	UnitService            unit.UnitService
	CategoryService        category.CategoryService
	ConversionService      conversion.Service
	SKUService             sku.Service
	InventoryService       inventory.Service
	WarehouseService       warehouse.Service
}

func NewRouter(deps Dependencies) *gin.Engine {
	router := gin.New()
	if len(deps.HTTPTrustedProxies) > 0 {
		_ = router.SetTrustedProxies(deps.HTTPTrustedProxies)
	} else {
		_ = router.SetTrustedProxies(nil)
	}
	// Audit must wrap Recovery so a panic still produces a 500 audit row, and it
	// must wrap auth so 401/400 responses are recorded as well.
	router.Use(middleware.TraceID())
	if deps.AuditSink != nil {
		router.Use(audit.NewMiddleware(deps.AuditSink, audit.MiddlewareOptions{}))
	}
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/api-docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := router.Group("/api/v1")
	health.NewHandler().RegisterRoutes(api)

	authService := resolveAuthService(deps)
	authenticator := deps.Authenticator
	if authenticator == nil {
		authenticator = authService
	}
	authMiddleware := auth.NewMiddleware(authenticator, auth.MiddlewareOptions{})
	if authService != nil {
		auth.NewHandler(authService, auth.CookieOptions{Secure: deps.AuthCookieSecure, SameSite: deps.AuthCookieSameSite}).RegisterRoutes(api, authMiddleware.AdminSession(), authMiddleware.AdminSessionAllowPasswordChange())
	}

	protected := api.Group("")
	protected.Use(authMiddleware.Protected())
	registerUnitRoutes(protected, deps)
	registerCategoryRoutes(protected, deps)
	registerConversionRoutes(protected, deps)
	registerMaterialRoutes(protected, deps)
	registerSKURoutes(protected, deps)
	registerWarehouseRoutes(protected, deps)
	registerInventoryRoutes(protected, deps)

	// Audit logs expose every caller's IP and identity, so they are admin-only:
	// API applications must not be able to read them through the shared
	// Protected() middleware.
	adminOnly := api.Group("")
	adminOnly.Use(authMiddleware.AdminSession())
	registerAuditRoutes(adminOnly, deps)

	return router
}

func resolveAuthService(deps Dependencies) auth.Service {
	if deps.AuthService != nil {
		return deps.AuthService
	}
	if deps.DB == nil {
		return nil
	}

	return auth.NewService(
		auth.NewPostgresRepository(deps.DB),
		auth.NewInMemorySessionStore(),
		auth.NewArgon2idPasswordHasher(auth.Argon2idParams{}),
		auth.ServiceOptions{SessionTTL: deps.AuthSessionTTL, LoginFailurePolicy: deps.AuthLoginFailurePolicy, LoginIPPolicy: deps.AuthLoginIPPolicy},
	)
}

func registerAuditRoutes(router gin.IRouter, deps Dependencies) {
	service := deps.AuditService
	if service == nil && deps.DB != nil {
		service = audit.NewService(audit.NewPostgresRepository(deps.DB), audit.ServiceOptions{})
	}
	if service == nil {
		return
	}
	audit.NewHandler(service).RegisterRoutes(router)
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
		materialValidator := deps.MaterialService
		if materialValidator == nil {
			materialValidator = material.NewService(material.NewPostgresRepository(deps.DB))
		}
		service = conversion.NewService(conversion.NewPostgresRepository(deps.DB), materialValidator)
	}

	if service == nil {
		return
	}

	conversion.NewHandler(service).RegisterRoutes(router)
}

func registerWarehouseRoutes(router gin.IRouter, deps Dependencies) {
	service := deps.WarehouseService
	if service == nil && deps.DB != nil {
		inventoryReferences := inventory.NewReferenceService(inventory.NewPostgresRepository(deps.DB))
		service = warehouse.NewService(warehouse.NewPostgresRepository(deps.DB), inventoryReferences)
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

		service = inventory.NewService(
			inventory.NewPostgresRepository(deps.DB),
			warehouseReader,
			skuReader,
			inventory.NewPostgresUnitOfWork(deps.DB),
		)
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
		inventoryReferences := inventory.NewReferenceService(inventory.NewPostgresRepository(deps.DB))
		service = sku.NewService(sku.NewPostgresRepository(deps.DB), materialValidator, inventoryReferences)
	}

	if service == nil {
		return
	}

	sku.NewHandler(service).RegisterRoutes(router)
}
