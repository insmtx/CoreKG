package commands

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/api"
	deviceauth "github.com/insmtx/corekg/clients/corekg-cli/internal/auth"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/clierr"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/output"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/store"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type authRow struct {
	Credential     string   `json:"credential"`
	Profiles       []string `json:"profiles,omitempty"`
	Server         string   `json:"server"`
	Organization   string   `json:"organization,omitempty"`
	OrganizationID string   `json:"organization_id,omitempty"`
	Purpose        string   `json:"purpose,omitempty"`
	Source         string   `json:"source,omitempty"`
	Current        bool     `json:"current"`
}

type authLogoutOutput struct {
	Profile  string   `json:"profile"`
	Profiles []string `json:"profiles"`
	Status   string   `json:"status"`
}

func (a *app) authCommand() *cobra.Command {
	authCommand := &cobra.Command{Use: "auth", Short: "Manage CoreKG credentials"}
	authCommand.AddCommand(a.authLoginCommand())
	authCommand.AddCommand(a.authImportCommand())
	authCommand.AddCommand(a.authListCommand())
	authCommand.AddCommand(a.authStatusCommand())
	authCommand.AddCommand(a.authLogoutCommand())
	return authCommand
}

type deviceLoginOutput struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

func (a *app) authLoginCommand() *cobra.Command {
	var name, deviceCode string
	var noBrowser, noWait bool
	command := &cobra.Command{
		Use:   "login",
		Short: "Log in through the CoreKG browser authorization flow",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if deviceCode != "" && noWait {
				return clierr.Usage("auth_login_flags_conflict", "--device-code cannot be combined with --no-wait")
			}
			paths, err := a.resolvePaths()
			if err != nil {
				return err
			}
			config, err := a.effectiveConfig()
			if err != nil {
				return err
			}
			serverURL := strings.TrimRight(strings.TrimSpace(config.Server), "/")
			if deviceCode != "" {
				storedAuth, authErr := store.LoadAuth(paths)
				if authErr != nil {
					return authErr
				}
				if pending, ok := storedAuth.Pending[loginPendingKey(deviceCode)]; ok {
					if strings.TrimSpace(pending.ServerURL) != "" {
						serverURL = pending.ServerURL
					}
					if name == "" {
						name = pending.Name
					}
				}
			}
			if serverURL == "" {
				return clierr.Usage("config_required", "CoreKG CLI is not initialized; run `corekg-cli config init`")
			}
			client, err := a.newAPIClient(serverURL)
			if err != nil {
				return clierr.New("invalid_server", err.Error())
			}

			var start *api.CLIAuthStart
			if deviceCode == "" {
				start, err = deviceauth.Start(cmd.Context(), client, "corekg-cli", a.info.Version)
				if err != nil {
					return clierr.Wrap("auth_login_start_failed", err)
				}
				verificationURI := configuredVerificationURI(config, start)
				if noWait {
					if err := a.savePendingLogin(paths, start.DeviceCode, store.PendingLogin{ServerURL: serverURL, Name: name, ExpiresAt: time.Now().Add(time.Duration(start.ExpiresIn) * time.Second)}); err != nil {
						return err
					}
					return a.writeOutput(deviceLoginOutput{DeviceCode: start.DeviceCode, UserCode: start.UserCode, VerificationURI: verificationURI, ExpiresIn: start.ExpiresIn, Interval: start.Interval})
				}
				fmt.Fprintf(a.errOut, "Open this URL to authorize CoreKG CLI:\n%s\nUser Code: %s\n", verificationURI, start.UserCode)
				if !noBrowser {
					if browserErr := deviceauth.OpenBrowser(verificationURI); browserErr != nil {
						fmt.Fprintf(a.errOut, "Warning: could not open browser automatically: %v\n", browserErr)
					}
				}
				deviceCode = start.DeviceCode
			} else {
				start = &api.CLIAuthStart{Interval: 5, ExpiresIn: 600}
			}

			result, err := deviceauth.Poll(cmd.Context(), client, deviceCode, start.Interval, start.ExpiresIn)
			if err != nil {
				return clierr.Wrap("auth_login_failed", err)
			}
			if err := a.persistDeviceLogin(paths, serverURL, name, *result, deviceCode); err != nil {
				return err
			}
			return a.writeOutput(identityOutput{Profile: resolvedLoginProfile(paths, serverURL, name, result.CompanyID), Server: serverURL, Uin: result.UIN, Organization: result.CompanyName, OrganizationID: result.CompanyID, APIKeyID: result.APIKeyID, APIKeyPurpose: result.APIKeyPurpose, Source: "device_login"})
		},
	}
	command.Flags().StringVar(&name, "name", "", "Name for the new Profile")
	command.Flags().StringVar(&deviceCode, "device-code", "", "Complete a previous device authorization")
	command.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the authorization URL without opening a browser")
	command.Flags().BoolVar(&noWait, "no-wait", false, "Start authorization and return the device code")
	return command
}

func configuredVerificationURI(config store.Config, start *api.CLIAuthStart) string {
	if start == nil || strings.TrimSpace(config.Frontend) == "" {
		if start == nil {
			return ""
		}
		return start.VerificationURI
	}
	frontend, err := url.Parse(config.Frontend)
	if err != nil {
		return start.VerificationURI
	}
	frontend.Path = strings.TrimRight(frontend.Path, "/") + "/cli/authorize"
	query := frontend.Query()
	query.Set("user_code", start.UserCode)
	frontend.RawQuery = query.Encode()
	frontend.Fragment = ""
	return frontend.String()
}

func (a *app) savePendingLogin(paths store.Paths, deviceCode string, pending store.PendingLogin) error {
	return store.WithLock(paths, func() error {
		auth, err := store.LoadAuth(paths)
		if err != nil {
			return err
		}
		auth.Pending[loginPendingKey(deviceCode)] = pending
		return store.SaveAuth(paths, auth)
	})
}

func (a *app) persistDeviceLogin(paths store.Paths, serverURL, requestedName string, result api.CLIAuthPoll, deviceCode string) error {
	name := strings.TrimSpace(requestedName)
	return store.WithLock(paths, func() error {
		settings, err := store.LoadState(paths)
		if err != nil {
			return err
		}
		auth, err := store.LoadAuth(paths)
		if err != nil {
			return err
		}
		if name == "" {
			for candidate, profile := range settings.Profiles {
				if profile.ServerURL == serverURL && profile.OrganizationID == fmt.Sprint(result.CompanyID) {
					name = candidate
					break
				}
			}
		}
		if name == "" {
			name = uniqueProfileName(settings, result.CompanyName, result.CompanyID)
		}
		if existing, ok := settings.Profiles[name]; ok && existing.OrganizationID != "" && existing.OrganizationID != fmt.Sprint(result.CompanyID) {
			return clierr.New("profile_exists", fmt.Sprintf("profile %q belongs to another organization", name))
		}
		credentialID := uuid.NewString()
		existing, hasExisting := settings.Profiles[name]
		if hasExisting && existing.ServerURL == serverURL && existing.OrganizationID == fmt.Sprint(result.CompanyID) && existing.Credential != "" {
			credentialID = existing.Credential
		}
		now := time.Now().UTC()
		previousAuth := cloneAuth(auth)
		auth = cloneAuth(auth)
		auth.Credentials[credentialID] = store.Credential{ServerURL: serverURL, APIKey: result.APIKey, Source: "device_login", APIKeyID: result.APIKeyID, APIKeyPurpose: result.APIKeyPurpose, UIN: result.UIN, OrganizationID: fmt.Sprint(result.CompanyID), OrganizationName: result.CompanyName, CreatedAt: now, UpdatedAt: now}
		profile := store.Profile{ServerURL: serverURL, Credential: credentialID, OrganizationID: fmt.Sprint(result.CompanyID), OrganizationName: result.CompanyName}
		if hasExisting && existing.ServerURL == serverURL && (existing.OrganizationID == "" || existing.OrganizationID == fmt.Sprint(result.CompanyID)) {
			profile.KnowledgeBaseID = existing.KnowledgeBaseID
			profile.KnowledgeBaseName = existing.KnowledgeBaseName
			profile.ChatSessions = existing.ChatSessions
		}
		settings.Profiles[name] = profile
		settings.CurrentProfile = name
		delete(auth.Pending, loginPendingKey(deviceCode))
		if err := store.SaveAuth(paths, auth); err != nil {
			return err
		}
		if err := store.SaveState(paths, settings); err != nil {
			_ = store.SaveAuth(paths, previousAuth)
			return err
		}
		return nil
	})
}

func resolvedLoginProfile(paths store.Paths, serverURL, requestedName string, companyID uint) string {
	name := strings.TrimSpace(requestedName)
	if name != "" {
		return name
	}
	settings, err := store.LoadState(paths)
	if err == nil {
		for candidate, profile := range settings.Profiles {
			if profile.ServerURL == serverURL && profile.OrganizationID == fmt.Sprint(companyID) {
				return candidate
			}
		}
	}
	return fmt.Sprintf("org-%d", companyID)
}

func uniqueProfileName(settings store.State, organization string, companyID uint) string {
	base := sanitizeProfileName(organization)
	if base == "" {
		base = fmt.Sprintf("org-%d", companyID)
	}
	if _, exists := settings.Profiles[base]; !exists {
		return base
	}
	for index := 2; ; index++ {
		candidate := fmt.Sprintf("%s-%d", base, index)
		if _, exists := settings.Profiles[candidate]; !exists {
			return candidate
		}
	}
}

func sanitizeProfileName(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), "-")
	value = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			return r
		}
		return -1
	}, value)
	return strings.Trim(value, "-_.")
}

func loginPendingKey(deviceCode string) string {
	digest := sha256.Sum256([]byte(deviceCode))
	return hex.EncodeToString(digest[:])
}

func (a *app) authImportCommand() *cobra.Command {
	var profileName string
	var apiKeyStdin bool
	var apiKeyEnv string
	command := &cobra.Command{
		Use:   "import",
		Short: "Import and verify an API Key",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := a.effectiveConfig()
			if err != nil {
				return err
			}
			serverURL := strings.TrimRight(strings.TrimSpace(config.Server), "/")
			profileName := strings.TrimSpace(profileName)
			if serverURL == "" {
				return clierr.Usage("config_required", "CoreKG CLI is not initialized; run `corekg-cli config init`")
			}
			if strings.TrimSpace(profileName) == "" {
				return clierr.Usage("profile_required", "--name is required")
			}
			key, err := a.readAPIKey(apiKeyStdin, apiKeyEnv)
			if err != nil {
				return err
			}
			client, err := a.newAPIClient(serverURL)
			if err != nil {
				return clierr.New("invalid_server", err.Error())
			}
			var identity api.Identity
			if err := client.DoJSON(cmd.Context(), key, "keapi.WhoAmI", map[string]any{}, &identity); err != nil {
				return clierr.Wrap("credential_verification_failed", err)
			}
			if identity.CompanyID == 0 {
				return clierr.New("organization_missing", "the API Key is not bound to an organization")
			}

			paths, err := a.resolvePaths()
			if err != nil {
				return err
			}
			if err := store.WithLock(paths, func() error {
				settings, loadErr := store.LoadState(paths)
				if loadErr != nil {
					return loadErr
				}
				if _, exists := settings.Profiles[profileName]; exists {
					return clierr.New("profile_exists", fmt.Sprintf("profile %q already exists; remove it before importing again", profileName))
				}
				auth, authErr := store.LoadAuth(paths)
				if authErr != nil {
					return authErr
				}
				previousAuth := cloneAuth(auth)
				auth = cloneAuth(auth)
				credentialID := uuid.NewString()
				now := time.Now().UTC()
				for existingID, existing := range auth.Credentials {
					if existing.ServerURL == serverURL && existing.APIKey == key {
						credentialID = existingID
						break
					}
				}
				auth.Credentials[credentialID] = store.Credential{
					ServerURL:        serverURL,
					APIKey:           key,
					Source:           "imported",
					APIKeyID:         identity.APIKeyID,
					APIKeyPurpose:    identity.APIKeyPurpose,
					OrganizationID:   fmt.Sprintf("%d", identity.CompanyID),
					OrganizationName: identity.CompanyName,
					CreatedAt:        now,
					UpdatedAt:        now,
				}
				settings.Profiles[profileName] = store.Profile{
					ServerURL:        serverURL,
					Credential:       credentialID,
					OrganizationID:   fmt.Sprintf("%d", identity.CompanyID),
					OrganizationName: identity.CompanyName,
				}
				if settings.CurrentProfile == "" {
					settings.CurrentProfile = profileName
				}
				if saveErr := store.SaveAuth(paths, auth); saveErr != nil {
					return saveErr
				}
				if saveErr := store.SaveState(paths, settings); saveErr != nil {
					_ = store.SaveAuth(paths, previousAuth)
					return saveErr
				}
				return nil
			}); err != nil {
				return err
			}
			return a.writeOutput(identityOutput{
				Profile:        profileName,
				Server:         serverURL,
				Uin:            identity.Uin,
				Organization:   identity.CompanyName,
				OrganizationID: identity.CompanyID,
				APIKeyID:       identity.APIKeyID,
				APIKeyPurpose:  identity.APIKeyPurpose,
				Source:         "imported",
			})
		},
	}
	command.Flags().StringVar(&profileName, "name", "", "Name for the new Profile")
	command.Flags().BoolVar(&apiKeyStdin, "api-key-stdin", false, "Read the API Key from stdin")
	command.Flags().StringVar(&apiKeyEnv, "api-key-env", "", "Read the API Key from the named environment variable")
	return command
}

func (a *app) authListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List locally stored credentials without revealing API Keys",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := a.resolvePaths()
			if err != nil {
				return err
			}
			settings, err := store.LoadState(paths)
			if err != nil {
				return err
			}
			auth, err := store.LoadAuth(paths)
			if err != nil {
				return err
			}
			rows := make([]authRow, 0, len(auth.Credentials))
			for credentialID, credential := range auth.Credentials {
				row := authRow{Credential: credentialID, Server: credential.ServerURL, Organization: credential.OrganizationName, OrganizationID: credential.OrganizationID, Purpose: credential.APIKeyPurpose, Source: credential.Source}
				for name, definition := range settings.Profiles {
					if definition.Credential == credentialID {
						row.Profiles = append(row.Profiles, name)
						if name == settings.CurrentProfile {
							row.Current = true
						}
					}
				}
				sort.Strings(row.Profiles)
				rows = append(rows, row)
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Current != rows[j].Current {
					return rows[i].Current
				}
				return rows[i].Credential < rows[j].Credential
			})
			return a.writeRows(rows, []string{"CURRENT", "CREDENTIAL", "PROFILES", "SERVER", "ORGANIZATION", "PURPOSE", "SOURCE"}, func(row authRow) []string {
				marker := ""
				if row.Current {
					marker = "*"
				}
				return []string{marker, row.Credential, strings.Join(row.Profiles, ","), row.Server, row.Organization, row.Purpose, row.Source}
			})
		},
	}
}

func (a *app) authStatusCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "status",
		Short: "Verify the active credential with CoreKG",
		RunE: func(cmd *cobra.Command, args []string) error {
			active, err := a.loadActiveProfile(a.profileName)
			if err != nil {
				return err
			}
			var identity api.Identity
			if err := active.Client.DoJSON(cmd.Context(), active.Credential.APIKey, "keapi.WhoAmI", map[string]any{}, &identity); err != nil {
				return clierr.Wrap("credential_verification_failed", err)
			}
			return a.writeOutput(newIdentityOutput(active.Name, active.Definition, active.Credential, identity))
		},
	}
	return command
}

func (a *app) authLogoutCommand() *cobra.Command {
	var yes, revoke bool
	command := &cobra.Command{
		Use:   "logout",
		Short: "Remove a credential and its local profiles",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return clierr.Confirm("confirmation_required", "auth logout requires --yes")
			}
			paths, err := a.resolvePaths()
			if err != nil {
				return err
			}
			selected := a.selectedProfile()
			if revoke {
				active, activeErr := a.loadActiveProfile(selected)
				if activeErr != nil {
					return activeErr
				}
				if revokeErr := active.Client.DoJSON(cmd.Context(), active.Credential.APIKey, "keapi.RevokeCurrentAPIKey", map[string]any{}, nil); revokeErr != nil {
					return clierr.Wrap("auth_revoke_failed", revokeErr)
				}
			}
			removed := make([]string, 0)
			if err := store.WithLock(paths, func() error {
				settings, loadErr := store.LoadState(paths)
				if loadErr != nil {
					return loadErr
				}
				if selected == "" {
					selected = settings.CurrentProfile
				}
				definition, ok := settings.Profiles[selected]
				if !ok {
					return clierr.New("profile_not_found", fmt.Sprintf("profile %q does not exist", selected))
				}
				auth, authErr := store.LoadAuth(paths)
				if authErr != nil {
					return authErr
				}
				for name, profile := range settings.Profiles {
					if profile.Credential == definition.Credential {
						removed = append(removed, name)
						delete(settings.Profiles, name)
					}
				}
				sort.Strings(removed)
				delete(auth.Credentials, definition.Credential)
				settings.CurrentProfile = ""
				for _, name := range store.ProfileNames(settings) {
					settings.CurrentProfile = name
					break
				}
				if saveErr := store.SaveState(paths, settings); saveErr != nil {
					return saveErr
				}
				return store.SaveAuth(paths, auth)
			}); err != nil {
				return err
			}
			return a.writeOutput(authLogoutOutput{Profile: selected, Profiles: removed, Status: "logged_out"})
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "Confirm removing the credential and profiles")
	command.Flags().BoolVar(&revoke, "revoke", false, "Revoke the selected API Key on the server before removing it")
	return command
}

func (a *app) readAPIKey(fromStdin bool, envName string) (string, error) {
	if fromStdin && strings.TrimSpace(envName) != "" {
		return "", clierr.Usage("api_key_sources_conflict", "use only one of --api-key-stdin and --api-key-env")
	}
	if strings.TrimSpace(envName) != "" {
		key := strings.TrimSpace(os.Getenv(envName))
		if key == "" {
			return "", clierr.Usage("api_key_empty", fmt.Sprintf("environment variable %q is empty", envName))
		}
		return key, nil
	}
	if fromStdin {
		reader := bufio.NewReader(a.in)
		key, err := io.ReadAll(reader)
		if err != nil {
			return "", clierr.New("api_key_read_failed", err.Error())
		}
		key = []byte(strings.TrimSpace(string(key)))
		if len(key) == 0 {
			return "", clierr.Usage("api_key_empty", "API Key from stdin is empty")
		}
		return string(key), nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", clierr.Usage("api_key_input_required", "use --api-key-stdin when stdin is not a terminal")
	}
	_, _ = fmt.Fprint(a.errOut, "API Key: ")
	key, err := term.ReadPassword(int(os.Stdin.Fd()))
	_, _ = fmt.Fprintln(a.errOut)
	if err != nil {
		return "", clierr.New("api_key_read_failed", err.Error())
	}
	if strings.TrimSpace(string(key)) == "" {
		return "", clierr.Usage("api_key_empty", "API Key is empty")
	}
	return strings.TrimSpace(string(key)), nil
}

func cloneAuth(value store.Auth) store.Auth {
	value.Credentials = make(map[string]store.Credential, len(value.Credentials))
	for key, credential := range value.Credentials {
		value.Credentials[key] = credential
	}
	value.Pending = make(map[string]store.PendingLogin, len(value.Pending))
	for key, pending := range value.Pending {
		value.Pending[key] = pending
	}
	return value
}

func (a *app) writeOutput(value any) error {
	format, err := a.format()
	if err != nil {
		return err
	}
	if format == "json" {
		return output.WriteJSON(a.out, value)
	}
	if format == "id" {
		switch row := value.(type) {
		case identityOutput:
			return output.WriteNames(a.out, []string{row.Profile})
		case authLogoutOutput:
			return output.WriteNames(a.out, []string{row.Profile})
		case kbSelectionOutput:
			return output.WriteNames(a.out, []string{row.KnowledgeBaseID})
		}
		return output.WriteNames(a.out, []string{fmt.Sprint(value)})
	}
	if row, ok := value.(map[string]string); ok {
		keys := make([]string, 0, len(row))
		for key := range row {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		tableRows := make([][]string, 0, len(keys))
		for _, key := range keys {
			tableRows = append(tableRows, []string{key, row[key]})
		}
		return output.WriteTable(a.out, []string{"FIELD", "VALUE"}, tableRows)
	}
	if row, ok := value.(identityOutput); ok {
		return output.WriteTable(a.out, []string{"PROFILE", "SERVER", "UIN", "ORGANIZATION", "ORGANIZATION ID", "API KEY ID", "PURPOSE", "SOURCE"}, [][]string{{row.Profile, row.Server, fmt.Sprint(row.Uin), row.Organization, fmt.Sprint(row.OrganizationID), fmt.Sprint(row.APIKeyID), row.APIKeyPurpose, row.Source}})
	}
	if row, ok := value.(authLogoutOutput); ok {
		return output.WriteTable(a.out, []string{"PROFILE", "REMOVED PROFILES", "STATUS"}, [][]string{{row.Profile, strings.Join(row.Profiles, ","), row.Status}})
	}
	if row, ok := value.(kbSelectionOutput); ok {
		return output.WriteTable(a.out, []string{"PROFILE", "KNOWLEDGE BASE ID", "KNOWLEDGE BASE NAME"}, [][]string{{row.Profile, row.KnowledgeBaseID, row.KnowledgeBaseName}})
	}
	_, err = fmt.Fprintln(a.out, value)
	return err
}

func (a *app) writeRows(rows []authRow, headers []string, values func(authRow) []string) error {
	format, err := a.format()
	if err != nil {
		return err
	}
	if format == "json" {
		return output.WriteJSON(a.out, rows)
	}
	if format == "id" {
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, row.Credential)
		}
		return output.WriteNames(a.out, names)
	}
	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, values(row))
	}
	if len(tableRows) == 0 {
		_, err := fmt.Fprintln(a.out, "No credentials configured.")
		return err
	}
	return output.WriteTable(a.out, headers, tableRows)
}
