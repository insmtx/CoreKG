package store

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	DirectoryName  = ".corekg"
	ConfigName     = "config.json"
	StateName      = "state.json"
	LegacyName     = "setting.json"
	AuthName       = "auth.json"
	currentVersion = 1
)

type Paths struct {
	RootDir          string
	ConfigFile       string
	StateFile        string
	LegacyConfigFile string
	AuthFile         string
	LockFile         string
}

func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user home directory: %w", err)
	}
	return NewPaths(filepath.Join(home, DirectoryName)), nil
}

func NewPaths(rootDir string) Paths {
	rootDir = filepath.Clean(rootDir)
	return Paths{
		RootDir:          rootDir,
		ConfigFile:       filepath.Join(rootDir, ConfigName),
		StateFile:        filepath.Join(rootDir, StateName),
		LegacyConfigFile: filepath.Join(rootDir, LegacyName),
		AuthFile:         filepath.Join(rootDir, AuthName),
		LockFile:         filepath.Join(rootDir, ".lock"),
	}
}

func CurrentVersion() int {
	return currentVersion
}
