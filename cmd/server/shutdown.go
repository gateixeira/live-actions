package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gateixeira/live-actions/pkg/logger"
	"go.uber.org/zap"
)

// GracefulShutdown handles graceful server shutdown
type GracefulShutdown struct {
	server   *http.Server
	timeout  time.Duration
	shutdown chan struct{}
}

// NewGracefulShutdown creates a new graceful shutdown handler
func NewGracefulShutdown(server *http.Server, timeout time.Duration) *GracefulShutdown {
	return &GracefulShutdown{
		server:   server,
		timeout:  timeout,
		shutdown: make(chan struct{}),
	}
}

// Start begins listening for shutdown signals
func (gs *GracefulShutdown) Start() {
	// Create a channel to receive OS signals
	sigChan := make(chan os.Signal, 1)

	// Register the channel to receive specific signals
	signal.Notify(sigChan,
		syscall.SIGINT,  // Ctrl+C
		syscall.SIGTERM, // Termination signal
		syscall.SIGQUIT, // Quit signal
	)

	// Start a goroutine to handle shutdown
	go func() {
		// Wait for a signal
		sig := <-sigChan
		logger.Logger.Info("Received shutdown signal", zap.String("signal", sig.String()))

		// Start graceful shutdown
		gs.shutdown <- struct{}{}

		// Create a context with timeout for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), gs.timeout)
		defer cancel()

		// Attempt to gracefully shutdown the server
		logger.Logger.Info("Starting graceful shutdown...", zap.Duration("timeout", gs.timeout))

		if err := gs.server.Shutdown(ctx); err != nil {
			logger.Logger.Error("Server forced to shutdown", zap.Error(err))
		} else {
			logger.Logger.Info("Server gracefully stopped")
		}
	}()
}

// Wait blocks until shutdown is initiated
func (gs *GracefulShutdown) Wait() {
	<-gs.shutdown
}

// IsShuttingDown returns true if shutdown has been initiated
func (gs *GracefulShutdown) IsShuttingDown() bool {
	select {
	case <-gs.shutdown:
		return true
	default:
		return false
	}
}
