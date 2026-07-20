package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/abdul-hamid-achik/bob/internal/fsutil"
)

var eventFilenamePattern = regexp.MustCompile(`^[0-9]{6}\.json$`)

// Query limits an aggregate to a time range and, optionally, one pseudonymous
// workspace. A zero Since includes every retained event.
type Query struct {
	Since       time.Time `json:"since,omitempty"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
}

// Stats is a privacy-preserving summary. It never returns individual events.
type Stats struct {
	SchemaVersion  int              `json:"schema_version"`
	Since          time.Time        `json:"since,omitempty,omitzero"`
	Until          time.Time        `json:"until"`
	WorkspaceID    string           `json:"workspace_id,omitempty"`
	Events         int              `json:"events"`
	Successes      int              `json:"successes"`
	Failures       int              `json:"failures"`
	ConflictEvents int              `json:"conflict_events"`
	DriftEvents    int              `json:"drift_events"`
	DurationMS     int64            `json:"duration_ms"`
	Actions        ActionCounts     `json:"actions"`
	Skipped        int              `json:"skipped"`
	ByOperation    []OperationStats `json:"by_operation"`
}

// OperationStats is the same bounded aggregate grouped by a closed operation.
type OperationStats struct {
	Operation      Operation    `json:"operation"`
	Events         int          `json:"events"`
	Successes      int          `json:"successes"`
	Failures       int          `json:"failures"`
	ConflictEvents int          `json:"conflict_events"`
	DriftEvents    int          `json:"drift_events"`
	DurationMS     int64        `json:"duration_ms"`
	Actions        ActionCounts `json:"actions"`
}

// PruneReport describes data removed by the configured calendar-day
// retention window.
type PruneReport struct {
	Days   int `json:"days"`
	Events int `json:"events"`
}

// Aggregate summarizes retained events since query.Since. Malformed files are
// counted in Skipped; a newer schema is refused so it cannot be misread.
func (store *Store) Aggregate(ctx context.Context, query Query) (Stats, error) {
	now := storeNow(store)
	stats := Stats{
		SchemaVersion: SchemaVersion,
		Since:         query.Since.UTC(),
		Until:         now,
		WorkspaceID:   query.WorkspaceID,
		ByOperation:   []OperationStats{},
	}
	if store == nil || !store.enabled {
		return stats, nil
	}
	if err := ctx.Err(); err != nil {
		return Stats{}, err
	}
	if query.WorkspaceID != "" && !workspacePattern.MatchString(query.WorkspaceID) {
		return Stats{}, errors.New("invalid telemetry workspace_id query")
	}
	root := filepath.Join(store.root, versionDir)
	entries, err := readSafeDirectory(root)
	if errors.Is(err, os.ErrNotExist) {
		return stats, nil
	}
	if err != nil {
		return Stats{}, fmt.Errorf("read telemetry store: %w", err)
	}
	byOperation := make(map[Operation]*OperationStats)
	for _, day := range entries {
		if err := ctx.Err(); err != nil {
			return Stats{}, err
		}
		dayTime, valid := parseDay(day.Name())
		if !valid {
			continue
		}
		if fsutil.DirEntryIsSymlinkOrNotDir(day) {
			stats.Skipped++
			continue
		}
		if !query.Since.IsZero() && dayTime.Before(dayStart(query.Since)) {
			continue
		}
		files, err := os.ReadDir(filepath.Join(root, day.Name()))
		if err != nil {
			stats.Skipped++
			continue
		}
		for _, file := range files {
			if err := ctx.Err(); err != nil {
				return Stats{}, err
			}
			if !eventFilenamePattern.MatchString(file.Name()) {
				continue
			}
			event, err := readEvent(filepath.Join(root, day.Name(), file.Name()))
			if errors.Is(err, ErrNewerSchema) {
				return Stats{}, err
			}
			if err != nil {
				stats.Skipped++
				continue
			}
			if (!query.Since.IsZero() && event.RecordedAt.Before(query.Since)) || event.RecordedAt.After(now) {
				continue
			}
			if query.WorkspaceID != "" && event.WorkspaceID != query.WorkspaceID {
				continue
			}
			addEventToStats(&stats, byOperation, event)
		}
	}
	operations := make([]string, 0, len(byOperation))
	for operation := range byOperation {
		operations = append(operations, string(operation))
	}
	sort.Strings(operations)
	for _, operation := range operations {
		stats.ByOperation = append(stats.ByOperation, *byOperation[Operation(operation)])
	}
	return stats, nil
}

// Prune removes complete UTC day directories outside the configured
// retention window. It refuses symlinked telemetry directories.
func (store *Store) Prune(ctx context.Context) (PruneReport, error) {
	if store == nil || !store.enabled {
		return PruneReport{}, nil
	}
	if err := ctx.Err(); err != nil {
		return PruneReport{}, err
	}
	root := filepath.Join(store.root, versionDir)
	entries, err := readSafeDirectory(root)
	if errors.Is(err, os.ErrNotExist) {
		return PruneReport{}, nil
	}
	if err != nil {
		return PruneReport{}, err
	}
	cutoff := dayStart(store.now().UTC()).AddDate(0, 0, -store.retentionDays+1)
	report := PruneReport{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		date, valid := parseDay(entry.Name())
		if !valid {
			continue
		}
		if fsutil.DirEntryIsSymlinkOrNotDir(entry) {
			return report, fmt.Errorf("telemetry day %s must be a directory, not a symlink", entry.Name())
		}
		if !date.Before(cutoff) {
			continue
		}
		path := filepath.Join(root, entry.Name())
		files, err := os.ReadDir(path)
		if err != nil {
			return report, err
		}
		for _, file := range files {
			if eventFilenamePattern.MatchString(file.Name()) && file.Type().IsRegular() {
				report.Events++
			}
		}
		if err := os.RemoveAll(path); err != nil {
			return report, err
		}
		report.Days++
	}
	return report, nil
}

func addEventToStats(stats *Stats, byOperation map[Operation]*OperationStats, event Event) {
	stats.Events++
	stats.DurationMS += event.DurationMS
	addActions(&stats.Actions, event.Actions)
	operation := byOperation[event.Operation]
	if operation == nil {
		operation = &OperationStats{Operation: event.Operation}
		byOperation[event.Operation] = operation
	}
	operation.Events++
	operation.DurationMS += event.DurationMS
	addActions(&operation.Actions, event.Actions)
	if event.Outcome == OutcomeOK {
		stats.Successes++
		operation.Successes++
	} else {
		stats.Failures++
		operation.Failures++
	}
	if event.Outcome == OutcomeConflict {
		stats.ConflictEvents++
		operation.ConflictEvents++
	}
	if event.Outcome == OutcomeDrift {
		stats.DriftEvents++
		operation.DriftEvents++
	}
}

func addActions(total *ActionCounts, value ActionCounts) {
	total.Create += value.Create
	total.Update += value.Update
	total.Adopt += value.Adopt
	total.Unchanged += value.Unchanged
	total.Conflict += value.Conflict
}

func readEvent(path string) (Event, error) {
	data, err := readPrivateRegularFile(path, maxEventBytes)
	if err != nil {
		return Event{}, err
	}
	var header struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &header); err == nil && header.SchemaVersion > SchemaVersion {
		return Event{}, fmt.Errorf("%w: event version %d", ErrNewerSchema, header.SchemaVersion)
	}
	var event Event
	if err := decodeStrictJSON(data, &event); err != nil {
		return Event{}, err
	}
	if err := validateEvent(event, true); err != nil {
		return Event{}, err
	}
	return event, nil
}

func readSafeDirectory(path string) ([]os.DirEntry, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if fsutil.IsSymlinkOrNotDir(info) {
		return nil, errors.New("telemetry directory must be a directory, not a symlink")
	}
	return os.ReadDir(path)
}

func parseDay(value string) (time.Time, bool) {
	date, err := time.Parse(time.DateOnly, value)
	return date, err == nil && date.Format(time.DateOnly) == value
}

func dayStart(value time.Time) time.Time {
	year, month, day := value.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func storeNow(store *Store) time.Time {
	if store == nil || store.now == nil {
		return time.Now().UTC()
	}
	return store.now().UTC()
}
