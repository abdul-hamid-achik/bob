package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/telemetry"
)

func TestLoadFileMissingUsesPrivacyPreservingDefaults(t *testing.T) {
	t.Setenv("BOB_TELEMETRY", "")
	got, err := LoadFile(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got != Default() {
		t.Fatalf("settings = %#v, want %#v", got, Default())
	}
	if got.Telemetry.Enabled {
		t.Fatal("telemetry must be disabled by default")
	}
}

func TestLoadFileIsStrictAndPreservesDefaultsForOmittedFields(t *testing.T) {
	t.Setenv("BOB_TELEMETRY", "")
	path := writeSettings(t, `schema_version: 1
telemetry:
  enabled: true
  retention_days: 7
`)
	got, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Telemetry.Enabled || got.Telemetry.RetentionDays != 7 || got.Telemetry.MaxEventsPerDay != DefaultMaxEventsPerDay {
		t.Fatalf("unexpected settings: %#v", got)
	}

	path = writeSettings(t, "schema_version: 1\ntelemetry:\n  endpoint: https://example.test\n")
	if _, err := LoadFile(path); err == nil || !strings.Contains(err.Error(), "field endpoint not found") {
		t.Fatalf("unknown field error = %v", err)
	}
}

func TestLoadFileRejectsMissingOrUnsupportedSchemaAndMultipleDocuments(t *testing.T) {
	t.Setenv("BOB_TELEMETRY", "")
	for name, body := range map[string]string{
		"missing":  "telemetry:\n  enabled: false\n",
		"future":   "schema_version: 2\n",
		"multiple": "schema_version: 1\n---\nschema_version: 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadFile(writeSettings(t, body)); err == nil {
				t.Fatal("expected strict schema error")
			}
		})
	}
}

func TestLoadFileValidatesTelemetryBounds(t *testing.T) {
	t.Setenv("BOB_TELEMETRY", "")
	for name, body := range map[string]string{
		"retention low":  "schema_version: 1\ntelemetry:\n  retention_days: 0\n",
		"retention high": "schema_version: 1\ntelemetry:\n  retention_days: 366\n",
		"daily low":      "schema_version: 1\ntelemetry:\n  max_events_per_day: 0\n",
		"daily high":     "schema_version: 1\ntelemetry:\n  max_events_per_day: 10001\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadFile(writeSettings(t, body)); err == nil {
				t.Fatal("expected bounds error")
			}
		})
	}
}

func TestLoadFileEnvironmentOverridesOnlyEnabled(t *testing.T) {
	path := writeSettings(t, "schema_version: 1\ntelemetry:\n  enabled: false\n  retention_days: 9\n")
	t.Setenv("BOB_TELEMETRY", "true")
	got, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Telemetry.Enabled || got.Telemetry.RetentionDays != 9 {
		t.Fatalf("unexpected environment override: %#v", got)
	}
	t.Setenv("BOB_TELEMETRY", "perhaps")
	if _, err := LoadFile(path); err == nil {
		t.Fatal("invalid boolean override was accepted")
	}
}

func TestLoadFileRejectsSymlinksAndOversizedFiles(t *testing.T) {
	t.Setenv("BOB_TELEMETRY", "")
	target := writeSettings(t, "schema_version: 1\n")
	link := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(link); err == nil {
		t.Fatal("symlink was accepted")
	}

	oversized := filepath.Join(t.TempDir(), "large.yaml")
	if err := os.WriteFile(oversized, []byte(strings.Repeat("x", maxConfigBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(oversized); err == nil {
		t.Fatal("oversized file was accepted")
	}
}

// TestTelemetryBoundsAgreeWithSettingsAtBoundary guards against
// internal/settings and internal/telemetry drifting apart on their shared
// retention/daily-cap bounds: settings validation must accept exactly what
// telemetry.Open accepts, and reject exactly what it rejects, at the max
// boundary and one past it.
func TestTelemetryBoundsAgreeWithSettingsAtBoundary(t *testing.T) {
	t.Setenv("BOB_TELEMETRY", "")
	if maxRetentionDays != telemetry.MaxRetentionDays || maxEventsPerDay != telemetry.MaxEventsPerDay {
		t.Fatalf("settings bounds drifted from telemetry: retention %d != %d, events %d != %d",
			maxRetentionDays, telemetry.MaxRetentionDays, maxEventsPerDay, telemetry.MaxEventsPerDay)
	}

	atMax := fmt.Sprintf("schema_version: 1\ntelemetry:\n  enabled: true\n  retention_days: %d\n  max_events_per_day: %d\n",
		telemetry.MaxRetentionDays, telemetry.MaxEventsPerDay)
	settingsAtMax, err := LoadFile(writeSettings(t, atMax))
	if err != nil {
		t.Fatalf("settings rejected the maximum bound: %v", err)
	}
	store, err := telemetry.Open(telemetry.Config{
		StateDir:        t.TempDir(),
		Enabled:         settingsAtMax.Telemetry.Enabled,
		RetentionDays:   settingsAtMax.Telemetry.RetentionDays,
		MaxEventsPerDay: settingsAtMax.Telemetry.MaxEventsPerDay,
	})
	if err != nil {
		t.Fatalf("telemetry.Open rejected the maximum bound settings allowed: %v", err)
	}
	if !store.Enabled() {
		t.Fatal("expected an enabled store at the maximum bound")
	}

	overRetention := fmt.Sprintf("schema_version: 1\ntelemetry:\n  enabled: true\n  retention_days: %d\n", telemetry.MaxRetentionDays+1)
	if _, err := LoadFile(writeSettings(t, overRetention)); err == nil {
		t.Fatal("settings accepted retention_days above the maximum")
	}
	if _, err := telemetry.Open(telemetry.Config{
		StateDir:        t.TempDir(),
		Enabled:         true,
		RetentionDays:   telemetry.MaxRetentionDays + 1,
		MaxEventsPerDay: telemetry.MaxEventsPerDay,
	}); err == nil {
		t.Fatal("telemetry.Open accepted retention_days above the maximum")
	}

	overEvents := fmt.Sprintf("schema_version: 1\ntelemetry:\n  enabled: true\n  max_events_per_day: %d\n", telemetry.MaxEventsPerDay+1)
	if _, err := LoadFile(writeSettings(t, overEvents)); err == nil {
		t.Fatal("settings accepted max_events_per_day above the maximum")
	}
	if _, err := telemetry.Open(telemetry.Config{
		StateDir:        t.TempDir(),
		Enabled:         true,
		RetentionDays:   telemetry.MaxRetentionDays,
		MaxEventsPerDay: telemetry.MaxEventsPerDay + 1,
	}); err == nil {
		t.Fatal("telemetry.Open accepted max_events_per_day above the maximum")
	}
}

func writeSettings(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
