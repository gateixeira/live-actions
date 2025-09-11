package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsHandler struct{}

func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{}
}

// Metrics serves the Prometheus metrics endpoint
func (h *MetricsHandler) Metrics() gin.HandlerFunc {
	promHandler := promhttp.Handler()

	return gin.WrapH(promHandler)
}
