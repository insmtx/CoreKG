package auth

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/insmtx/corekg/clients/corekg-cli/internal/api"
	"github.com/stretchr/testify/require"
)

func TestStartUsesUnauthenticatedDeviceAuthorizationRequest(t *testing.T) {
	client, err := api.New("https://corekg.example.com")
	require.NoError(t, err)
	client.HTTPClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "/v3/keapi.CLIAuthStart", request.URL.Path)
		require.Empty(t, request.Header.Get("Authorization"))
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), `"client_name":"corekg-cli"`)
		require.Contains(t, string(body), `"cli_version":"test"`)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"code":0,"response":{"device_code":"device","user_code":"ABCD2345","verification_uri":"https://corekg.example.com/cli/authorize?user_code=ABCD2345","expires_in":600,"interval":5}}`)),
			Request:    request,
		}, nil
	})

	result, err := Start(t.Context(), client, "corekg-cli", "test")
	require.NoError(t, err)
	require.Equal(t, "device", result.DeviceCode)
	require.Equal(t, "ABCD2345", result.UserCode)
	require.Equal(t, 5, result.Interval)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
