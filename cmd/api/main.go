// Package main is the entrypoint for the Fruits API service.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
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
)

// @title			Fruits API
// @version		1.0
// @description	RESTful service for managing fruits, backed by PostgreSQL.
// @license.name	MIT
// @host			localhost:8080
// @BasePath		/
// @schemes		http https
func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		log.Fatalf("migration: %v", err)
	}

	router := gin.Default()
	handlers.New(db).RegisterRoutes(router)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	// Drain in-flight requests on SIGTERM so rolling Kubernetes deploys don't
	// cut connections mid-response.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("stopped")
}
