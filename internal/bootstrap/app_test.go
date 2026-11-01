package bootstrap

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	srv := NewServer(":0")
	require.Equal(t, ":0", srv.Addr)

	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.JSONEq(t, `{"status":"ok"}`, string(body))
}
