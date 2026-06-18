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

	"github.com/Mercer08572/stock-flow/internal/shared/config"
	"github.com/Mercer08572/stock-flow/internal/shared/database"
	httpserver "github.com/Mercer08572/stock-flow/internal/shared/http"
)

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
		Addr:              cfg.HTTPAddr,
		Handler:           httpserver.NewRouter(httpserver.Dependencies{DB: db}),
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
