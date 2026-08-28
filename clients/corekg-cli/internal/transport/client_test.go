package transport

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewRejectsUnsafeBaseURLParts(t *testing.T) {
	for _, value := range []string{
		"https://user:password@example.com",
		"https://example.com?tenant=one",
		"https://example.com#fragment",
	} {
		_, err := New(value)
		require.Error(t, err, value)
	}
}

func TestNewUsesConfiguredTimeoutAndRejectsRedirects(t *testing.T) {
	client, err := NewWithTimeout("https://example.com", time.Second)
	require.NoError(t, err)
	require.Equal(t, time.Second, client.HTTPClient.Timeout)
	require.ErrorIs(t, client.HTTPClient.CheckRedirect(nil, nil), http.ErrUseLastResponse)
}

func TestReadBodyRejectsOversizedResponse(t *testing.T) {
	response := &http.Response{Body: io.NopCloser(strings.NewReader("12345"))}
	_, err := ReadBody(response, 4)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds")
}
