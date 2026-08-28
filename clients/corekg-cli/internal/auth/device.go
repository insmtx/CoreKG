package auth

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/insmtx/corekg/clients/corekg-cli/internal/api"
)

func Start(ctx context.Context, client *api.Client, clientName, cliVersion string) (*api.CLIAuthStart, error) {
	var result api.CLIAuthStart
	if err := client.DoJSON(ctx, "", "keapi.CLIAuthStart", map[string]any{
		"client_name": clientName,
		"cli_version": cliVersion,
	}, &result); err != nil {
		return nil, err
	}
	if result.DeviceCode == "" || result.VerificationURI == "" {
		return nil, fmt.Errorf("server returned an incomplete device authorization")
	}
	return &result, nil
}

func Poll(ctx context.Context, client *api.Client, deviceCode string, interval, expiresIn int) (*api.CLIAuthPoll, error) {
	if interval < 1 {
		interval = 5
	}
	if expiresIn < 1 {
		expiresIn = 600
	}
	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-time.After(time.Duration(interval) * time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		var result api.CLIAuthPoll
		if err := client.DoJSON(ctx, "", "keapi.CLIAuthPoll", map[string]any{"device_code": deviceCode}, &result); err != nil {
			return nil, err
		}
		switch result.Status {
		case "approved":
			if result.APIKey == "" {
				return nil, fmt.Errorf("server approved login without an API Key")
			}
			return &result, nil
		case "denied":
			return nil, fmt.Errorf("browser authorization was denied")
		case "expired":
			return nil, fmt.Errorf("browser authorization expired")
		case "pending":
			continue
		default:
			return nil, fmt.Errorf("server returned unknown authorization status %q", result.Status)
		}
	}
	return nil, fmt.Errorf("browser authorization expired")
}

func OpenBrowser(url string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		command, args = "open", []string{url}
	case "linux":
		command, args = "xdg-open", []string{url}
	case "windows":
		command, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		return fmt.Errorf("browser opening is not supported on %s", runtime.GOOS)
	}
	return exec.Command(command, args...).Start()
}
