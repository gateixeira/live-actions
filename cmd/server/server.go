package server

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gateixeira/live-actions/handlers"
	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/internal/middleware"
	"github.com/gateixeira/live-actions/internal/services"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SetupAndRun configures the router and starts the server
func SetupAndRun() {
	config := config.NewConfig()

	logger.InitLogger(config.Vars.LogLevel)
	defer logger.SyncLogger()

	if config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	err := database.InitDB(config.GetDSN())
	if err != nil {
		logger.Logger.Error("Failed to initialize database", zap.Error(err))
		os.Exit(1)
	}

	defer func() {
		if err := database.CloseDB(); err != nil {
			logger.Logger.Error("Failed to close database connection", zap.Error(err))
		}
	}()

	db := database.NewDBWrapper()

	ctx := context.Background()

	cleanupService := services.NewCleanupService(config, db, ctx)
	metricsService := services.NewMetricsUpdateService(db, 10*time.Second, ctx)
	prometheusService := services.NewPrometheusService(config.Vars.PrometheusURL)

	handlers.InitSSEHandler()
	sseHandler := handlers.GetSSEHandler()
	webhookHandler := handlers.NewWebhookHandler(config, db, prometheusService)
	apiHandler := handlers.NewAPIHandler(config, db, prometheusService)
	dashboardHandler := handlers.NewDashboardHandler(config)
	rootHandler := handlers.NewRootHandler()
	metricsHandler := handlers.NewMetricsHandler()

	r := gin.New()

	r.Use(middleware.ErrorHandler())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.SecurityLogger())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.InputValidator())

	r.Static("/static", "./static")
	r.Static("/assets", "./static/dist/assets")

	// Routes
	r.GET("/", rootHandler.Root())
	r.POST("/webhook", handlers.ValidateGitHubWebhook(config), webhookHandler.Handle())
	r.GET("/api/workflow-runs", handlers.ValidateOrigin(), apiHandler.GetWorkflowRuns())
	r.GET("/api/workflow-jobs/:run_id", handlers.ValidateOrigin(), apiHandler.GetWorkflowJobsByRunID())
	r.GET("/api/label-metrics", handlers.ValidateOrigin(), apiHandler.GetLabelMetrics())
	r.GET("/api/metrics/query_range", handlers.ValidateOrigin(), apiHandler.GetCurrentMetrics())
	r.GET("/events", sseHandler.HandleSSE())
	r.GET("/dashboard", dashboardHandler.Dashboard())
	r.GET("/metrics", metricsHandler.Metrics())

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + config.Vars.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Setup graceful shutdown
	gracefulShutdown := NewGracefulShutdown(srv, 30*time.Second)

	go cleanupService.Start()
	go metricsService.Start()
	go gracefulShutdown.Start()

	logger.Logger.Info("Starting server",
		zap.String("port", config.Vars.Port),
		zap.String("environment", config.Vars.Environment),
		zap.Bool("tls_enabled", config.Vars.TLSEnabled),
		zap.Int("data_retention_days", config.Vars.DataRetentionDays),
		zap.Int("cleanup_interval_hours", config.Vars.CleanupIntervalHours),
		zap.String("log_level", config.Vars.LogLevel),
	)

	// Start server
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Logger.Error("Failed to start server", zap.Error(err))
		os.Exit(1)
	}

	// Wait for graceful shutdown
	gracefulShutdown.Wait()

	// Stop cleanup service
	cleanupService.Stop()
	metricsService.Stop()

	logger.Logger.Info("Server shutdown complete")
}
