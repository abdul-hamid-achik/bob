// Package paths resolves Bob's per-user filesystem layout.
//
// Resolution is side-effect free: callers decide which directories, if any,
// should be created.
package paths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const configFilename = "config.yaml"

// Layout is Bob's XDG-based per-user filesystem layout.
type Layout struct {
	ConfigDir  string
	ConfigFile string
	DataDir    string
	StateDir   string
	CacheDir   string
}

// Resolve returns Bob's per-user filesystem layout. Bob-specific overrides
// take precedence over XDG variables. Every supplied override must be an
// absolute path.
func Resolve() (Layout, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Layout{}, fmt.Errorf("resolve home directory: %w", err)
	}
	return resolve(home, os.Getenv)
}

func resolve(home string, getenv func(string) string) (Layout, error) {
	if !filepath.IsAbs(home) {
		return Layout{}, errors.New("home directory must be absolute")
	}

	configDir, err := directory(getenv, "BOB_CONFIG_DIR", "XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err != nil {
		return Layout{}, err
	}
	dataDir, err := directory(getenv, "BOB_DATA_DIR", "XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	if err != nil {
		return Layout{}, err
	}
	stateDir, err := directory(getenv, "BOB_STATE_DIR", "XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	if err != nil {
		return Layout{}, err
	}
	cacheDir, err := directory(getenv, "BOB_CACHE_DIR", "XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	if err != nil {
		return Layout{}, err
	}

	configFile := filepath.Join(configDir, configFilename)
	if override := getenv("BOB_CONFIG"); override != "" {
		if !filepath.IsAbs(override) {
			return Layout{}, errors.New("BOB_CONFIG must be an absolute path")
		}
		configFile = filepath.Clean(override)
	}

	return Layout{
		ConfigDir:  configDir,
		ConfigFile: configFile,
		DataDir:    dataDir,
		StateDir:   stateDir,
		CacheDir:   cacheDir,
	}, nil
}

func directory(getenv func(string) string, bobName, xdgName, fallback string) (string, error) {
	if value := getenv(bobName); value != "" {
		if !filepath.IsAbs(value) {
			return "", fmt.Errorf("%s must be an absolute path", bobName)
		}
		return filepath.Clean(value), nil
	}
	if value := getenv(xdgName); value != "" {
		if !filepath.IsAbs(value) {
			return "", fmt.Errorf("%s must be an absolute path", xdgName)
		}
		return filepath.Join(filepath.Clean(value), "bob"), nil
	}
	return filepath.Join(filepath.Clean(fallback), "bob"), nil
}
