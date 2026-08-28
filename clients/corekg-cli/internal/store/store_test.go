package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigStateAndAuthUseRestrictedPermissions(t *testing.T) {
	paths := NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	config := DefaultConfig()
	require.NoError(t, SaveConfig(paths, config))

	state := NewState()
	state.Profiles["work"] = Profile{ServerURL: "https://corekg.example.com", Credential: "work", OrganizationID: "org-1", KnowledgeBaseID: "kb-1"}
	state.CurrentProfile = "work"
	state.CurrentContext = "work"
	require.NoError(t, SaveState(paths, state))

	auth := NewAuth()
	auth.Credentials["work"] = Credential{ServerURL: "https://corekg.example.com", APIKey: "yg-test"}
	require.NoError(t, SaveAuth(paths, auth))

	loadedConfig, err := LoadConfig(paths)
	require.NoError(t, err)
	require.Equal(t, config, loadedConfig)
	loadedState, err := LoadState(paths)
	require.NoError(t, err)
	require.Equal(t, state, loadedState)
	loadedAuth, err := LoadAuth(paths)
	require.NoError(t, err)
	require.Equal(t, auth, loadedAuth)

	rootInfo, err := os.Stat(paths.RootDir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0700), rootInfo.Mode().Perm())
	for _, filename := range []string{paths.ConfigFile, paths.StateFile, paths.AuthFile} {
		fileInfo, statErr := os.Stat(filename)
		require.NoError(t, statErr)
		require.Equal(t, os.FileMode(0600), fileInfo.Mode().Perm())
	}
}

func TestSetCurrentProfileRequiresExistingProfile(t *testing.T) {
	state := NewState()
	require.Error(t, SetCurrentProfile(&state, "missing"))

	state.Profiles["default"] = Profile{}
	require.NoError(t, SetCurrentProfile(&state, "default"))
	require.Equal(t, "default", state.CurrentProfile)
}

func TestStateInitializesPerKnowledgeBaseChatSessions(t *testing.T) {
	state := NewState()
	state.Profiles["work"] = Profile{ServerURL: "https://corekg.example.com"}
	state = state.Normalize()
	require.NotNil(t, state.Profiles["work"].ChatSessions)
	state.Profiles["work"].ChatSessions["20001"] = 30001

	data, err := json.Marshal(state)
	require.NoError(t, err)
	var loaded State
	require.NoError(t, json.Unmarshal(data, &loaded))
	require.Equal(t, uint(30001), loaded.Profiles["work"].ChatSessions["20001"])
}

func TestLoadMissingFilesReturnsEmptyStores(t *testing.T) {
	paths := NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	config, err := LoadConfig(paths)
	require.NoError(t, err)
	require.Equal(t, NewConfig(), config)
	state, err := LoadState(paths)
	require.NoError(t, err)
	require.Equal(t, NewState(), state)
	auth, err := LoadAuth(paths)
	require.NoError(t, err)
	require.Equal(t, NewAuth(), auth)
	_, err = os.Stat(paths.RootDir)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestLoadRejectsFutureVersion(t *testing.T) {
	paths := NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	require.NoError(t, os.MkdirAll(paths.RootDir, 0700))
	require.NoError(t, os.WriteFile(paths.StateFile, []byte(`{"version":99,"profiles":{}}`), 0600))
	_, err := LoadState(paths)
	require.Error(t, err)
	require.Contains(t, err.Error(), "newer than supported")
}

func TestLoadLegacyStaticConfigAndProfilesWithoutDeletingSource(t *testing.T) {
	paths := NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	require.NoError(t, os.MkdirAll(paths.RootDir, 0700))
	require.NoError(t, os.WriteFile(paths.LegacyConfigFile, []byte(`{"version":2,"default_server":"https://corekg.example.com","default_output":"json","current_context":"work","contexts":{"work":{"server":"https://corekg.example.com","credential":"credential-1"}}}`), 0600))

	config, err := LoadConfig(paths)
	require.NoError(t, err)
	require.Equal(t, "https://corekg.example.com", config.Server)
	require.Equal(t, "json", config.Output)
	state, err := LoadState(paths)
	require.NoError(t, err)
	require.Equal(t, "work", state.CurrentProfile)
	require.Contains(t, state.Profiles, "work")

	require.NoError(t, SaveConfig(paths, config))
	require.NoError(t, SaveState(paths, state))
	configData, err := os.ReadFile(paths.ConfigFile)
	require.NoError(t, err)
	require.Contains(t, string(configData), `"server": "https://corekg.example.com"`)
	stateData, err := os.ReadFile(paths.StateFile)
	require.NoError(t, err)
	require.Contains(t, string(stateData), `"current_profile": "work"`)
	legacyData, err := os.ReadFile(paths.LegacyConfigFile)
	require.NoError(t, err)
	require.Contains(t, string(legacyData), `"current_context"`)
}

func TestParseConfigJSONIsStrict(t *testing.T) {
	for _, value := range []string{
		`{"version":1,"unknown":true}`,
		`{"version":1} {}`,
		`[]`,
		`null`,
		`{"version":1,"server":"https://user:pass@example.com"}`,
		`{"version":1,"frontend":"https://corekg.example.com?tenant=one"}`,
	} {
		_, err := ParseConfigJSON([]byte(value))
		require.Error(t, err, value)
	}
	config, err := ParseConfigJSON([]byte(`{"version":1,"server":"https://api.corekg.example.com","frontend":"https://corekg.example.com","output":"id","timeout":"1s"}`))
	require.NoError(t, err)
	require.Equal(t, "https://corekg.example.com", config.Frontend)
	require.Equal(t, "id", config.Output)
}

func TestLoadConfigSourceReadsFileWithoutModifyingIt(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "custom-config.json")
	original := []byte("{\n  \"server\": \"https://example.com\",\n  \"output\": \"json\"\n}\n")
	require.NoError(t, os.WriteFile(filename, original, 0600))
	config, err := LoadConfigSource(filename)
	require.NoError(t, err)
	require.Equal(t, "https://example.com", config.Server)
	data, err := os.ReadFile(filename)
	require.NoError(t, err)
	require.Equal(t, original, data)
}

func TestValidateServerURL(t *testing.T) {
	for _, value := range []string{"ftp://example.com", "https://", "https://example.com?x=1", "https://example.com#fragment"} {
		require.Error(t, ValidateServerURL(value), value)
	}
	require.NoError(t, ValidateServerURL("https://example.com/path"))
}

func TestWithLockReleasesLockAfterCallback(t *testing.T) {
	paths := NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	called := false
	require.NoError(t, WithLock(paths, func() error {
		called = true
		return nil
	}))
	require.True(t, called)
	_, err := os.Stat(paths.LockFile)
	require.ErrorIs(t, err, os.ErrNotExist)
}
