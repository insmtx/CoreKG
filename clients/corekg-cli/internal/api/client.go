package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/insmtx/corekg/clients/corekg-cli/internal/transport"
)

type APIError struct {
	Action     string
	Code       int
	Message    string
	StatusCode int
}

func (e *APIError) CLIErrorDetails() any {
	return map[string]any{
		"action":      e.Action,
		"api_code":    e.Code,
		"http_status": e.StatusCode,
	}
}

func (e *APIError) Error() string {
	if e.StatusCode != 0 && e.Code != 0 {
		return fmt.Sprintf("%s failed: HTTP %d, code %d: %s", e.Action, e.StatusCode, e.Code, e.Message)
	}
	if e.StatusCode != 0 {
		return fmt.Sprintf("%s failed: HTTP %d: %s", e.Action, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("%s failed: code %d: %s", e.Action, e.Code, e.Message)
}

type Client struct {
	*transport.Client
}

type envelope struct {
	Code     int             `json:"code"`
	Message  string          `json:"message"`
	Response json.RawMessage `json:"response"`
}

type requestEnvelope struct {
	Request any `json:"request"`
}

func New(baseURL string) (*Client, error) {
	return NewWithTimeout(baseURL, 30*time.Second)
}

func NewWithTimeout(baseURL string, timeout time.Duration) (*Client, error) {
	client, err := transport.NewWithTimeout(baseURL, timeout)
	if err != nil {
		return nil, err
	}
	return &Client{Client: client}, nil
}

func (c *Client) DoJSON(ctx context.Context, apiKey, action string, request any, response any) error {
	body, err := json.Marshal(requestEnvelope{Request: request})
	if err != nil {
		return fmt.Errorf("marshal %s request: %w", action, err)
	}
	httpResponse, err := c.Client.Do(ctx, apiKey, action, body)
	if err != nil {
		return err
	}
	return c.decodeEnvelope(action, httpResponse, response)
}

func (c *Client) DoMultipart(ctx context.Context, apiKey, action, contentType string, body io.Reader, response any) error {
	httpResponse, err := c.Client.DoReader(ctx, apiKey, action, contentType, body)
	if err != nil {
		return err
	}
	return c.decodeEnvelope(action, httpResponse, response)
}

func (c *Client) decodeEnvelope(action string, httpResponse *http.Response, response any) error {
	responseBody, err := transport.ReadBody(httpResponse, 4<<20)
	if err != nil {
		return fmt.Errorf("read %s response: %w", action, err)
	}

	var payload envelope
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return fmt.Errorf("decode %s response: %w", action, err)
	}
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		return &APIError{Action: action, Code: payload.Code, Message: responseMessage(payload.Message, responseBody), StatusCode: httpResponse.StatusCode}
	}
	if payload.Code != 0 {
		return &APIError{Action: action, Code: payload.Code, Message: payload.Message, StatusCode: httpResponse.StatusCode}
	}
	if response == nil || len(payload.Response) == 0 || string(payload.Response) == "null" {
		return nil
	}
	if err := json.Unmarshal(payload.Response, response); err != nil {
		return fmt.Errorf("decode %s response payload: %w", action, err)
	}
	return nil
}

func (c *Client) ChatCompletion(ctx context.Context, apiKey string, request any) (*ChatCompletion, error) {
	body, err := json.Marshal(requestEnvelope{Request: request})
	if err != nil {
		return nil, fmt.Errorf("marshal keapi chat completion request: %w", err)
	}
	action := "keapi.chat/chat/completions"
	httpResponse, err := c.Client.Do(ctx, apiKey, action, body)
	if err != nil {
		return nil, err
	}
	responseBody, err := transport.ReadBody(httpResponse, 8<<20)
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", action, err)
	}
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		return nil, &ChatAPIError{Action: action, StatusCode: httpResponse.StatusCode, Message: responseMessageFromChatError(responseBody)}
	}
	var completion ChatCompletion
	if err := json.Unmarshal(responseBody, &completion); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", action, err)
	}
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("decode %s response: choices is empty", action)
	}
	return &completion, nil
}

type ChatAPIError struct {
	Action     string
	StatusCode int
	Message    string
}

func (e *ChatAPIError) CLIErrorDetails() any {
	return map[string]any{
		"action":      e.Action,
		"http_status": e.StatusCode,
	}
}

func (e *ChatAPIError) Error() string {
	return fmt.Sprintf("%s failed: HTTP %d: %s", e.Action, e.StatusCode, e.Message)
}

func responseMessageFromChatError(body []byte) string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && strings.TrimSpace(payload.Error.Message) != "" {
		if strings.TrimSpace(payload.Error.Code) != "" {
			return payload.Error.Code + ": " + payload.Error.Message
		}
		return payload.Error.Message
	}
	return responseMessage("", body)
}

func responseMessage(message string, body []byte) string {
	if strings.TrimSpace(message) != "" {
		return message
	}
	const maxMessageBytes = 512
	if len(body) > maxMessageBytes {
		body = body[:maxMessageBytes]
	}
	return strings.TrimSpace(string(body))
}
