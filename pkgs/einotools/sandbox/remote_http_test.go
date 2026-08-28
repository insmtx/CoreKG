package sandbox

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRemoteHTTPSandboxExecDoesNotRequireServicePool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("request method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/run" {
			t.Errorf("request path = %q, want %q", r.URL.Path, "/run")
		}
		if got := r.Header.Get("Authorization"); got != "test-token" {
			t.Errorf("authorization header = %q, want %q", got, "test-token")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if !strings.Contains(string(body), `"lang":"python"`) {
			t.Errorf("request body = %s, want python language", body)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Stdout":"ok","Stderr":"","ExitCod":0}`))
	}))
	defer server.Close()

	remote, err := NewRemoteHTTPSandbox(&Config{
		HttpBaseURL: server.URL,
		HttpToken:   "test-token",
		Timeout:     1,
	})
	if err != nil {
		t.Fatalf("create sandbox: %v", err)
	}
	if remote.(*remoteHTTPSandbox).usePool {
		t.Fatal("sandbox without service-pool configuration should fall back to direct HTTP")
	}

	result, err := remote.Exec(context.Background(), "python", "print('ok')")
	if err != nil {
		t.Fatalf("execute code: %v", err)
	}
	if result.Stdout != "ok" || result.ExitCode != 0 {
		t.Fatalf("result = %+v, want successful output", result)
	}
}

func TestRemoteHTTPSandboxCheckSyntaxDoesNotRequireServicePool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("request method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/check" {
			t.Errorf("request path = %q, want %q", r.URL.Path, "/check")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Valid":true,"Stdout":"","Stderr":"","ExitCod":0}`))
	}))
	defer server.Close()

	remote, err := NewRemoteHTTPSandbox(&Config{
		HttpBaseURL: server.URL,
		Timeout:     1,
	})
	if err != nil {
		t.Fatalf("create sandbox: %v", err)
	}

	result, err := remote.CheckSyntax(context.Background(), "python", "print('ok')")
	if err != nil {
		t.Fatalf("check syntax: %v", err)
	}
	if !result.Valid || result.ExitCode != 0 {
		t.Fatalf("result = %+v, want valid syntax", result)
	}
}
