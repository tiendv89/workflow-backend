// Package api provides the cobra subcommand for starting the HTTP API server.
package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/tiendv89/workflow-backend/configs"
	"github.com/tiendv89/workflow-backend/internal/adapter"
	apimiddleware "github.com/tiendv89/workflow-backend/internal/app/api/middleware"
	"github.com/tiendv89/workflow-backend/internal/database"
	"github.com/tiendv89/workflow-backend/internal/handler"
	"github.com/tiendv89/workflow-backend/internal/service"
)

const maxRequestBodyBytes = 1 << 20 // 1 MB

var skipPaths = map[string]struct{}{
	"/healthz": {},
}

// Command is the cobra subcommand for starting the HTTP API server.
var Command = &cobra.Command{
	Use:   "api",
	Short: "Start the HTTP API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(configs.G)
	},
}

func run(cfg *configs.Config) error {
	if cfg.DB.AutoMigration {
		migCtx, migCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		if err := database.RunMigrations(migCtx, cfg.DB.DSN()); err != nil {
			migCancel()
			return fmt.Errorf("migrations: %w", err)
		}
		migCancel()
	}

	connCtx, connCancel := context.WithTimeout(context.Background(), 15*time.Second)
	pool, err := database.Connect(connCtx, cfg.DB)
	connCancel()
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()

	reader := database.NewReader(pool)
	adapterClient := adapter.New(cfg.API.AdapterServiceURL)
	svc := service.New(reader, adapterClient, cfg.StaleThreshold())

	gin.SetMode(cfg.API.HTTP.Mode)

	r := gin.New()
	r.Use(requestid.New())
	r.Use(apimiddleware.Log(skipPaths))
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
	r.Use(maxBodySizeMiddleware())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	h := handler.New(svc)
	h.RegisterRoutes(api)

	srv := &http.Server{
		Addr:         cfg.API.HTTP.Address,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info().Str("address", cfg.API.HTTP.Address).Msg("api-service listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("listen failed")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("api-service: shutting down")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()

	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("shutdown error")
	}
	return nil
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func maxBodySizeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
		c.Next()
	}
}
