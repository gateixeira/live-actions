package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupRootTest() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewRootHandler()
	router.GET("/", handler.Root())
	return router
}

func TestNewRootHandler(t *testing.T) {
	handler := NewRootHandler()
	assert.NotNil(t, handler, "NewRootHandler should return a non-nil handler")
}

func TestRoot(t *testing.T) {
	router := setupRootTest()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Response status code should be 200")
	assert.Equal(t, "Live Actions - GitHub Actions Runner Monitoring", w.Body.String(), "Response body should match expected string")
}
