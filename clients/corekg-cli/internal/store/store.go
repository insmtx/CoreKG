package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	currentConfigVersion = 1
	currentStateVersion  = 2
	currentAuthVersion   = 2
	legacyVersion        = 2
	defaultServer        = "http://127.0.0.1:8080"
	defaultFrontend      = "http://localhost:3001"
	defaultOutput        = "table"
	defaultTimeout       = "30s"
)

type Profile struct {
	ServerURL         string          `json:"server"`
	Credential        string          `json:"credential"`
	OrganizationID    string          `json:"organization_id,omitempty"`
	OrganizationName  string          `json:"organization_name,omitempty"`
	KnowledgeBaseID   string          `json:"knowledge_base_id,omitempty"`
	KnowledgeBaseName string          `json:"knowledge_base_name,omitempty"`
	ChatSessions      map[string]uint `json:"chat_sessions,omitempty"`
}

type Context = Profile

type Config struct {
	Version  int    `json:"version"`
	Server   string `json:"server,omitempty"`
	Frontend string `json:"frontend,omitempty"`
	Output   string `json:"output,omitempty"`
	Timeout  string `json:"timeout,omitempty"`
}

type State struct {
	Version         int                `json:"version"`
	CurrentProfile  string             `json:"current_profile,omitempty"`
	PreviousProfile string             `json:"previous_profile,omitempty"`
	Profiles        map[string]Profile `json:"profiles"`

	CurrentContext string             `json:"-"`
	Contexts       map[string]Profile `json:"-"`
}

type Credential struct {
	ServerURL        string    `json:"server"`
	APIKey           string    `json:"api_key"`
	Source           string    `json:"source,omitempty"`
	APIKeyID         uint      `json:"api_key_id,omitempty"`
	APIKeyPurpose    string    `json:"api_key_purpose,omitempty"`
	UIN              uint      `json:"uin,omitempty"`
	OrganizationID   string    `json:"organization_id,omitempty"`
	OrganizationName string    `json:"organization_name,omitempty"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}

type Auth struct {
	Version     int                     `json:"version"`
	Credentials map[string]Credential   `json:"credentials"`
	Pending     map[string]PendingLogin `json:"pending_logins,omitempty"`
}

type PendingLogin struct {
	ServerURL string    `json:"server"`
	Name      string    `json:"name,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
}

type legacySettings struct {
	Version         int                `json:"version"`
	CurrentProfile  string             `json:"current_profile,omitempty"`
	PreviousProfile string             `json:"previous_profile,omitempty"`
	Server          string             `json:"server,omitempty"`
	Output          string             `json:"output,omitempty"`
	Timeout         string             `json:"timeout,omitempty"`
	DefaultServer   string             `json:"default_server,omitempty"`
	DefaultOutput   string             `json:"default_output,omitempty"`
	Profiles        map[string]Profile `json:"profiles"`
	CurrentContext  string             `json:"current_context,omitempty"`
	Contexts        map[string]Profile `json:"contexts"`
}

func NewConfig() Config {
	return Config{Version: currentConfigVersion}
}

func DefaultConfig() Config {
	return Config{Version: currentConfigVersion, Server: defaultServer, Frontend: defaultFrontend, Output: defaultOutput, Timeout: defaultTimeout}
}

func NewState() State {
	profiles := map[string]Profile{}
	return State{Version: currentStateVersion, Profiles: profiles, Contexts: profiles}
}

func NewAuth() Auth {
	return Auth{Version: currentAuthVersion, Credentials: map[string]Credential{}, Pending: map[string]PendingLogin{}}
}

func (s State) Normalize() State {
	if s.Version < currentStateVersion {
		s.Version = currentStateVersion
	}
	if s.Profiles == nil {
		s.Profiles = s.Contexts
	}
	if s.Profiles == nil {
		s.Profiles = map[string]Profile{}
	}
	if s.CurrentProfile == "" {
		s.CurrentProfile = s.CurrentContext
	}
	for name, profile := range s.Profiles {
		if profile.ChatSessions == nil {
			profile.ChatSessions = map[string]uint{}
			s.Profiles[name] = profile
		}
	}
	s.Contexts = s.Profiles
	s.CurrentContext = s.CurrentProfile
	return s
}

func (s *State) UnmarshalJSON(data []byte) error {
	type wire struct {
		Version         int                `json:"version"`
		CurrentProfile  string             `json:"current_profile,omitempty"`
		PreviousProfile string             `json:"previous_profile,omitempty"`
		Profiles        map[string]Profile `json:"profiles"`
		CurrentContext  string             `json:"current_context,omitempty"`
		Contexts        map[string]Profile `json:"contexts"`
	}
	var value wire
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	profiles := value.Profiles
	if profiles == nil {
		profiles = value.Contexts
	}
	current := value.CurrentProfile
	if current == "" {
		current = value.CurrentContext
	}
	*s = State{Version: value.Version, CurrentProfile: current, PreviousProfile: value.PreviousProfile, Profiles: profiles}
	*s = s.Normalize()
	return nil
}

func (s State) MarshalJSON() ([]byte, error) {
	s = s.Normalize()
	type wire struct {
		Version         int                `json:"version"`
		CurrentProfile  string             `json:"current_profile,omitempty"`
		PreviousProfile string             `json:"previous_profile,omitempty"`
		Profiles        map[string]Profile `json:"profiles"`
	}
	return json.Marshal(wire{Version: s.Version, CurrentProfile: s.CurrentProfile, PreviousProfile: s.PreviousProfile, Profiles: s.Profiles})
}

func (c Config) Normalize() Config {
	if c.Version < currentConfigVersion {
		c.Version = currentConfigVersion
	}
	c.Server = strings.TrimRight(strings.TrimSpace(c.Server), "/")
	c.Frontend = strings.TrimRight(strings.TrimSpace(c.Frontend), "/")
	c.Output = strings.TrimSpace(c.Output)
	c.Timeout = strings.TrimSpace(c.Timeout)
	return c
}

func (c Config) TimeoutDuration() (time.Duration, error) {
	value := strings.TrimSpace(c.Timeout)
	if value == "" {
		value = defaultTimeout
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		if err == nil {
			err = fmt.Errorf("duration must be positive")
		}
		return 0, fmt.Errorf("invalid timeout %q: %w", value, err)
	}
	return duration, nil
}

func (c Config) Validate() error {
	c = c.Normalize()
	if c.Version > currentConfigVersion {
		return fmt.Errorf("configuration version %d is newer than supported version %d", c.Version, currentConfigVersion)
	}
	if c.Server != "" {
		if err := ValidateServerURL(c.Server); err != nil {
			return err
		}
	}
	if c.Frontend != "" {
		if err := ValidateServerURL(c.Frontend); err != nil {
			return fmt.Errorf("invalid frontend URL: %w", err)
		}
	}
	if c.Output != "" {
		if c.Output != "table" && c.Output != "json" && c.Output != "id" {
			return fmt.Errorf("unsupported output format %q; use table, json, or id", c.Output)
		}
	}
	if _, err := c.TimeoutDuration(); err != nil {
		return err
	}
	return nil
}

func ValidateServerURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("invalid server URL %q: %w", value, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("server URL must use http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("server URL must include a host")
	}
	if parsed.User != nil {
		return fmt.Errorf("server URL must not include user information")
	}
	if parsed.RawQuery != "" {
		return fmt.Errorf("server URL must not include a query")
	}
	if parsed.Fragment != "" {
		return fmt.Errorf("server URL must not include a fragment")
	}
	return nil
}

func MergeConfig(base, override Config) Config {
	base = base.Normalize()
	override = override.Normalize()
	if override.Version != 0 {
		base.Version = override.Version
	}
	if override.Server != "" {
		base.Server = override.Server
	}
	if override.Frontend != "" {
		base.Frontend = override.Frontend
	}
	if override.Output != "" {
		base.Output = override.Output
	}
	if override.Timeout != "" {
		base.Timeout = override.Timeout
	}
	return base.Normalize()
}

func ParseConfigJSON(data []byte) (Config, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return Config{}, fmt.Errorf("decode config JSON: %w", err)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return Config{}, fmt.Errorf("config JSON must be an object")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, fmt.Errorf("config JSON must contain exactly one object")
		}
		return Config{}, fmt.Errorf("decode trailing config JSON: %w", err)
	}
	configDecoder := json.NewDecoder(bytes.NewReader(raw))
	configDecoder.DisallowUnknownFields()
	var config Config
	if err := configDecoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode config JSON: %w", err)
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config.Normalize(), nil
}

func LoadConfigSource(source string) (Config, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return Config{}, fmt.Errorf("config source must not be empty")
	}
	if strings.HasPrefix(source, "{") {
		return ParseConfigJSON([]byte(source))
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return Config{}, fmt.Errorf("read config file %q: %w", source, err)
	}
	return ParseConfigJSON(data)
}

func validateVersion(version, supported int) error {
	if version > supported {
		return fmt.Errorf("configuration version %d is newer than supported version %d", version, supported)
	}
	return nil
}

func (a Auth) Normalize() Auth {
	if a.Version < currentAuthVersion {
		a.Version = currentAuthVersion
	}
	if a.Credentials == nil {
		a.Credentials = map[string]Credential{}
	}
	if a.Pending == nil {
		a.Pending = map[string]PendingLogin{}
	}
	return a
}

func loadLegacy(paths Paths) (legacySettings, error) {
	var value legacySettings
	if err := readJSON(paths, paths.LegacyConfigFile, &value); err != nil {
		return legacySettings{}, err
	}
	return value, nil
}

func LoadConfig(paths Paths) (Config, error) {
	config, err := loadConfigFile(paths, paths.ConfigFile)
	if err == nil {
		if err := validateVersion(config.Version, currentConfigVersion); err != nil {
			return Config{}, fmt.Errorf("load config: %w", err)
		}
		config = config.Normalize()
		if err := config.Validate(); err != nil {
			return Config{}, fmt.Errorf("load config: %w", err)
		}
		return config, nil
	} else if !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	legacy, err := loadLegacy(paths)
	if err != nil {
		if os.IsNotExist(err) {
			return NewConfig(), nil
		}
		return Config{}, fmt.Errorf("load legacy config: %w", err)
	}
	config = Config{Version: legacy.Version, Server: legacy.Server, Output: legacy.Output, Timeout: legacy.Timeout}
	if config.Server == "" {
		config.Server = legacy.DefaultServer
	}
	if config.Output == "" {
		config.Output = legacy.DefaultOutput
	}
	if err := validateVersion(config.Version, legacyVersion); err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	config.Version = currentConfigVersion
	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	return config.Normalize(), nil
}

func SaveConfig(paths Paths, config Config) error {
	config = config.Normalize()
	if err := config.Validate(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	if err := writeJSON(paths, paths.ConfigFile, config); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

func LoadState(paths Paths) (State, error) {
	var state State
	if err := readJSON(paths, paths.StateFile, &state); err == nil {
		if err := validateVersion(state.Version, currentStateVersion); err != nil {
			return State{}, fmt.Errorf("load state: %w", err)
		}
		return state.Normalize(), nil
	} else if !os.IsNotExist(err) {
		return State{}, fmt.Errorf("load state: %w", err)
	}
	legacy, err := loadLegacy(paths)
	if err != nil {
		if os.IsNotExist(err) {
			return NewState(), nil
		}
		return State{}, fmt.Errorf("load legacy state: %w", err)
	}
	state = State{Version: legacy.Version, CurrentProfile: legacy.CurrentProfile, PreviousProfile: legacy.PreviousProfile, Profiles: legacy.Profiles, CurrentContext: legacy.CurrentContext, Contexts: legacy.Contexts}
	if err := validateVersion(state.Version, legacyVersion); err != nil {
		return State{}, fmt.Errorf("load state: %w", err)
	}
	state.Version = currentStateVersion
	return state.Normalize(), nil
}

func SaveState(paths Paths, state State) error {
	state = state.Normalize()
	if err := writeJSON(paths, paths.StateFile, state); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	return nil
}

func LoadAuth(paths Paths) (Auth, error) {
	var auth Auth
	if err := readJSON(paths, paths.AuthFile, &auth); err != nil {
		if os.IsNotExist(err) {
			return NewAuth(), nil
		}
		return Auth{}, fmt.Errorf("load auth: %w", err)
	}
	if err := validateVersion(auth.Version, currentAuthVersion); err != nil {
		return Auth{}, fmt.Errorf("load auth: %w", err)
	}
	return auth.Normalize(), nil
}

func SaveAuth(paths Paths, auth Auth) error {
	auth = auth.Normalize()
	if err := writeJSON(paths, paths.AuthFile, auth); err != nil {
		return fmt.Errorf("save auth: %w", err)
	}
	return nil
}

func WithLock(paths Paths, fn func() error) error {
	if fn == nil {
		return fmt.Errorf("lock callback must not be nil")
	}
	if err := ensureRoot(paths); err != nil {
		return err
	}
	lockFile := paths.LockFile
	if lockFile == "" {
		lockFile = filepath.Join(paths.RootDir, ".lock")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		file, err := os.OpenFile(lockFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			_ = file.Close()
			defer os.Remove(lockFile)
			return fn()
		}
		if !os.IsExist(err) {
			return fmt.Errorf("create configuration lock: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("configuration lock is held by another process: %s", lockFile)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func ProfileNames(state State) []string {
	state = state.Normalize()
	names := make([]string, 0, len(state.Profiles))
	for name := range state.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func SetCurrentProfile(state *State, name string) error {
	if state == nil {
		return fmt.Errorf("state must not be nil")
	}
	*state = state.Normalize()
	if _, ok := state.Profiles[name]; !ok {
		return fmt.Errorf("profile %q does not exist", name)
	}
	if state.CurrentProfile != name {
		state.PreviousProfile = state.CurrentProfile
	}
	state.CurrentProfile = name
	state.CurrentContext = name
	return nil
}

func RenameProfile(state *State, oldName, newName string) error {
	if state == nil {
		return fmt.Errorf("state must not be nil")
	}
	*state = state.Normalize()
	if oldName == newName {
		return fmt.Errorf("profile names are unchanged")
	}
	profile, ok := state.Profiles[oldName]
	if !ok {
		return fmt.Errorf("profile %q does not exist", oldName)
	}
	if _, exists := state.Profiles[newName]; exists {
		return fmt.Errorf("profile %q already exists", newName)
	}
	state.Profiles[newName] = profile
	delete(state.Profiles, oldName)
	if state.CurrentProfile == oldName {
		state.CurrentProfile = newName
	}
	if state.PreviousProfile == oldName {
		state.PreviousProfile = newName
	}
	*state = state.Normalize()
	return nil
}

func DeleteProfile(state *State, name string) (Profile, error) {
	if state == nil {
		return Profile{}, fmt.Errorf("state must not be nil")
	}
	*state = state.Normalize()
	profile, ok := state.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("profile %q does not exist", name)
	}
	delete(state.Profiles, name)
	if state.CurrentProfile == name {
		state.CurrentProfile = ""
		for _, candidate := range ProfileNames(*state) {
			state.CurrentProfile = candidate
			break
		}
	}
	if state.PreviousProfile == name {
		state.PreviousProfile = ""
	}
	*state = state.Normalize()
	return profile, nil
}

func ContextNames(state State) []string                 { return ProfileNames(state) }
func SetCurrentContext(state *State, name string) error { return SetCurrentProfile(state, name) }
func RenameContext(state *State, oldName, newName string) error {
	return RenameProfile(state, oldName, newName)
}
func DeleteContext(state *State, name string) (Context, error) { return DeleteProfile(state, name) }

func ensureRoot(paths Paths) error {
	if paths.RootDir == "" {
		return fmt.Errorf("configuration directory is empty")
	}
	info, err := os.Lstat(paths.RootDir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(paths.RootDir, 0700); err != nil {
			return fmt.Errorf("create configuration directory: %w", err)
		}
		info, err = os.Lstat(paths.RootDir)
	}
	if err != nil {
		return fmt.Errorf("inspect configuration directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("configuration directory must not be a symbolic link: %s", paths.RootDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("configuration path is not a directory: %s", paths.RootDir)
	}
	if info.Mode().Perm()&0077 != 0 {
		if err := os.Chmod(paths.RootDir, 0700); err != nil {
			return fmt.Errorf("restrict configuration directory permissions: %w", err)
		}
	}
	return nil
}

func readJSON(paths Paths, filename string, destination any) error {
	data, err := readFile(paths, filename)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode JSON: multiple values")
		}
		return fmt.Errorf("decode trailing JSON: %w", err)
	}
	return nil
}

func loadConfigFile(paths Paths, filename string) (Config, error) {
	data, err := readFile(paths, filename)
	if err != nil {
		return Config{}, err
	}
	return ParseConfigJSON(data)
}

func readFile(paths Paths, filename string) ([]byte, error) {
	if err := inspectRoot(paths); err != nil {
		return nil, err
	}
	info, err := os.Lstat(filename)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("configuration file must not be a symbolic link: %s", filename)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("configuration path is not a regular file: %s", filename)
	}
	if info.Mode().Perm()&0077 != 0 {
		if err := os.Chmod(filename, 0600); err != nil {
			return nil, fmt.Errorf("restrict configuration file permissions: %w", err)
		}
	}
	return os.ReadFile(filename)
}

func inspectRoot(paths Paths) error {
	if paths.RootDir == "" {
		return fmt.Errorf("configuration directory is empty")
	}
	info, err := os.Lstat(paths.RootDir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("configuration directory must not be a symbolic link: %s", paths.RootDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("configuration path is not a directory: %s", paths.RootDir)
	}
	return nil
}

func writeJSON(paths Paths, filename string, value any) error {
	if err := ensureRoot(paths); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(paths.RootDir, ".corekg-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary configuration file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("restrict temporary configuration file permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary configuration file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary configuration file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary configuration file: %w", err)
	}
	if err := os.Rename(temporaryName, filename); err != nil {
		return fmt.Errorf("replace configuration file: %w", err)
	}
	return syncDirectory(paths.RootDir)
}

func syncDirectory(dirname string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(dirname)
	if err != nil {
		return fmt.Errorf("open configuration directory for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync configuration directory: %w", err)
	}
	return nil
}
