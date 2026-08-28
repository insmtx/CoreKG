package commands

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insmtx/corekg/clients/corekg-cli/internal/api"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/buildinfo"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/clierr"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/store"
	"github.com/stretchr/testify/require"
)

func TestVersionOutput(t *testing.T) {
	info := buildinfo.Info{
		Name:      "corekg-cli",
		Version:   "2.1.206",
		GitCommit: "abc123",
		BuiltAt:   "2026-08-27T00:00:00Z",
	}

	for _, args := range [][]string{{"-v"}, {"--version"}, {"version"}} {
		var output bytes.Buffer
		root := NewRootWithOptions(info, Options{Paths: store.NewPaths(t.TempDir()), Out: &output, ErrOut: &output})
		root.SetArgs(args)
		require.NoError(t, root.Execute())
		require.Equal(t, "2.1.206 (CoreKG CLI)\n", output.String())
	}
}

func TestVersionJSON(t *testing.T) {
	info := buildinfo.Info{
		Name:      "corekg-cli",
		Version:   "2.1.206",
		GitCommit: "abc123",
		BuiltAt:   "2026-08-27T00:00:00Z",
	}
	var output bytes.Buffer
	root := NewRootWithOptions(info, Options{Paths: store.NewPaths(t.TempDir()), Out: &output, ErrOut: &output})
	root.SetArgs([]string{"version", "--output", "json"})
	require.NoError(t, root.Execute())
	require.JSONEq(t, `{"name":"CoreKG CLI","version":"2.1.206"}`, output.String())
}

func TestProfileListAndUse(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	settings := store.NewState()
	settings.Contexts["default"] = store.Context{ServerURL: "https://corekg.example.com", Credential: "default"}
	settings.Contexts["work"] = store.Context{ServerURL: "https://work.example.com", Credential: "work"}
	require.NoError(t, store.SaveState(paths, settings))

	var output bytes.Buffer
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths, Out: &output, ErrOut: &output})
	root.SetArgs([]string{"profile", "list", "--output", "id"})
	require.NoError(t, root.Execute())
	require.Equal(t, "default\nwork\n", output.String())

	output.Reset()
	root = NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths, Out: &output, ErrOut: &output})
	root.SetArgs([]string{"profile", "use", "work", "--output", "json"})
	require.NoError(t, root.Execute())
	require.Contains(t, output.String(), `"current_profile": "work"`)

	loaded, err := store.LoadState(paths)
	require.NoError(t, err)
	require.Equal(t, "work", loaded.CurrentProfile)

	output.Reset()
	root = NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths, Out: &output, ErrOut: &output})
	root.SetArgs([]string{"profile", "use", "default", "--output", "json"})
	require.NoError(t, root.Execute())

	output.Reset()
	root = NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths, Out: &output, ErrOut: &output})
	root.SetArgs([]string{"profile", "use", "work", "--output", "json"})
	require.NoError(t, root.Execute())

	output.Reset()
	root = NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths, Out: &output, ErrOut: &output})
	root.SetArgs([]string{"profile", "use", "-", "--output", "json"})
	require.NoError(t, root.Execute())
	require.Contains(t, output.String(), `"current_profile": "default"`)
}

func TestConfigPath(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	var output bytes.Buffer
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths, Out: &output, ErrOut: &output})
	root.SetArgs([]string{"config", "path", "--output", "json"})
	require.NoError(t, root.Execute())
	require.Contains(t, output.String(), paths.ConfigFile)
	require.Contains(t, output.String(), paths.AuthFile)
	require.Contains(t, output.String(), `"ok": true`)
	_, err := os.Stat(paths.RootDir)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestConfigInitUsesDefaultsAndWritesOnlyConfig(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	var output, prompts bytes.Buffer
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{
		Paths:  paths,
		In:     strings.NewReader("\n\n\n\n\n"),
		Out:    &output,
		ErrOut: &prompts,
	})
	root.SetArgs([]string{"config", "init"})
	require.NoError(t, root.Execute())
	require.Contains(t, prompts.String(), "CoreKG server [http://127.0.0.1:8080]:")
	require.Contains(t, prompts.String(), "CoreKG frontend [http://localhost:3001]:")
	require.Contains(t, prompts.String(), "Default output [table]:")
	require.Contains(t, prompts.String(), "HTTP timeout [30s]:")
	require.Contains(t, prompts.String(), "Save configuration? [Y/n]:")
	config, err := store.LoadConfig(paths)
	require.NoError(t, err)
	require.Equal(t, store.DefaultConfig(), config)
	_, err = os.Stat(paths.StateFile)
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(paths.AuthFile)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestConfigInitAcceptsValuesAndCanBeCancelled(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	var output bytes.Buffer
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{
		Paths:  paths,
		In:     strings.NewReader("https://api.corekg.example.com\nhttps://corekg.example.com\njson\n2s\ny\n"),
		Out:    &output,
		ErrOut: &output,
	})
	root.SetArgs([]string{"config", "init"})
	require.NoError(t, root.Execute())
	config, err := store.LoadConfig(paths)
	require.NoError(t, err)
	require.Equal(t, "https://api.corekg.example.com", config.Server)
	require.Equal(t, "https://corekg.example.com", config.Frontend)
	require.Equal(t, "json", config.Output)
	require.Equal(t, "2s", config.Timeout)

	root = NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{
		Paths:  paths,
		In:     strings.NewReader("https://other.example.com\nhttps://other-frontend.example.com\n\n\nno\n"),
		Out:    &output,
		ErrOut: &output,
	})
	root.SetArgs([]string{"config", "init"})
	err = root.Execute()
	require.Equal(t, clierr.ExitConfirm, clierr.ExitCode(err))
	config, err = store.LoadConfig(paths)
	require.NoError(t, err)
	require.Equal(t, "https://api.corekg.example.com", config.Server)
	require.Equal(t, "https://corekg.example.com", config.Frontend)
}

func TestConfigSourceCanBeInlineOrFileAndInitRejectsIt(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	var output bytes.Buffer
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths, Out: &output, ErrOut: &output})
	root.SetArgs([]string{"--config", `{"output":"json","timeout":"1s"}`, "config", "path"})
	require.NoError(t, root.Execute())
	require.Contains(t, output.String(), `"config_file"`)

	output.Reset()
	root = NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths, In: strings.NewReader("\n\n\n\n"), Out: &output, ErrOut: &output})
	root.SetArgs([]string{"--config", `{"server":"https://example.com"}`, "config", "init"})
	err := root.Execute()
	require.Equal(t, clierr.ExitUsage, clierr.ExitCode(err))
	_, err = os.Stat(paths.ConfigFile)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestConfigCommandOnlyExposesInitAndPath(t *testing.T) {
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: store.NewPaths(t.TempDir())})
	command, _, err := root.Find([]string{"config"})
	require.NoError(t, err)
	names := make([]string, 0, len(command.Commands()))
	for _, child := range command.Commands() {
		names = append(names, child.Name())
	}
	require.ElementsMatch(t, []string{"init", "path"}, names)
}

func TestLoginRequiresConfigWhenNoDefaultExists(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths})
	root.SetArgs([]string{"auth", "login", "--no-browser", "--no-wait"})
	err := root.Execute()
	require.Equal(t, clierr.ExitUsage, clierr.ExitCode(err))
	require.Contains(t, err.Error(), "config_required")
}

func TestConfiguredVerificationURIUsesFrontendAddress(t *testing.T) {
	start := &api.CLIAuthStart{
		UserCode:        "ABC 123",
		VerificationURI: "https://api.corekg.example.com/cli/authorize?user_code=ABC+123",
	}
	got := configuredVerificationURI(store.Config{Frontend: "https://corekg.example.com/console"}, start)
	require.Equal(t, "https://corekg.example.com/console/cli/authorize?user_code=ABC+123", got)

	start.VerificationURI = "https://api.corekg.example.com/cli/authorize?user_code=ABC+123"
	require.Equal(t, start.VerificationURI, configuredVerificationURI(store.Config{}, start))
}

func TestAuthImportUsesCustomConfigServerAndWritesOnlyStateAndAuth(t *testing.T) {
	var seenPath, seenAuthorization string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seenPath = request.URL.Path
		seenAuthorization = request.Header.Get("Authorization")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(writer, `{"code":0,"response":{"uin":1,"company_id":7,"company_name":"Acme","api_key_id":8,"api_key_purpose":"corekg_cli"}}`)
	}))
	defer server.Close()

	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	t.Setenv("COREKG_TEST_API_KEY", "test-key")
	var output bytes.Buffer
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths, Out: &output, ErrOut: &output})
	root.SetArgs([]string{"--config", fmt.Sprintf(`{"server":%q,"timeout":"1s"}`, server.URL), "auth", "import", "--name", "work", "--api-key-env", "COREKG_TEST_API_KEY", "--output", "json"})
	require.NoError(t, root.Execute())
	require.Equal(t, "/v3/keapi.WhoAmI", seenPath)
	require.Equal(t, "Bearer test-key", seenAuthorization)
	_, err := os.Stat(paths.ConfigFile)
	require.ErrorIs(t, err, os.ErrNotExist)
	state, err := store.LoadState(paths)
	require.NoError(t, err)
	require.Contains(t, state.Profiles, "work")
	auth, err := store.LoadAuth(paths)
	require.NoError(t, err)
	require.Len(t, auth.Credentials, 1)
}

func TestPublicCommandFlagsAreRegistered(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths})
	for _, commandPath := range [][]string{{"kb", "create"}, {"kb", "list"}, {"profile", "delete"}, {"auth", "logout"}, {"file", "list"}, {"file", "upload"}, {"ask"}} {
		command, _, err := root.Find(commandPath)
		require.NoError(t, err)
		switch commandPath[0] {
		case "kb":
			if commandPath[1] == "create" {
				require.NotNil(t, command.Flag("description"))
				require.NotNil(t, command.Flag("avatar-url"))
				require.NotNil(t, command.Flag("use"))
				require.NotNil(t, command.Flag("yes"))
			} else {
				require.NotNil(t, command.Flag("offset"))
				require.NotNil(t, command.Flag("limit"))
			}
		case "profile", "auth":
			require.NotNil(t, command.Flag("yes"))
		case "file":
			if commandPath[1] == "list" {
				require.NotNil(t, command.Flag("kb"))
				require.NotNil(t, command.Flag("all"))
			} else {
				require.NotNil(t, command.Flag("wait"))
				require.NotNil(t, command.Flag("upload-timeout"))
			}
		case "ask":
			require.NotNil(t, command.Flag("session-id"))
			require.NotNil(t, command.Flag("new"))
			require.NotNil(t, command.Flag("ask-timeout"))
		}
	}
}

func TestAskInputAndSessionIDValidation(t *testing.T) {
	require.Equal(t, "hello world", mustQuestion(t, []string{"hello", "world"}, ""))
	require.Equal(t, "from stdin", mustQuestion(t, []string{"-"}, "from stdin"))
	_, err := parseSessionID("not-a-number")
	require.Equal(t, clierr.ExitUsage, clierr.ExitCode(err))
	_, err = nonEmptyQuestion("  ")
	require.Equal(t, clierr.ExitUsage, clierr.ExitCode(err))
}

func mustQuestion(t *testing.T, args []string, input string) string {
	t.Helper()
	state := &app{in: strings.NewReader(input)}
	question, err := state.readQuestion(args, "")
	require.NoError(t, err)
	return question
}

func TestProfileShowDoesNotRequireCredential(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	settings := store.NewState()
	settings.CurrentContext = "work"
	settings.Contexts["work"] = store.Context{ServerURL: "https://corekg.example.com", Credential: "missing"}
	require.NoError(t, store.SaveState(paths, settings))

	var output bytes.Buffer
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths, Out: &output, ErrOut: &output})
	root.SetArgs([]string{"profile", "show", "--output", "json"})
	require.NoError(t, root.Execute())
	require.Contains(t, output.String(), `"name": "work"`)
}

func TestProfileDeleteRequiresConfirmationExitCode(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	var output bytes.Buffer
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths, Out: &output, ErrOut: &output})
	root.SetArgs([]string{"profile", "delete", "work"})
	err := root.Execute()
	require.Equal(t, clierr.ExitConfirm, clierr.ExitCode(err))
}

func TestInvalidArgumentsUseUsageExitCode(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths})
	root.SetArgs([]string{"profile", "use"})
	err := root.Execute()
	require.Equal(t, clierr.ExitUsage, clierr.ExitCode(err))
}

func TestAuthLogoutRemovesAllProfilesForCredential(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	settings := store.NewState()
	settings.CurrentContext = "work"
	settings.Contexts["work"] = store.Context{ServerURL: "https://corekg.example.com", Credential: "shared"}
	settings.Contexts["alias"] = store.Context{ServerURL: "https://corekg.example.com", Credential: "shared"}
	settings.Contexts["other"] = store.Context{ServerURL: "https://other.example.com", Credential: "other"}
	require.NoError(t, store.SaveState(paths, settings))
	auth := store.NewAuth()
	auth.Credentials["shared"] = store.Credential{ServerURL: "https://corekg.example.com", APIKey: "secret"}
	auth.Credentials["other"] = store.Credential{ServerURL: "https://other.example.com", APIKey: "other-secret"}
	require.NoError(t, store.SaveAuth(paths, auth))

	var output bytes.Buffer
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths, Out: &output, ErrOut: &output})
	root.SetArgs([]string{"auth", "logout", "--yes", "--output", "json"})
	require.NoError(t, root.Execute())
	require.Contains(t, output.String(), `"logged_out"`)

	loadedSettings, err := store.LoadState(paths)
	require.NoError(t, err)
	require.NotContains(t, loadedSettings.Contexts, "work")
	require.NotContains(t, loadedSettings.Contexts, "alias")
	require.Contains(t, loadedSettings.Contexts, "other")
	loadedAuth, err := store.LoadAuth(paths)
	require.NoError(t, err)
	require.NotContains(t, loadedAuth.Credentials, "shared")
}
