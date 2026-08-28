package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ygpkg/yg-go/logs"
	"github.com/ygpkg/yg-go/pool/svrpool"
)

type remoteHTTPSandbox struct {
	baseURL string
	token   string
	timeout time.Duration
	client  *http.Client
	usePool bool
}

const sandboxPoolKey = "sandbox"

var sandboxPool = &svrpool.PoolManager{}

func NewRemoteHTTPSandbox(cfg *Config) (Sandbox, error) {
	if cfg.HttpBaseURL == "" {
		return nil, errors.New("HttpBaseURL required")
	}
	usePool := sandboxPool.RegistryServicePool("knowledge", sandboxPoolKey)
	if !usePool {
		logs.Warnw("sandbox service pool unavailable; falling back to direct HTTP", "key", sandboxPoolKey)
	}
	cli := &http.Client{Timeout: time.Duration(cfg.Timeout) * time.Second}
	return &remoteHTTPSandbox{
		baseURL: strings.TrimRight(cfg.HttpBaseURL, "/"),
		token:   cfg.HttpToken,
		timeout: time.Duration(cfg.Timeout) * time.Second,
		client:  cli,
		usePool: usePool,
	}, nil
}

type execRequest struct {
	Lang string `json:"lang"`
	Code string `json:"code"`
}

type execResponse struct {
	Stdout   string `json:"Stdout"`
	Stderr   string `json:"Stderr"`
	ExitCode int    `json:"ExitCod"`
}

type checkResponse struct {
	Valid    bool   `json:"Valid"`
	Stdout   string `json:"Stdout"`
	Stderr   string `json:"Stderr"`
	ExitCode int    `json:"ExitCod"`
}

func (s *remoteHTTPSandbox) Exec(ctx context.Context, lang string, code string) (*ExecResult, error) {
	if s.usePool {
		id, _, err := sandboxPool.AcquireService(sandboxPoolKey, time.Second, 20)
		if err != nil {
			return &ExecResult{Stdout: "", Stderr: err.Error(), ExitCode: -1}, err
		}
		defer sandboxPool.ReleaseService(sandboxPoolKey, id)
	}

	if lang == "" {
		lang = "python"
	}
	// POST /run
	url := s.baseURL + "/run"
	payload := &execRequest{Lang: lang, Code: code}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return &ExecResult{Stdout: "", Stderr: err.Error(), ExitCode: -1}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		// req.Header.Set("Authorization", "Bearer "+s.token)
		req.Header.Set("Authorization", s.token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return &ExecResult{Stdout: "", Stderr: err.Error(), ExitCode: -1}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &ExecResult{Stdout: "", Stderr: fmt.Sprintf("http status %d", resp.StatusCode), ExitCode: -1}, errors.New("sandbox http error")
	}

	var er execResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return &ExecResult{Stdout: "", Stderr: err.Error(), ExitCode: -1}, err
	}
	return &ExecResult{Stdout: er.Stdout, Stderr: er.Stderr, ExitCode: er.ExitCode}, nil
}

func (s *remoteHTTPSandbox) CheckSyntax(ctx context.Context, lang string, code string) (*SyntaxCheckResult, error) {
	if s.usePool {
		id, _, err := sandboxPool.AcquireService(sandboxPoolKey, time.Second, 20)
		if err != nil {
			return &SyntaxCheckResult{Valid: false, Stdout: "", Stderr: err.Error(), ExitCode: -1}, err
		}
		defer sandboxPool.ReleaseService(sandboxPoolKey, id)
	}

	if lang == "" {
		lang = "python"
	}
	// POST /check
	url := s.baseURL + "/check"
	payload := &execRequest{Lang: lang, Code: code}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return &SyntaxCheckResult{Valid: false, Stdout: "", Stderr: err.Error(), ExitCode: -1}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		// req.Header.Set("Authorization", "Bearer "+s.token)
		req.Header.Set("Authorization", s.token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return &SyntaxCheckResult{Valid: false, Stdout: "", Stderr: err.Error(), ExitCode: -1}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &SyntaxCheckResult{Valid: false, Stdout: "", Stderr: fmt.Sprintf("http status %d", resp.StatusCode), ExitCode: -1}, errors.New("sandbox http error")
	}

	var cr checkResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return &SyntaxCheckResult{Valid: false, Stdout: "", Stderr: err.Error(), ExitCode: -1}, err
	}
	return &SyntaxCheckResult{Valid: cr.Valid, Stdout: cr.Stdout, Stderr: cr.Stderr, ExitCode: cr.ExitCode}, nil
}
