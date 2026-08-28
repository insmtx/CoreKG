package commands

import (
	"fmt"
	"time"

	"github.com/insmtx/corekg/clients/corekg-cli/internal/api"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/clierr"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/store"
)

type activeProfile struct {
	Paths      store.Paths
	Settings   store.State
	Name       string
	Definition store.Profile
	Auth       store.Auth
	Credential store.Credential
	Client     *api.Client
}

func (a *app) loadActiveProfile(name string) (*activeProfile, error) {
	paths, err := a.resolvePaths()
	if err != nil {
		return nil, err
	}
	settings, err := store.LoadState(paths)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = a.selectedProfile()
	}
	if name == "" {
		name = settings.CurrentProfile
	}
	if name == "" {
		return nil, clierr.New("auth_required", "no active profile; run auth import or auth login")
	}
	definition, ok := settings.Profiles[name]
	if !ok {
		return nil, clierr.New("profile_not_found", fmt.Sprintf("profile %q does not exist", name))
	}
	auth, err := store.LoadAuth(paths)
	if err != nil {
		return nil, err
	}
	credential, ok := auth.Credentials[definition.Credential]
	if !ok || credential.APIKey == "" {
		return nil, clierr.New("credential_not_found", fmt.Sprintf("credential for profile %q does not exist", name))
	}
	if credential.ServerURL != "" && credential.ServerURL != definition.ServerURL {
		return nil, clierr.New("profile_credential_mismatch", fmt.Sprintf("profile %q and its credential use different servers", name))
	}
	client, err := a.newAPIClient(definition.ServerURL)
	if err != nil {
		return nil, clierr.New("invalid_server", err.Error())
	}
	return &activeProfile{
		Paths:      paths,
		Settings:   settings,
		Name:       name,
		Definition: definition,
		Auth:       auth,
		Credential: credential,
		Client:     client,
	}, nil
}

func (a *app) loadActiveProfileForOperation(name string, defaultTimeout, commandTimeout time.Duration) (*activeProfile, error) {
	active, err := a.loadActiveProfile(name)
	if err != nil {
		return nil, err
	}

	timeout, err := a.operationTimeout(defaultTimeout, commandTimeout)
	if err != nil {
		return nil, err
	}
	client, err := a.newAPIClientWithTimeout(active.Definition.ServerURL, timeout)
	if err != nil {
		return nil, clierr.New("invalid_server", err.Error())
	}
	active.Client = client
	return active, nil
}

func (a *app) operationTimeout(defaultTimeout, commandTimeout time.Duration) (time.Duration, error) {
	if commandTimeout != 0 {
		if commandTimeout < 0 {
			return 0, fmt.Errorf("HTTP request timeout must be positive")
		}
		return commandTimeout, nil
	}
	if a.timeout != 0 {
		if a.timeout < 0 {
			return 0, fmt.Errorf("HTTP request timeout must be positive")
		}
		return a.timeout, nil
	}
	config, err := a.effectiveConfig()
	if err != nil {
		return 0, err
	}
	timeout, err := config.TimeoutDuration()
	if err != nil {
		return 0, err
	}
	if timeout < defaultTimeout {
		return defaultTimeout, nil
	}
	return timeout, nil
}

type identityOutput struct {
	Profile        string `json:"profile"`
	Server         string `json:"server"`
	Uin            uint   `json:"uin"`
	Organization   string `json:"organization"`
	OrganizationID uint   `json:"organization_id"`
	APIKeyID       uint   `json:"api_key_id,omitempty"`
	APIKeyPurpose  string `json:"api_key_purpose,omitempty"`
	Source         string `json:"source,omitempty"`
}

func newIdentityOutput(name string, definition store.Profile, credential store.Credential, identity api.Identity) identityOutput {
	return identityOutput{
		Profile:        name,
		Server:         definition.ServerURL,
		Uin:            identity.Uin,
		Organization:   identity.CompanyName,
		OrganizationID: identity.CompanyID,
		APIKeyID:       identity.APIKeyID,
		APIKeyPurpose:  identity.APIKeyPurpose,
		Source:         credential.Source,
	}
}
