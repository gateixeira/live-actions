package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

type fakeQueueStats struct {
	length, capacity int
}

func (f fakeQueueStats) IngestQueueLen() int { return f.length }
func (f fakeQueueStats) IngestQueueCap() int { return f.capacity }

func newInMemoryDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newClosedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Close())
	return db
}

func runReadyz(t *testing.T, h gin.HandlerFunc) (int, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/readyz", h)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/readyz", nil)
	r.ServeHTTP(w, req)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return w.Code, body
}

func TestReadyzHandler_HealthyAllChecks(t *testing.T) {
	writeDB := newInMemoryDB(t)
	readDB := newInMemoryDB(t)
	queue := fakeQueueStats{length: 10, capacity: 1000}

	code, body := runReadyz(t, ReadyzHandler(writeDB, readDB, queue))

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ready", body["status"])
	checks := body["checks"].(map[string]any)
	assert.Equal(t, "ok", checks["write_db"])
	assert.Equal(t, "ok", checks["read_db"])
	assert.Equal(t, "ok", checks["ingest_queue"])
}

func TestReadyzHandler_WriteDBDown(t *testing.T) {
	writeDB := newClosedDB(t)
	readDB := newInMemoryDB(t)
	queue := fakeQueueStats{length: 0, capacity: 1000}

	code, body := runReadyz(t, ReadyzHandler(writeDB, readDB, queue))

	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "not_ready", body["status"])
	checks := body["checks"].(map[string]any)
	assert.Contains(t, checks["write_db"], "error")
	assert.Equal(t, "ok", checks["read_db"])
}

func TestReadyzHandler_QueueSaturated(t *testing.T) {
	writeDB := newInMemoryDB(t)
	readDB := newInMemoryDB(t)
	// 950 / 1000 = 0.95 → saturated.
	queue := fakeQueueStats{length: 950, capacity: 1000}

	code, body := runReadyz(t, ReadyzHandler(writeDB, readDB, queue))

	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "not_ready", body["status"])
	checks := body["checks"].(map[string]any)
	assert.Equal(t, "saturated", checks["ingest_queue"])
}

func TestReadyzHandler_NilQueueIsTolerated(t *testing.T) {
	writeDB := newInMemoryDB(t)
	readDB := newInMemoryDB(t)

	code, body := runReadyz(t, ReadyzHandler(writeDB, readDB, nil))

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ready", body["status"])
	checks := body["checks"].(map[string]any)
	_, hasQueueKey := checks["ingest_queue"]
	assert.False(t, hasQueueKey, "queue checks should be omitted when no provider is supplied")
}
