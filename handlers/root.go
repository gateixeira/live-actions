package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type RootHandler struct{}

func NewRootHandler() *RootHandler {
	return &RootHandler{}
}

// Root serves the root endpoint
func (h *RootHandler) Root() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.String(http.StatusOK, "Live Actions - GitHub Actions Runner Monitoring")
	}
}
