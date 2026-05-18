package server

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gateixeira/live-actions/handlers"
	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/internal/middleware"
	"github.com/gateixeira/live-actions/internal/services"
	"github.com/gateixeira/live-actions/internal/services/ghws"
	"github.com/gateixeira/live-actions/pkg/logger"
	pkgmetrics "github.com/gateixeira/live-actions/pkg/metrics"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ingestAdapter bridges the *handlers.WebhookHandler.Ingest signature into
// the ghws.Ingester interface so the ghws package has no compile-time
// dependency on the handlers package.
type ingestAdapter struct{ h *handlers.WebhookHandler }

func (a ingestAdapter) Ingest(eventType, deliveryID string, body []byte) ghws.IngestResult {
	r := a.h.Ingest(eventType, deliveryID, body)
	return ghws.IngestResult{Status: r.Status, Message: r.Message}
}

// SetupAndRun configures the router and starts the server
func SetupAndRun(staticFS embed.FS) {
	cfg, err := config.NewConfig()
	if err != nil {
		logger.InitLogger("error")
		logger.Logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	logger.InitLogger(cfg.Vars.LogLevel)
	defer logger.SyncLogger()

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Ensure data directory exists for SQLite
	dbPath := cfg.GetDatabasePath()
	if dir := filepath.Dir(dbPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			logger.Logger.Error("Failed to create data directory", zap.String("dir", dir), zap.Error(err))
		}
	}

	writeDB, readDB, err := database.InitDB(dbPath)
	if err != nil {
		logger.Logger.Error("Failed to initialize database", zap.Error(err))
		os.Exit(1)
	}

	defer func() {
		if err := writeDB.Close(); err != nil {
			logger.Logger.Error("Failed to close database write connection", zap.Error(err))
		}
		if err := readDB.Close(); err != nil {
			logger.Logger.Error("Failed to close database read connection", zap.Error(err))
		}
	}()

	db := database.NewDBWrapper(writeDB, readDB)

	ctx := context.Background()

	cleanupService := services.NewCleanupService(cfg, db, ctx)
	metricsService := services.NewMetricsUpdateService(db, 2*time.Second, ctx)

	handlers.InitSSEHandler()
	sseHandler := handlers.GetSSEHandler()
	webhookHandler := handlers.NewWebhookHandler(cfg, db)
	apiHandler := handlers.NewAPIHandler(cfg, db)
	metricsHandler := handlers.NewMetricsHandler()

	// Register dynamic Prometheus collectors now that pools and services exist.
	pmReg := pkgmetrics.GetRegistry()
	pmReg.RegisterDBStats("write", writeDB)
	pmReg.RegisterDBStats("read", readDB)
	if os := webhookHandler.OrderingService(); os != nil {
		pmReg.IngestQueueCapacity.Set(float64(os.IngestQueueCap()))
		pmReg.RegisterIngestQueueDepth(func() float64 {
			return float64(os.IngestQueueLen())
		})
	}
	pmReg.RegisterSSESubscribers(func() float64 {
		return float64(sseHandler.SubscriberCount())
	})

	r := gin.New()

	r.Use(middleware.ErrorHandler())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.SecurityLogger())
	r.Use(middleware.SecurityHeaders(cfg))
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
	r.GET("/api/analytics/failures", handlers.ValidateOrigin(), apiHandler.GetFailureAnalytics())
	r.GET("/api/analytics/labels", handlers.ValidateOrigin(), apiHandler.GetLabelDemand())
	r.GET("/api/repositories", handlers.ValidateOrigin(), apiHandler.GetRepositories())
	r.GET("/events", handlers.ValidateSSEOrigin(), sseHandler.HandleSSE())
	r.GET("/metrics", metricsHandler.Metrics())
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/readyz", handlers.ReadyzHandler(writeDB, readDB, webhookHandler.OrderingService()))

	// Serve the React SPA for all other routes
	indexHTML, err := fs.ReadFile(staticFS, "frontend/dist/index.html")
	if err != nil {
		logger.Logger.Fatal("Failed to load embedded index.html", zap.Error(err))
	}
	r.NoRoute(spaFallbackHandler(indexHTML))

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

	// Optional: open a WebSocket relay subscription so deliveries can flow
	// without a publicly reachable HTTP endpoint.
	var (
		wsCancel context.CancelFunc
		wsDone   = make(chan struct{})
	)
	if cfg.Vars.WebhookTransport == "websocket" {
		sub, err := ghws.NewSubscriber(ghws.Config{
			Token:      cfg.Vars.GitHubToken,
			Host:       cfg.Vars.GitHubHost,
			Repo:       cfg.Vars.GitHubRepo,
			Org:        cfg.Vars.GitHubOrg,
			Enterprise: cfg.Vars.GitHubEnterprise,
			Events:     splitEvents(cfg.Vars.GitHubEvents),
			Secret:     cfg.Vars.WebhookSecret,
		}, ingestAdapter{h: webhookHandler})
		if err != nil {
			logger.Logger.Fatal("Invalid WebSocket subscriber config", zap.Error(err))
		}
		var wsCtx context.Context
		wsCtx, wsCancel = context.WithCancel(context.Background())
		go func() {
			defer close(wsDone)
			if err := sub.Run(wsCtx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Logger.Error("WebSocket subscriber exited with error", zap.Error(err))
			}
		}()
		logger.Logger.Info("WebSocket transport enabled",
			zap.String("repo", cfg.Vars.GitHubRepo),
			zap.String("org", cfg.Vars.GitHubOrg),
			zap.String("enterprise", cfg.Vars.GitHubEnterprise),
			zap.String("events", cfg.Vars.GitHubEvents))
	} else {
		close(wsDone)
	}

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

	// Stop services
	if wsCancel != nil {
		wsCancel()
		<-wsDone
	}
	webhookHandler.Shutdown()
	cleanupService.Stop()
	metricsService.Stop()

	logger.Logger.Info("Server shutdown complete")
}

func spaFallbackHandler(indexHTML []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
			return
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	}
}

// splitEvents parses a comma-separated GITHUB_EVENTS value into a slice
// suitable for ghws.Config. Whitespace is trimmed and empty entries are
// dropped so trailing/duplicate commas are forgiving.
func splitEvents(s string) []string {
parts := strings.Split(s, ",")
out := make([]string, 0, len(parts))
for _, p := range parts {
if t := strings.TrimSpace(p); t != "" {
out = append(out, t)
}
}
return out
}
