package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/internal/auth"
	"github.com/Mercer08572/stock-flow/internal/shared/config"
	"github.com/Mercer08572/stock-flow/internal/shared/database"
	httpserver "github.com/Mercer08572/stock-flow/internal/shared/http"
	_ "github.com/Mercer08572/stock-flow/openapi"
)

// @title Stock-Flow API
// @version 1.0
// @description Back-end API service for the Stock-Flow inventory management system.
// @description
// @description 管理员会话认证：登录后由服务端下发 HttpOnly 会话 Cookie，受保护接口在该 Cookie
// @description 有效时即可访问；强制改密期间只有 login / logout / me / password 可用。
// @description
// @description 外部系统认证：使用 API 应用的公开标识与密钥，两个请求头必须同时提供。
// @BasePath /api/v1
//
// @securityDefinitions.apikey AdminSession
// @in cookie
// @name stock_flow_admin_session
// @description 管理员会话 Cookie，由 POST /auth/admin/login 下发，HttpOnly 且不可通过 JS 读取。
//
// @securityDefinitions.apikey APIAppCredentials
// @in header
// @name X-Stock-Flow-App-ID
// @description 外部系统认证。除本请求头外还必须提供 X-Stock-Flow-Secret，密钥仅在签发时明文返回一次。
func main() {
	if err := run(); err != nil {
		log.Fatalf("api stopped: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	gin.SetMode(cfg.GinMode)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpserver.NewRouter(httpserver.Dependencies{
			DB:                     db,
			AuthSessionTTL:         cfg.AuthAdminSessionTTL,
			AuthCookieSecure:       cfg.AuthAdminCookieSecure,
			AuthCookieSameSite:     sameSiteMode(cfg.AuthAdminCookieSameSite),
			AuthLoginFailurePolicy: auth.RateLimitPolicy{MaxAttempts: cfg.AuthLoginFailureMaxAttempts, Window: cfg.AuthLoginFailureWindow, Lockout: cfg.AuthLoginLockout},
			AuthLoginIPPolicy:      auth.RateLimitPolicy{MaxAttempts: cfg.AuthLoginIPMaxAttempts, Window: cfg.AuthLoginFailureWindow, Lockout: cfg.AuthLoginLockout},
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("api listening on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}

func sameSiteMode(value string) http.SameSite {
	switch value {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
