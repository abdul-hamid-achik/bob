package studio

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
)

type fakeSource struct {
	load func(context.Context, string) (Snapshot, error)
}

func (f fakeSource) Load(ctx context.Context, workspace string) (Snapshot, error) {
	return f.load(ctx, workspace)
}

func fixtureSnapshot() Snapshot {
	plan := engine.PlanResult{
		SchemaVersion: engine.PlanSchemaVersion,
		Recipe:        engine.LockRecipe{ID: "go-agent-tool", Version: 3}, // matches the fixture's go-agent-tool recipe version
		ConflictCount: 1,
		LockChanged:   true,
		Actions: []engine.Action{
			{Path: "README.md", Kind: engine.ActionConflict, CurrentSHA256: strings.Repeat("a", 64), DesiredSHA256: strings.Repeat("b", 64), Reason: "unmanaged file differs from desired content", CurrentMode: 0o644, DesiredMode: 0o644},
			{Path: "cmd/acme/main.go", Kind: engine.ActionCreate, DesiredSHA256: strings.Repeat("c", 64), DesiredPreview: "package main\n\nfunc main() {}\n", DesiredMode: 0o644, Reason: "destination does not exist"},
			{Path: "go.mod", Kind: engine.ActionUnchanged, DesiredSHA256: strings.Repeat("d", 64), CurrentSHA256: strings.Repeat("d", 64), LockedSHA256: strings.Repeat("d", 64), CurrentMode: 0o644, DesiredMode: 0o644, Reason: "managed file already matches"},
		},
	}
	fresh := false
	return Snapshot{
		CapturedAt: time.Date(2026, 7, 12, 14, 32, 8, 0, time.UTC),
		Plan:       &plan,
		Stats: Stats{
			Enabled: true, WindowDays: 30, Total: 3, Success: 2, Conflicts: 1,
			PerOperation: map[string]int{"apply": 1, "plan": 2},
		},
		Report: inspectpkg.Report{
			SchemaVersion: inspectpkg.SchemaVersion,
			Workspace:     "/tmp/界面/acme",
			Repository: inspectpkg.Repository{
				State: "conflicted", Recipe: "go-agent-tool", Ready: false, Converged: false,
				LockChanged: true, ManagedFiles: 3, ConflictCount: 1,
				Actions: inspectpkg.ActionCounts{Create: 1, Conflict: 1, Unchanged: 1},
			},
			Integrations: []inspectpkg.Integration{
				{Name: "codemap", Selected: true, Available: true, Probe: inspectpkg.Probe{State: inspectpkg.ProbeNotRequested}, Index: inspectpkg.Index{State: inspectpkg.IndexUnknown, Fresh: &fresh}},
				{Name: "vecgrep", Selected: true, Available: false, Probe: inspectpkg.Probe{State: inspectpkg.ProbeUnavailable}, Index: inspectpkg.Index{State: inspectpkg.IndexUnknown}},
			},
			Degraded:    true,
			Warnings:    []string{"codemap index status was not probed"},
			NextActions: []inspectpkg.CommandAction{{Reason: "review conflicts", CWD: "/tmp/界面/acme", Argv: []string{"bob", "plan", "."}}},
		},
	}
}

func loadedModel(t *testing.T) Model {
	t.Helper()
	snapshot := fixtureSnapshot()
	source := fakeSource{load: func(context.Context, string) (Snapshot, error) { return snapshot, nil }}
	m := NewModel(snapshot.Report.Workspace, source, false)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init returned nil command")
	}
	updated, _ := m.Update(cmd())
	return updated.(Model)
}

func key(value string) tea.KeyPressMsg {
	switch value {
	case "tab":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyTab})
	case "shift+tab":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift})
	case "up":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyUp})
	case "down":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyDown})
	case "pgup":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp})
	case "pgdown":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown})
	case "esc":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
	default:
		runes := []rune(value)
		return tea.KeyPressMsg(tea.Key{Text: value, Code: runes[0]})
	}
}

func updateModel(m Model, msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(Model), cmd
}

func TestViewsAndAttentionFilter(t *testing.T) {
	m := loadedModel(t)
	if got := len(m.visibleActions()); got != 2 {
		t.Fatalf("attention actions = %d, want 2", got)
	}
	m, _ = updateModel(m, key("2"))
	if m.active != viewPlan {
		t.Fatalf("active = %d, want plan", m.active)
	}
	m, _ = updateModel(m, key("a"))
	if m.attention || len(m.visibleActions()) != 3 {
		t.Fatalf("all action filter not enabled: attention=%t actions=%d", m.attention, len(m.visibleActions()))
	}
	m, _ = updateModel(m, key("3"))
	if m.active != viewStats {
		t.Fatalf("active = %d, want stats", m.active)
	}
	m, _ = updateModel(m, key("tab"))
	if m.active != viewOverview {
		t.Fatalf("tab did not wrap to overview: %d", m.active)
	}
}

func TestPlanNavigationAndSelectionPreservedAcrossRefresh(t *testing.T) {
	m := loadedModel(t)
	m.active = viewPlan
	m, _ = updateModel(m, key("down"))
	selected := m.selectedPath()
	if selected != "cmd/acme/main.go" {
		t.Fatalf("selected = %q", selected)
	}
	snapshot := fixtureSnapshot()
	snapshot.Plan.Actions[0], snapshot.Plan.Actions[1] = snapshot.Plan.Actions[1], snapshot.Plan.Actions[0]
	m.request = 2
	m.loading = true
	m, _ = updateModel(m, snapshotLoadedMsg{request: 2, snapshot: snapshot})
	if got := m.selectedPath(); got != selected {
		t.Fatalf("selection after refresh = %q, want %q", got, selected)
	}
}

func TestRefreshQueuesAndIgnoresStaleResponses(t *testing.T) {
	m := loadedModel(t)
	m.loading = true
	m.request = 7
	m, cmd := updateModel(m, key("r"))
	if cmd != nil || !m.queued {
		t.Fatalf("busy refresh should queue: cmd=%v queued=%t", cmd, m.queued)
	}
	before := m.snapshot
	m, _ = updateModel(m, snapshotLoadedMsg{request: 6, err: errors.New("stale")})
	if m.snapshot != before || m.refreshErr != "" {
		t.Fatal("stale response changed model")
	}
	m, cmd = updateModel(m, snapshotLoadedMsg{request: 7, snapshot: fixtureSnapshot()})
	if cmd == nil || !m.loading || m.request != 8 || m.queued {
		t.Fatalf("queued refresh not started: cmd=%v loading=%t request=%d queued=%t", cmd, m.loading, m.request, m.queued)
	}
}

func TestRefreshErrorKeepsLastGoodSnapshot(t *testing.T) {
	m := loadedModel(t)
	before := m.snapshot
	m.request = 2
	m.loading = true
	m, _ = updateModel(m, snapshotLoadedMsg{request: 2, err: errors.New("disk changed")})
	if m.snapshot != before {
		t.Fatal("refresh error discarded the last good snapshot")
	}
	if !strings.Contains(m.render(), "showing snapshot") {
		t.Fatalf("render did not disclose stale snapshot:\n%s", m.render())
	}
}

func TestHelpAndQuitKeys(t *testing.T) {
	m := loadedModel(t)
	m, _ = updateModel(m, key("?"))
	if !m.help || !strings.Contains(m.render(), "never applies a plan") {
		t.Fatal("help did not open with authority boundary")
	}
	m, _ = updateModel(m, key("esc"))
	if m.help {
		t.Fatal("escape did not close help")
	}
	m, cmd := updateModel(m, key("q"))
	if !m.quitting || cmd == nil {
		t.Fatal("quit did not return tea.Quit")
	}
}

func TestRenderFitsTerminalAtSupportedSizes(t *testing.T) {
	for _, size := range [][2]int{{120, 40}, {96, 24}, {80, 20}, {50, 12}, {40, 10}, {20, 6}} {
		for _, view := range []viewID{viewOverview, viewPlan, viewStats} {
			t.Run(fmt.Sprintf("%dx%d-view%d", size[0], size[1], view), func(t *testing.T) {
				m := loadedModel(t)
				m.width, m.height, m.active = size[0], size[1], view
				assertGrid(t, m.render(), size[0], size[1])
			})
		}
	}
}

func TestWindowSizeBeforeInitialSnapshotDoesNotPanic(t *testing.T) {
	m := NewModel("/tmp/acme", fakeSource{}, false)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	got := updated.(Model)
	if got.width != 90 || got.height != 24 || got.snapshot != nil {
		t.Fatalf("unexpected pre-snapshot resize state: %#v", got)
	}
}

func TestSinglePaneStillShowsSelectedPlanDetail(t *testing.T) {
	m := loadedModel(t)
	m.singlePane = true
	m.active = viewPlan
	m.width, m.height = 100, 24
	out := m.render()
	if !strings.Contains(out, "Selected action") || !strings.Contains(out, "README.md") {
		t.Fatalf("single-pane plan lost action detail:\n%s", out)
	}
}

func TestStatsViewUsesSourceSuppliedAggregate(t *testing.T) {
	m := loadedModel(t)
	m.active = viewStats
	out := m.render()
	for _, want := range []string{"enabled · local only", "Rolling 30-day aggregate", "operations  3", "success     2", "conflicts   1", "apply", "plan"} {
		if !strings.Contains(out, want) {
			t.Fatalf("Stats view missing %q:\n%s", want, out)
		}
	}
}

func TestStatsViewExplainsDisabledTelemetry(t *testing.T) {
	m := loadedModel(t)
	m.active = viewStats
	m.snapshot.Stats = Stats{PerOperation: map[string]int{}}
	out := m.render()
	for _, want := range []string{"telemetry   disabled", "No local usage events are being recorded"} {
		if !strings.Contains(out, want) {
			t.Fatalf("disabled Stats view missing %q:\n%s", want, out)
		}
	}
}

func TestUntrustedTextCannotInjectTerminalControls(t *testing.T) {
	m := loadedModel(t)
	m.snapshot.Report.Warnings = []string{"warning\x1b[2J\nsecond line"}
	out := m.render()
	if strings.Contains(out, "\x1b[2J") || !strings.Contains(out, "warning second line") {
		t.Fatalf("untrusted control sequence not sanitized: %q", out)
	}
}

func TestMissingPlanRendersOrdinaryEmptyState(t *testing.T) {
	snapshot := Snapshot{
		Report:     inspectpkg.Report{Workspace: "/tmp/new", Repository: inspectpkg.Repository{State: "missing_manifest", Error: "bob.yaml is missing"}},
		CapturedAt: time.Now(), Stats: Stats{PerOperation: map[string]int{}},
	}
	m := NewModel(snapshot.Report.Workspace, fakeSource{load: func(context.Context, string) (Snapshot, error) { return snapshot, nil }}, false)
	updated, _ := m.Update(snapshotLoadedMsg{request: 1, snapshot: snapshot})
	m = updated.(Model)
	m.active = viewPlan
	if out := m.render(); !strings.Contains(out, "plan unavailable") || !strings.Contains(out, "bob.yaml is missing") {
		t.Fatalf("missing manifest state not useful:\n%s", out)
	}
}

func assertGrid(t *testing.T, output string, width, height int) {
	t.Helper()
	lines := strings.Split(output, "\n")
	if len(lines) != height {
		t.Fatalf("height = %d, want %d", len(lines), height)
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != width {
			t.Fatalf("line %d width = %d, want %d: %q", i, got, width, line)
		}
	}
}

func TestModeString(t *testing.T) {
	if got := modeString(fs.FileMode(0)); got != "none" {
		t.Fatalf("zero mode = %q", got)
	}
	if got := modeString(fs.FileMode(0o644)); !strings.Contains(got, "rw-r--r--") {
		t.Fatalf("0644 mode = %q", got)
	}
}
