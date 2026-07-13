package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAggregateFiltersSinceAndWorkspaceAndSumsPlanActions(t *testing.T) {
	current := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	store := openTestStore(t, Config{Now: func() time.Time { return current }})
	workspaceA, err := store.WorkspaceID(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	workspaceB, err := store.WorkspaceID(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	first := validEvent()
	first.WorkspaceID = workspaceA
	first.Actions = ActionCounts{Create: 1, Update: 2, Adopt: 3, Unchanged: 4, Conflict: 5}
	first.DurationMS = 10
	if err := store.Record(context.Background(), first); err != nil {
		t.Fatal(err)
	}

	current = current.Add(time.Hour)
	second := validEvent()
	second.WorkspaceID = workspaceB
	second.Outcome = OutcomeConflict
	second.Reason = ReasonOwnershipConflict
	second.Actions = ActionCounts{Create: 10, Conflict: 1}
	second.DurationMS = 20
	if err := store.Record(context.Background(), second); err != nil {
		t.Fatal(err)
	}

	current = current.Add(time.Hour)
	third := validEvent()
	third.WorkspaceID = workspaceA
	third.Operation = OperationApply
	third.Outcome = OutcomeDrift
	third.Actions = ActionCounts{Update: 6, Adopt: 7, Unchanged: 8}
	third.DurationMS = 30
	if err := store.Record(context.Background(), third); err != nil {
		t.Fatal(err)
	}
	current = current.Add(time.Minute)

	stats, err := store.Aggregate(context.Background(), Query{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Events != 3 || stats.Successes != 1 || stats.Failures != 2 || stats.ConflictEvents != 1 || stats.DriftEvents != 1 || stats.DurationMS != 60 {
		t.Fatalf("unexpected aggregate: %#v", stats)
	}
	wantActions := ActionCounts{Create: 11, Update: 8, Adopt: 10, Unchanged: 12, Conflict: 6}
	if stats.Actions != wantActions {
		t.Fatalf("actions = %#v, want %#v", stats.Actions, wantActions)
	}
	if len(stats.ByOperation) != 2 || stats.ByOperation[0].Operation != OperationApply || stats.ByOperation[1].Operation != OperationPlan {
		t.Fatalf("operation aggregates not deterministic: %#v", stats.ByOperation)
	}

	since := time.Date(2026, 7, 10, 10, 30, 0, 0, time.UTC)
	filtered, err := store.Aggregate(context.Background(), Query{Since: since, WorkspaceID: workspaceA})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Events != 1 || filtered.Actions != third.Actions || len(filtered.ByOperation) != 1 || filtered.ByOperation[0].Operation != OperationApply {
		t.Fatalf("filtered aggregate = %#v", filtered)
	}
	if _, err := store.Aggregate(context.Background(), Query{WorkspaceID: "raw/workspace/path"}); err == nil {
		t.Fatal("raw workspace query was accepted")
	}
}

func TestAggregateSkipsCorruptionButRefusesNewerSchema(t *testing.T) {
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	store := openTestStore(t, Config{Now: func() time.Time { return now }})
	if err := store.Record(context.Background(), validEvent()); err != nil {
		t.Fatal(err)
	}
	day := filepath.Join(store.root, versionDir, now.Format(time.DateOnly))
	if err := os.WriteFile(filepath.Join(day, "000001.json"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	stats, err := store.Aggregate(context.Background(), Query{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Events != 1 || stats.Skipped != 1 {
		t.Fatalf("corruption handling = %#v", stats)
	}

	future := map[string]any{"schema_version": SchemaVersion + 1}
	data, err := json.Marshal(future)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(day, "000002.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Aggregate(context.Background(), Query{}); !errors.Is(err, ErrNewerSchema) {
		t.Fatalf("future schema error = %v", err)
	}
}

func TestRetentionPrunesWholeUTCDaysAndKeepsConfiguredWindow(t *testing.T) {
	current := time.Date(2026, 7, 10, 23, 0, 0, 0, time.UTC)
	store := openTestStore(t, Config{Now: func() time.Time { return current }, RetentionDays: 2})
	if err := store.Record(context.Background(), validEvent()); err != nil {
		t.Fatal(err)
	}
	oldDay := filepath.Join(store.root, versionDir, current.Format(time.DateOnly))
	current = current.AddDate(0, 0, 1)
	if err := store.Record(context.Background(), validEvent()); err != nil {
		t.Fatal(err)
	}
	current = current.AddDate(0, 0, 1)
	if err := store.Record(context.Background(), validEvent()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldDay); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired day still exists: %v", err)
	}
	stats, err := store.Aggregate(context.Background(), Query{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Events != 2 {
		t.Fatalf("retained events = %d, want 2", stats.Events)
	}
}

func TestOpenRefusesFutureMetadataSchema(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	root := filepath.Join(state, "telemetry")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{"schema_version": SchemaVersion + 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, metadataFilename), data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Open(Config{StateDir: state, Enabled: true, RetentionDays: 30, MaxEventsPerDay: 100})
	if !errors.Is(err, ErrNewerSchema) {
		t.Fatalf("future metadata error = %v", err)
	}
}

func TestOpenValidatesEnabledStoreBounds(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	for name, config := range map[string]Config{
		"relative state": {StateDir: "relative", Enabled: true, RetentionDays: 30, MaxEventsPerDay: 1},
		"retention":      {StateDir: state, Enabled: true, RetentionDays: 0, MaxEventsPerDay: 1},
		"daily cap":      {StateDir: state, Enabled: true, RetentionDays: 30, MaxEventsPerDay: 0},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Open(config); err == nil {
				t.Fatal("invalid config was accepted")
			}
		})
	}
}
