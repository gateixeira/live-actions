package server

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
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
func SetupAndRun(staticFS embed.FS) {
	cfg := config.NewConfig()

	logger.InitLogger(cfg.Vars.LogLevel)
	defer logger.SyncLogger()

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Ensure data directory exists for SQLite
	dbPath := cfg.GetDatabasePath()
	if dir := filepath.Dir(dbPath); dir != "." && dir != "" {
		os.MkdirAll(dir, 0700)
	}

	err := database.InitDB(dbPath)
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

	cleanupService := services.NewCleanupService(cfg, db, ctx)
	metricsService := services.NewMetricsUpdateService(db, 10*time.Second, ctx)

	handlers.InitSSEHandler()
	sseHandler := handlers.GetSSEHandler()
	webhookHandler := handlers.NewWebhookHandler(cfg, db)
	apiHandler := handlers.NewAPIHandler(cfg, db)
	metricsHandler := handlers.NewMetricsHandler()

	r := gin.New()

	r.Use(middleware.ErrorHandler())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.SecurityLogger())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.InputValidator())

	// Serve static assets from embedded FS
	distFS, err := fs.Sub(staticFS, "frontend/dist")
	if err != nil {
		logger.Logger.Fatal("Failed to load embedded frontend/dist", zap.Error(err))
	}
	assetsFS, err := fs.Sub(staticFS, "frontend/dist/assets")
	if err != nil {
		logger.Logger.Fatal("Failed to load embedded frontend/dist/assets", zap.Error(err))
	}
	r.StaticFS("/static", http.FS(distFS))
	r.StaticFS("/assets", http.FS(assetsFS))

	// Routes
	r.POST("/webhook", handlers.ValidateGitHubWebhook(cfg), webhookHandler.Handle())
	r.GET("/api/csrf", apiHandler.GetCSRFToken())
	r.GET("/api/workflow-runs", handlers.ValidateOrigin(), apiHandler.GetWorkflowRuns())
	r.GET("/api/workflow-jobs/:run_id", handlers.ValidateOrigin(), apiHandler.GetWorkflowJobsByRunID())
	r.GET("/api/metrics/query_range", handlers.ValidateOrigin(), apiHandler.GetCurrentMetrics())
	r.GET("/events", sseHandler.HandleSSE())
	r.GET("/metrics", metricsHandler.Metrics())

	// Serve the React SPA for all other routes
	indexHTML, err := fs.ReadFile(staticFS, "frontend/dist/index.html")
	if err != nil {
		logger.Logger.Fatal("Failed to load embedded index.html", zap.Error(err))
	}
	r.NoRoute(func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Vars.Port,
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
		zap.String("port", cfg.Vars.Port),
		zap.String("environment", cfg.Vars.Environment),
		zap.Bool("tls_enabled", cfg.Vars.TLSEnabled),
		zap.Int("data_retention_days", cfg.Vars.DataRetentionDays),
		zap.Int("cleanup_interval_hours", cfg.Vars.CleanupIntervalHours),
		zap.String("log_level", cfg.Vars.LogLevel),
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
