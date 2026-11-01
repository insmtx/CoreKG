package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Register wires the health endpoint onto the engine.
func Register(eng *gin.Engine) { eng.GET("/healthz", Handler) }

// Handler reports liveness.
func Handler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
