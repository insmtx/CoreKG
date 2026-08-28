package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientDoJSONUsesCoreKGEnvelopeAndBearer(t *testing.T) {
	client, err := New("https://corekg.example.com")
	require.NoError(t, err)
	client.HTTPClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "/v3/keapi.WhoAmI", request.URL.Path)
		require.Equal(t, "Bearer yg-test", request.Header.Get("Authorization"))
		var body struct {
			Request map[string]any `json:"request"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		require.Equal(t, "value", body.Request["key"])
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"code":0,"message":"","response":{"company_id":7}}`)),
			Request:    request,
		}, nil
	})
	var response struct {
		CompanyID uint `json:"company_id"`
	}
	require.NoError(t, client.DoJSON(context.Background(), "yg-test", "keapi.WhoAmI", map[string]string{"key": "value"}, &response))
	require.Equal(t, uint(7), response.CompanyID)
}

func TestClientRejectsCrossOriginRedirect(t *testing.T) {
	client, err := New("https://corekg.example.com")
	require.NoError(t, err)
	previous, err := http.NewRequest(http.MethodPost, "https://corekg.example.com/v3/keapi.WhoAmI", nil)
	require.NoError(t, err)
	redirect, err := http.NewRequest(http.MethodPost, "https://attacker.example.com/collect", nil)
	require.NoError(t, err)
	err = client.HTTPClient.CheckRedirect(redirect, []*http.Request{previous})
	require.ErrorIs(t, err, http.ErrUseLastResponse)
}

func TestClientChatCompletionUsesOpenAIResponse(t *testing.T) {
	client, err := New("https://corekg.example.com")
	require.NoError(t, err)
	client.HTTPClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "/v3/keapi.chat/chat/completions", request.URL.Path)
		require.Equal(t, "Bearer yg-test", request.Header.Get("Authorization"))
		var body struct {
			Request struct {
				SessionID uint `json:"session_id"`
			} `json:"request"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		require.Equal(t, uint(9), body.Request.SessionID)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"id":"message-1","model":"forest-chat","choices":[{"index":0,"message":{"role":"assistant","content":"answer"},"finish_reason":"stop"}]}`)),
			Request:    request,
		}, nil
	})
	completion, err := client.ChatCompletion(context.Background(), "yg-test", map[string]any{"session_id": 9})
	require.NoError(t, err)
	require.Equal(t, "message-1", completion.ID)
	require.Equal(t, "answer", completion.Choices[0].Message.Content)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
