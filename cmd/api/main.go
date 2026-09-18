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
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/sudosz/fruits-api/docs"
	"github.com/sudosz/fruits-api/internal/config"
	"github.com/sudosz/fruits-api/internal/database"
	"github.com/sudosz/fruits-api/internal/handlers"
	"github.com/sudosz/fruits-api/internal/middleware"
	"github.com/sudosz/fruits-api/internal/repository"
	"github.com/sudosz/fruits-api/internal/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	connectCtx, cancelConnect := context.WithTimeout(ctx, cfg.DBConnectTimeout)
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

	migrateCtx, cancelMigrate := context.WithTimeout(ctx, cfg.DBMigrateTimeout)
	defer cancelMigrate()

	if err := database.Migrate(migrateCtx, db); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	// Config -> DB -> Repository -> Service -> Handler -> Router.
	repo := repository.NewFruitRepository(db)
	svc := service.NewFruitService(repo)
	handler := handlers.NewFruitHandler(svc)

	router, err := newRouter(cfg, handler)
	if err != nil {
		return fmt.Errorf("build router: %w", err)
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown: %v", err)
		}
	}()

	log.Printf("listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}

	return nil
}

func newRouter(cfg config.Config, handler *handlers.FruitHandler) (*gin.Engine, error) {
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
		middleware.BodyLimit(cfg.MaxBodyBytes),
		middleware.RateLimit(cfg.RateLimitPerSecond, cfg.RateLimitBurst, cfg.RateLimitReapEvery),
	)

	handler.RegisterRoutes(router)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return router, nil
}
