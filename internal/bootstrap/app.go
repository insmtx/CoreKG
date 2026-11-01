package bootstrap

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/insmtx/corekg/internal/api/health"
)

// NewServer builds the corekg HTTP server with all routes registered.
func NewServer(addr string) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	eng := gin.New()
	eng.Use(gin.Recovery())
	health.Register(eng)
	return &http.Server{
		Addr:              addr,
		Handler:           eng,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
