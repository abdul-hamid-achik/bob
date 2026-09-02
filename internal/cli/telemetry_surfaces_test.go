package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	bobpaths "github.com/abdul-hamid-achik/bob/internal/paths"
	"github.com/abdul-hamid-achik/bob/internal/settings"
	"github.com/abdul-hamid-achik/bob/internal/telemetry"
)

// TestRecordedOperationExcludesReadOnlyProductSurfaces guards the closed
// operation vocabulary directly: stats, studio, and config must never become
// telemetry-recordable, independent of whatever a future command might name
// its positional argument. AGENTS.md's invariant is explicit that studio,
// stats, and configuration commands never record events even when telemetry
// is enabled.
func TestRecordedOperationExcludesReadOnlyProductSurfaces(t *testing.T) {
	t.Parallel()
	cases := [][]string{
		{"stats"},
		{"stats", "--all"},
		{"studio"},
		{"studio", "--single-pane"},
		{"config"},
		{"config", "show"},
		{"config", "init", "--telemetry", "--write"},
	}
	for _, args := range cases {
		if operation, ok := recordedOperation(args); ok {
			t.Fatalf("recordedOperation(%v) = %q, true; want not-ok", args, operation)
		}
	}
}

// TestStatsStudioConfigAppendZeroTelemetryEventsWhenEnabled proves the
// integration end of the same invariant: with telemetry enabled end to end
// (settings on disk, an open store, and a live recorder wired into
// Dependencies exactly as Execute does), running stats, studio, and every
// config subcommand appends nothing to the telemetry store. This fails the
// moment recordedOperation starts recognizing "stats", "studio", or "config".
func TestStatsStudioConfigAppendZeroTelemetryEventsWhenEnabled(t *testing.T) {
	isolateBobXDG(t)
	layout, err := bobpaths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	value := settings.Default()
	value.Telemetry.Enabled = true
	if err := settings.WriteFile(layout.ConfigFile, value); err != nil {
		t.Fatal(err)
	}
	store, err := telemetry.Open(telemetry.Config{
		StateDir: layout.StateDir, Enabled: true,
		RetentionDays: value.Telemetry.RetentionDays, MaxEventsPerDay: value.Telemetry.MaxEventsPerDay,
	})
	if err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(t.TempDir(), "acme")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}

	studioRunner := &capturingStudioRunner{}
	var stdout, stderr bytes.Buffer
	deps := Dependencies{
		Out: &stdout, ErrOut: &stderr, Prober: testProber{}, IntegrationRunner: testIntegrationRunner{},
		Recorder: telemetry.BestEffort(store), Telemetry: store, StudioRunner: studioRunner,
	}

	runs := []struct {
		name string
		args []string
	}{
		{"stats", []string{"--json", "stats", target}},
		{"stats all", []string{"stats", "--all"}},
		{"config show", []string{"--json", "config", "show"}},
		{"config init preview", []string{"config", "init", "--telemetry"}},
		{"studio", []string{"studio", target, "--single-pane"}},
	}
	for _, run := range runs {
		stdout.Reset()
		stderr.Reset()
		if err := execute(run.args, deps); err != nil {
			t.Fatalf("%s: %v\nstdout=%s\nstderr=%s", run.name, err, stdout.String(), stderr.String())
		}
	}
	if !studioRunner.called {
		t.Fatal("studio runner was never invoked; test did not exercise the studio surface")
	}

	matches, err := filepath.Glob(filepath.Join(layout.StateDir, "telemetry", "v1", "*", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("stats/studio/config appended telemetry events: %v", matches)
	}
}
