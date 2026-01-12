package httpserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleHealth handles health check requests
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}
