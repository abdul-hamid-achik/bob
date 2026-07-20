// Package settings loads Bob's per-user configuration.
package settings

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/abdul-hamid-achik/bob/internal/fsutil"
	"github.com/abdul-hamid-achik/bob/internal/paths"
	"go.yaml.in/yaml/v3"
)

const (
	SchemaVersion          = 1
	DefaultRetentionDays   = 30
	DefaultMaxEventsPerDay = 1000
	maxRetentionDays       = 365
	maxEventsPerDay        = 10_000
	maxConfigBytes         = 1 << 20
)

// Settings contains Bob's machine-local user preferences. Repository intent
// belongs in bob.yaml instead.
type Settings struct {
	SchemaVersion int       `json:"schema_version" yaml:"schema_version"`
	Telemetry     Telemetry `json:"telemetry" yaml:"telemetry"`
}

// Telemetry configures Bob's local-only, privacy-bounded telemetry store.
type Telemetry struct {
	Enabled         bool `json:"enabled" yaml:"enabled"`
	RetentionDays   int  `json:"retention_days" yaml:"retention_days"`
	MaxEventsPerDay int  `json:"max_events_per_day" yaml:"max_events_per_day"`
}

type fileSettings struct {
	SchemaVersion *int           `yaml:"schema_version"`
	Telemetry     *fileTelemetry `yaml:"telemetry"`
}

type fileTelemetry struct {
	Enabled         *bool `yaml:"enabled"`
	RetentionDays   *int  `yaml:"retention_days"`
	MaxEventsPerDay *int  `yaml:"max_events_per_day"`
}

// Default returns settings with telemetry disabled.
func Default() Settings {
	return Settings{
		SchemaVersion: SchemaVersion,
		Telemetry: Telemetry{
			Enabled:         false,
			RetentionDays:   DefaultRetentionDays,
			MaxEventsPerDay: DefaultMaxEventsPerDay,
		},
	}
}

// Load resolves and loads Bob's per-user config file. A missing config file is
// equivalent to Default. BOB_TELEMETRY may override telemetry.enabled.
func Load() (Settings, error) {
	layout, err := paths.Resolve()
	if err != nil {
		return Settings{}, err
	}
	return loadFile(layout.ConfigFile, os.Getenv)
}

// LoadFile loads settings from path. A missing file is equivalent to Default.
// BOB_TELEMETRY may override telemetry.enabled.
func LoadFile(path string) (Settings, error) {
	return loadFile(path, os.Getenv)
}

func loadFile(path string, getenv func(string) string) (Settings, error) {
	settings := Default()
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return applyEnvironment(settings, getenv)
	}
	if err != nil {
		return Settings{}, fmt.Errorf("inspect settings: %w", err)
	}
	if fsutil.IsSymlinkOrNotRegular(info) {
		return Settings{}, errors.New("settings file must be a regular file, not a symlink")
	}
	if info.Size() > maxConfigBytes {
		return Settings{}, fmt.Errorf("settings file exceeds %d bytes", maxConfigBytes)
	}

	file, err := os.Open(path)
	if err != nil {
		return Settings{}, fmt.Errorf("open settings: %w", err)
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	if err != nil {
		return Settings{}, fmt.Errorf("read settings: %w", err)
	}
	if len(data) > maxConfigBytes {
		return Settings{}, fmt.Errorf("settings file exceeds %d bytes", maxConfigBytes)
	}

	var document fileSettings
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil {
		return Settings{}, fmt.Errorf("decode settings: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Settings{}, errors.New("settings must contain exactly one YAML document")
		}
		return Settings{}, fmt.Errorf("decode settings: %w", err)
	}
	if document.SchemaVersion == nil {
		return Settings{}, errors.New("settings schema_version is required")
	}
	if *document.SchemaVersion != SchemaVersion {
		return Settings{}, fmt.Errorf("unsupported settings schema_version %d", *document.SchemaVersion)
	}

	if telemetry := document.Telemetry; telemetry != nil {
		if telemetry.Enabled != nil {
			settings.Telemetry.Enabled = *telemetry.Enabled
		}
		if telemetry.RetentionDays != nil {
			settings.Telemetry.RetentionDays = *telemetry.RetentionDays
		}
		if telemetry.MaxEventsPerDay != nil {
			settings.Telemetry.MaxEventsPerDay = *telemetry.MaxEventsPerDay
		}
	}
	if err := validate(settings); err != nil {
		return Settings{}, err
	}
	return applyEnvironment(settings, getenv)
}

func applyEnvironment(settings Settings, getenv func(string) string) (Settings, error) {
	if value := getenv("BOB_TELEMETRY"); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return Settings{}, fmt.Errorf("BOB_TELEMETRY must be a boolean: %w", err)
		}
		settings.Telemetry.Enabled = enabled
	}
	return settings, nil
}

func validate(settings Settings) error {
	if settings.Telemetry.RetentionDays < 1 || settings.Telemetry.RetentionDays > maxRetentionDays {
		return fmt.Errorf("telemetry.retention_days must be between 1 and %d", maxRetentionDays)
	}
	if settings.Telemetry.MaxEventsPerDay < 1 || settings.Telemetry.MaxEventsPerDay > maxEventsPerDay {
		return fmt.Errorf("telemetry.max_events_per_day must be between 1 and %d", maxEventsPerDay)
	}
	return nil
}
