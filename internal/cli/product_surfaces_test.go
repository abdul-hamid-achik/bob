package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	bobpaths "github.com/abdul-hamid-achik/bob/internal/paths"
	"github.com/abdul-hamid-achik/bob/internal/settings"
	"github.com/abdul-hamid-achik/bob/internal/studio"
	"github.com/abdul-hamid-achik/bob/internal/telemetry"
)

func TestConfigInitPreviewsThenCreatesPrivateXDGSettings(t *testing.T) {
	isolateBobXDG(t)
	layout, err := bobpaths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("config", "init", "--telemetry")
	if err != nil || !bytes.Contains([]byte(stdout), []byte("preview; nothing written")) {
		t.Fatalf("preview: output=%q err=%v", stdout, err)
	}
	if _, err := os.Stat(layout.ConfigFile); !os.IsNotExist(err) {
		t.Fatalf("preview wrote settings: %v", err)
	}
	if _, _, err := executeForTest("config", "init", "--telemetry", "--write"); err != nil {
		t.Fatal(err)
	}
	value, err := settings.LoadFile(layout.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if !value.Telemetry.Enabled {
		t.Fatal("written settings did not enable telemetry")
	}
}

func TestCLIRecordsPlanAndStatsReturnsOnlyAggregate(t *testing.T) {
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
	target := filepath.Join(t.TempDir(), "private-workspace-name")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	deps := Dependencies{
		Out: &stdout, ErrOut: &stderr, Prober: testProber{}, IntegrationRunner: testIntegrationRunner{},
		Recorder: telemetry.BestEffort(store), Telemetry: store,
	}
	if err := execute([]string{"plan", target}, deps); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := execute([]string{"--json", "stats", target}, deps); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data struct {
			Enabled bool            `json:"enabled"`
			Stats   telemetry.Stats `json:"stats"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode stats: %v\n%s", err, stdout.String())
	}
	if !envelope.Data.Enabled || envelope.Data.Stats.Events != 1 || len(envelope.Data.Stats.ByOperation) != 1 || envelope.Data.Stats.ByOperation[0].Operation != telemetry.OperationPlan {
		t.Fatalf("unexpected stats: %#v", envelope.Data)
	}
	state, err := os.ReadFile(firstTelemetryEvent(t, layout.StateDir))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(state, []byte(target)) || bytes.Contains(state, []byte("github.com/acme/acme")) {
		t.Fatalf("telemetry leaked workspace or module: %s", state)
	}
}

type capturingStudioRunner struct {
	called   bool
	snapshot studio.Snapshot
}

func (runner *capturingStudioRunner) Run(ctx context.Context, root string, options studio.Options) error {
	runner.called = true
	snapshot, err := options.Source.Load(ctx, root)
	runner.snapshot = snapshot
	return err
}

type panicProbeRunner struct{}

func (panicProbeRunner) LookPath(string) (string, error) { return "/bin/tool", nil }
func (panicProbeRunner) Run(context.Context, string, string, ...string) inspectpkg.CommandResult {
	panic("studio launched a specialist probe")
}

func TestStudioCommandUsesReadOnlySourceAndRejectsJSON(t *testing.T) {
	target := filepath.Join(t.TempDir(), "acme")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}
	runner := &capturingStudioRunner{}
	var stdout, stderr bytes.Buffer
	command := New(Dependencies{
		Out: &stdout, ErrOut: &stderr, Prober: testProber{},
		IntegrationRunner: panicProbeRunner{}, StudioRunner: runner,
	})
	command.SetArgs([]string{"studio", target, "--single-pane"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !runner.called || runner.snapshot.Plan == nil || runner.snapshot.Report.Repository.State != "clean" {
		t.Fatalf("unexpected studio snapshot: called=%t snapshot=%#v", runner.called, runner.snapshot)
	}
	if runner.snapshot.Stats.Enabled {
		t.Fatal("studio reported disabled telemetry as enabled")
	}
	command = New(Dependencies{Out: &stdout, ErrOut: &stderr, StudioRunner: runner})
	command.SetArgs([]string{"--json", "studio", target})
	if err := command.Execute(); err == nil {
		t.Fatal("studio unexpectedly accepted --json")
	}
}

func isolateBobXDG(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, name := range []string{"BOB_CONFIG", "BOB_CONFIG_DIR", "BOB_DATA_DIR", "BOB_STATE_DIR", "BOB_CACHE_DIR", "BOB_TELEMETRY"} {
		t.Setenv(name, "")
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
}

func firstTelemetryEvent(t *testing.T, stateDir string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(stateDir, "telemetry", "v1", "*", "*.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("telemetry events = %v, err=%v", matches, err)
	}
	return matches[0]
}
