package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDisabledStoreAndNoopNeverTouchDiskOrFail(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state", "bob")
	store, err := Open(Config{StateDir: stateDir, Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Record(context.Background(), Event{Surface: "anything"}); err != nil {
		t.Fatalf("disabled Record returned %v", err)
	}
	if id, err := store.WorkspaceID("a raw path that is never inspected"); err != nil || id != "" {
		t.Fatalf("disabled WorkspaceID = %q, %v", id, err)
	}
	if _, err := os.Stat(stateDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("disabled telemetry touched disk: %v", err)
	}
	if err := (Noop{}).Record(context.Background(), Event{}); err != nil {
		t.Fatal(err)
	}
	if err := BestEffort(failingRecorder{}).Record(context.Background(), Event{}); err != nil {
		t.Fatalf("best effort recorder leaked error: %v", err)
	}
	if err := BestEffort(nil).Record(context.Background(), Event{}); err != nil {
		t.Fatal(err)
	}
}

func TestRecordUsesPrivateFilesAndCannotContainRawSensitiveFields(t *testing.T) {
	now := time.Date(2026, 7, 12, 15, 30, 0, 0, time.UTC)
	store := openTestStore(t, Config{Now: func() time.Time { return now }})
	workspace := t.TempDir()
	workspaceID, err := store.WorkspaceID(workspace)
	if err != nil {
		t.Fatal(err)
	}
	event := Event{
		Surface:       SurfaceCLI,
		Operation:     OperationPlan,
		Outcome:       OutcomeConflict,
		Reason:        ReasonOwnershipConflict,
		DurationMS:    37,
		WorkspaceID:   workspaceID,
		Recipe:        RecipeGoAgentTool,
		RecipeVersion: 1,
		Actions: ActionCounts{
			Create: 1, Update: 2, Adopt: 3, Unchanged: 4, Conflict: 5,
		},
	}
	if err := store.Record(context.Background(), event); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(store.root)
	dayDir := filepath.Join(root, versionDir, now.Format(time.DateOnly))
	eventPath := filepath.Join(dayDir, "000000.json")
	assertMode(t, root, 0o700)
	assertMode(t, filepath.Join(root, metadataFilename), 0o600)
	assertMode(t, filepath.Join(root, versionDir), 0o700)
	assertMode(t, dayDir, 0o700)
	assertMode(t, eventPath, 0o600)

	data, err := os.ReadFile(eventPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(workspace)) {
		t.Fatal("raw workspace path was persisted")
	}
	for _, forbidden := range []string{`"path"`, `"argv"`, `"content"`, `"error"`, `"removed"`} {
		if bytes.Contains(data, []byte(forbidden)) {
			t.Fatalf("event contains forbidden field %s: %s", forbidden, data)
		}
	}
	var stored Event
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.SchemaVersion != SchemaVersion || stored.EventID == "" || !stored.RecordedAt.Equal(now) {
		t.Fatalf("store-owned fields missing: %#v", stored)
	}
	if stored.Actions != event.Actions {
		t.Fatalf("actions = %#v, want %#v", stored.Actions, event.Actions)
	}
}

func TestWorkspacePseudonymsAreStablePerStoreKeyAndCanonicalPath(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	state := filepath.Join(t.TempDir(), "state")
	config := Config{
		StateDir: state, Enabled: true, RetentionDays: 30, MaxEventsPerDay: 10,
		Now: func() time.Time { return now }, Random: bytes.NewReader(bytes.Repeat([]byte{0x41}, 128)),
	}
	first, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	id1, err := first.WorkspaceID(workspace)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Open(Config{
		StateDir: state, Enabled: true, RetentionDays: 30, MaxEventsPerDay: 10,
		Now: func() time.Time { return now }, Random: bytes.NewReader(bytes.Repeat([]byte{0x99}, 128)),
	})
	if err != nil {
		t.Fatal(err)
	}
	id2, err := second.WorkspaceID(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 || !workspacePattern.MatchString(id1) {
		t.Fatalf("pseudonyms are not stable and bounded: %q %q", id1, id2)
	}
	other, err := second.WorkspaceID(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if other == id1 {
		t.Fatal("different canonical workspaces received the same pseudonym")
	}

	alias := filepath.Join(t.TempDir(), "workspace-link")
	if err := os.Symlink(workspace, alias); err != nil {
		t.Fatal(err)
	}
	aliasID, err := second.WorkspaceID(alias)
	if err != nil {
		t.Fatal(err)
	}
	if aliasID != id1 {
		t.Fatalf("canonical alias id = %q, want %q", aliasID, id1)
	}
}

func TestRecordEnforcesClosedSchemaAndStrictDailyCapConcurrently(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	store := openTestStore(t, Config{Now: func() time.Time { return now }, MaxEventsPerDay: 5})
	invalid := validEvent()
	invalid.Surface = Surface("terminal-with-raw-label")
	if err := store.Record(context.Background(), invalid); err == nil {
		t.Fatal("unknown surface was accepted")
	}
	invalid = validEvent()
	invalid.Reason = Reason("raw error: /secret/path")
	if err := store.Record(context.Background(), invalid); err == nil {
		t.Fatal("free-form reason was accepted")
	}

	var successes atomic.Int64
	var capped atomic.Int64
	var unexpected atomic.Value
	var group sync.WaitGroup
	for range 25 {
		group.Add(1)
		go func() {
			defer group.Done()
			err := store.Record(context.Background(), validEvent())
			switch {
			case err == nil:
				successes.Add(1)
			case errors.Is(err, ErrDailyCap):
				capped.Add(1)
			default:
				unexpected.Store(err)
			}
		}()
	}
	group.Wait()
	if value := unexpected.Load(); value != nil {
		t.Fatalf("unexpected record error: %v", value)
	}
	if successes.Load() != 5 || capped.Load() != 20 {
		t.Fatalf("successes/capped = %d/%d, want 5/20", successes.Load(), capped.Load())
	}
	stats, err := store.Aggregate(context.Background(), Query{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Events != 5 {
		t.Fatalf("stored events = %d, want 5", stats.Events)
	}
}

func TestEventTypeHasNoRawSensitiveFieldEscapeHatch(t *testing.T) {
	typeOfEvent := reflect.TypeFor[Event]()
	for index := range typeOfEvent.NumField() {
		name := strings.ToLower(typeOfEvent.Field(index).Name)
		for _, forbidden := range []string{"path", "argv", "content", "error", "message", "label"} {
			if strings.Contains(name, forbidden) {
				t.Fatalf("Event.%s permits sensitive free-form telemetry", typeOfEvent.Field(index).Name)
			}
		}
	}
}

func TestRecordRefusesSymlinkedTelemetryVersionDirectory(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	store := openTestStore(t, Config{Now: func() time.Time { return now }})
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(store.root, versionDir)); err != nil {
		t.Fatal(err)
	}
	if err := store.Record(context.Background(), validEvent()); err == nil {
		t.Fatal("symlinked version directory was accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("telemetry escaped its root: %v", entries)
	}
}

type failingRecorder struct{}

func (failingRecorder) Record(context.Context, Event) error { return errors.New("disk full") }

func openTestStore(t *testing.T, override Config) *Store {
	t.Helper()
	if override.StateDir == "" {
		override.StateDir = filepath.Join(t.TempDir(), "state", "bob")
	}
	override.Enabled = true
	if override.RetentionDays == 0 {
		override.RetentionDays = 30
	}
	if override.MaxEventsPerDay == 0 {
		override.MaxEventsPerDay = 100
	}
	if override.Random == nil {
		override.Random = bytes.NewReader(deterministicBytes(8192))
	}
	store, err := Open(override)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func deterministicBytes(size int) []byte {
	value := make([]byte, size)
	for index := range value {
		value[index] = byte(index % 251)
	}
	return value
}

func validEvent() Event {
	return Event{Surface: SurfaceCLI, Operation: OperationPlan, Outcome: OutcomeOK}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode of %s = %04o, want %04o", path, got, want)
	}
}
