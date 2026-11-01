package health

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHealthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}
