package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// readyzQueueFullThreshold is the queue-utilisation point at which /readyz
// starts reporting NOT READY. We deliberately fail before the queue is 100%
// full so a load balancer can pull the instance out of rotation while there
// is still headroom for in-flight deliveries.
const readyzQueueFullThreshold = 0.95

// queueStatProvider is the minimal surface /readyz needs from
// EventOrderingService; defining it as an interface here keeps this package
// from importing internal/services and makes the handler easy to test.
type queueStatProvider interface {
	IngestQueueLen() int
	IngestQueueCap() int
}

// ReadyzHandler returns a gin handler that reports readiness to serve traffic.
// It pings both DB pools (with a short timeout) and refuses traffic if the
// in-memory ingest queue is essentially full. /healthz remains a simple
// process-liveness probe; /readyz is the right thing for load balancers.
func ReadyzHandler(writeDB, readDB *sql.DB, queue queueStatProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		checks := gin.H{}
		ready := true

		if writeDB != nil {
			if err := writeDB.PingContext(ctx); err != nil {
				ready = false
				checks["write_db"] = "error: " + err.Error()
			} else {
				checks["write_db"] = "ok"
			}
		}
		if readDB != nil {
			if err := readDB.PingContext(ctx); err != nil {
				ready = false
				checks["read_db"] = "error: " + err.Error()
			} else {
				checks["read_db"] = "ok"
			}
		}

		if queue != nil {
			depth := queue.IngestQueueLen()
			capacity := queue.IngestQueueCap()
			checks["ingest_queue_depth"] = depth
			checks["ingest_queue_capacity"] = capacity
			if capacity > 0 && float64(depth)/float64(capacity) >= readyzQueueFullThreshold {
				ready = false
				checks["ingest_queue"] = "saturated"
			} else {
				checks["ingest_queue"] = "ok"
			}
		}

		status := http.StatusOK
		state := "ready"
		if !ready {
			status = http.StatusServiceUnavailable
			state = "not_ready"
		}
		c.JSON(status, gin.H{"status": state, "checks": checks})
	}
}
