package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/tiendv89/workflow-backend/configs"
	"github.com/tiendv89/workflow-backend/internal/adapter"
	"github.com/tiendv89/workflow-backend/internal/database"
	"github.com/tiendv89/workflow-backend/internal/handler"
	"github.com/tiendv89/workflow-backend/internal/service"
)

const maxRequestBodyBytes = 1 << 20 // 1 MB

func maxBodySizeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
		c.Next()
	}
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

func main() {
	cfgFile := "configs/config.yaml"
	if v := os.Getenv("CONFIG_FILE"); v != "" {
		cfgFile = v
	}

	cfg, err := configs.Load(cfgFile)
	if err != nil {
		// zerolog not initialized yet — use stderr directly
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	// Initialize zerolog.
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	level, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	migCtx, migCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	if err := database.RunMigrations(migCtx, cfg.Database.URL); err != nil {
		migCancel()
		log.Fatal().Err(err).Msg("migrations failed")
	}
	migCancel()

	connCtx, connCancel := context.WithTimeout(context.Background(), 15*time.Second)
	pool, err := database.Connect(connCtx, cfg.Database.URL)
	connCancel()
	if err != nil {
		log.Fatal().Err(err).Msg("database connect failed")
	}
	defer pool.Close()

	reader := database.NewReader(pool)
	adapterClient := adapter.New(cfg.API.AdapterServiceURL)
	svc := service.New(reader, adapterClient, cfg.StaleThreshold())

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
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
		Addr:         fmt.Sprintf(":%d", cfg.API.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Info().Int("port", cfg.API.Port).Msg("api-service listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("listen failed")
		}
	}()

	<-done
	log.Info().Msg("api-service: shutting down")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()

	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("shutdown error")
	}
}
