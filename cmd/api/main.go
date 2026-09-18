// Package main starts the Fruits API HTTP server.
//
//	@title			Fruits API
//	@version		1.0
//	@description	RESTful API for managing fruits, backed by PostgreSQL.
//	@host			localhost:8080
//	@BasePath		/
//	@schemes		http https
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/sudosz/fruits-api/docs"
	"github.com/sudosz/fruits-api/internal/config"
	"github.com/sudosz/fruits-api/internal/database"
	"github.com/sudosz/fruits-api/internal/handlers"
	"github.com/sudosz/fruits-api/internal/middleware"
)

func main() {
	// run() owns every deferred cleanup so that a fatal error still closes the
	// database pool; log.Fatal in main would skip those defers.
	if err := run(); err != nil {
		log.Fatalf("fruits-api: %v", err)
	}
}

func run() error {
	cfg := config.Load()

	// Signal-aware context: startup retries and shutdown share one cancel path.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	connectCtx, cancelConnect := context.WithTimeout(ctx, 30*time.Second)
	defer cancelConnect()

	db, err := database.Connect(connectCtx, cfg)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			log.Printf("close database: %v", cerr)
		}
	}()

	migrateCtx, cancelMigrate := context.WithTimeout(ctx, 15*time.Second)
	defer cancelMigrate()

	if err := database.Migrate(migrateCtx, db); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	router, err := newRouter(db)
	if err != nil {
		return fmt.Errorf("build router: %w", err)
	}

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
		// Timeouts bound how long a single connection can tie up a worker,
		// which is the cheapest defense against slowloris-style clients.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- fmt.Errorf("listen: %w", err)
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		log.Println("shutting down")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Println("stopped")
	return nil
}

func newRouter(db *sql.DB) (*gin.Engine, error) {
	router := gin.New()

	// Client IPs come from the platform's load balancer, not arbitrary
	// X-Forwarded-For headers, so proxy trust stays off by default.
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}
	router.RedirectTrailingSlash = true
	router.HandleMethodNotAllowed = true

	router.Use(
		gin.Recovery(),
		gin.LoggerWithConfig(gin.LoggerConfig{
			// Probe traffic would otherwise dominate the logs.
			SkipPaths: []string{"/healthz"},
		}),
		middleware.SecurityHeaders(),
		middleware.BodyLimit(middleware.MaxBodyBytes),
		middleware.RateLimit(50, 100),
	)

	handlers.New(db).RegisterRoutes(router)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return router, nil
}
