// Package engine plans and applies deterministic, ownership-aware repository
// changes. It never runs commands; its only mutation surface is the artifact
// files declared by a recipe and the bob.lock ownership registry.
package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf8"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

const (
	// PlanSchemaVersion independently versions the structured plan projection.
	PlanSchemaVersion = 1
	// RecipeVersion is the current built-in recipe contract. Locks
	// from older positive versions are safe upgrade inputs because they record
	// exact content hashes for every previously managed whole file.
	RecipeVersion = 3
)

// ActionKind is the planner's decision for one desired artifact.
type ActionKind string

const (
	ActionCreate    ActionKind = "create"
	ActionUpdate    ActionKind = "update"
	ActionUnchanged ActionKind = "unchanged"
	ActionAdopt     ActionKind = "adopt"
	ActionConflict  ActionKind = "conflict"
)

// ErrPlanConflicts reports an apply that was refused before any mutation.
var ErrPlanConflicts = errors.New("plan contains conflicts")

// Action describes the complete, deterministic decision for one artifact.
// Hashes are lowercase hexadecimal SHA-256 values. CurrentSHA256 is empty when
// the destination does not exist.
type Action struct {
	Path           string      `json:"path" yaml:"path"`
	Kind           ActionKind  `json:"kind" yaml:"kind"`
	CurrentSHA256  string      `json:"current_sha256,omitempty" yaml:"current_sha256,omitempty"`
	DesiredSHA256  string      `json:"desired_sha256" yaml:"desired_sha256"`
	DesiredPreview string      `json:"desired_preview,omitempty" yaml:"desired_preview,omitempty"`
	LockedSHA256   string      `json:"locked_sha256,omitempty" yaml:"locked_sha256,omitempty"`
	CurrentMode    fs.FileMode `json:"current_mode,omitempty" yaml:"current_mode,omitempty"`
	DesiredMode    fs.FileMode `json:"desired_mode" yaml:"desired_mode"`
	Reason         string      `json:"reason,omitempty" yaml:"reason,omitempty"`
	expectedExists bool
}

// PlanResult is a read-only projection of desired and observed repository
// state. Actions are sorted by path.
type PlanResult struct {
	SchemaVersion int        `json:"schema_version" yaml:"schema_version"`
	Recipe        LockRecipe `json:"recipe" yaml:"recipe"`
	Actions       []Action   `json:"actions" yaml:"actions"`
	ConflictCount int        `json:"conflict_count" yaml:"conflict_count"`
	LockChanged   bool       `json:"lock_changed" yaml:"lock_changed"`
	DesiredLock   LockFile   `json:"desired_lock" yaml:"desired_lock"`
	lockExists    bool
	lockSHA256    string
}

// HasConflicts reports whether Apply must refuse the plan.
func (p PlanResult) HasConflicts() bool { return p.ConflictCount > 0 }

// ApplyResult reports exactly what Apply changed. Written and Adopted are
// sorted because they follow the deterministic plan order.
type ApplyResult struct {
	Plan        PlanResult `json:"plan" yaml:"plan"`
	Written     []string   `json:"written,omitempty" yaml:"written,omitempty"`
	Adopted     []string   `json:"adopted,omitempty" yaml:"adopted,omitempty"`
	Unchanged   []string   `json:"unchanged,omitempty" yaml:"unchanged,omitempty"`
	LockWritten bool       `json:"lock_written" yaml:"lock_written"`
}

type desiredArtifact struct {
	recipe.Artifact
	path string
	hash string
	mode fs.FileMode
}

// Plan compares a validated manifest and rendered artifact set with the
// repository. It does not mutate the filesystem.
func Plan(root string, m manifest.Manifest, artifacts []recipe.Artifact) (PlanResult, error) {
	var result PlanResult
	if err := m.Validate(); err != nil {
		return result, fmt.Errorf("plan: validate manifest: %w", err)
	}
	canonicalRoot, err := validateRoot(root)
	if err != nil {
		return result, fmt.Errorf("plan: %w", err)
	}
	desired, err := normalizeArtifacts(canonicalRoot, artifacts)
	if err != nil {
		return result, fmt.Errorf("plan: %w", err)
	}

	lock, lockExists, lockBytes, err := loadLock(canonicalRoot)
	if err != nil {
		return result, fmt.Errorf("plan: %w", err)
	}
	if lockExists && lock.Recipe.ID != m.Recipe {
		return result, fmt.Errorf("plan: lock recipe %s@%d does not match %s", lock.Recipe.ID, lock.Recipe.Version, m.Recipe)
	}
	if lockExists && lock.Recipe.Version > RecipeVersion {
		return result, fmt.Errorf("plan: lock recipe %s@%d is newer than supported %s@%d", lock.Recipe.ID, lock.Recipe.Version, m.Recipe, RecipeVersion)
	}

	locked := make(map[string]LockEntry, len(lock.Files))
	for _, entry := range lock.Files {
		locked[entry.Path] = entry
	}
	desiredPaths := make(map[string]struct{}, len(desired))
	for _, artifact := range desired {
		desiredPaths[artifact.path] = struct{}{}
	}
	result = PlanResult{
		SchemaVersion: PlanSchemaVersion,
		Recipe:        LockRecipe{ID: m.Recipe, Version: RecipeVersion},
		Actions:       make([]Action, 0, len(desired)),
		DesiredLock:   desiredLock(m, desired),
		lockExists:    lockExists,
	}
	if lockExists {
		result.lockSHA256 = hashBytes(lockBytes)
	}
	desiredLockBytes, err := encodeLock(result.DesiredLock)
	if err != nil {
		return PlanResult{}, fmt.Errorf("plan: encode desired lock: %w", err)
	}
	result.LockChanged = !lockExists || string(lockBytes) != string(desiredLockBytes)

	for _, entry := range lock.Files {
		if _, retained := desiredPaths[entry.Path]; retained {
			continue
		}
		observation, err := inspectDestination(canonicalRoot, entry.Path)
		if err != nil {
			return PlanResult{}, fmt.Errorf("plan: inspect retired path %q: %w", entry.Path, err)
		}
		if !observation.exists {
			continue
		}
		result.Actions = append(result.Actions, Action{
			Path:           entry.Path,
			Kind:           ActionConflict,
			CurrentSHA256:  observation.hash,
			LockedSHA256:   entry.SHA256,
			CurrentMode:    observation.mode,
			expectedExists: true,
			Reason:         "bob.lock owns this file but the recipe no longer declares it; remove it manually before applying the lock update",
		})
		result.ConflictCount++
	}

	for _, artifact := range desired {
		observation, err := inspectDestination(canonicalRoot, artifact.path)
		if err != nil {
			return PlanResult{}, fmt.Errorf("plan: inspect %q: %w", artifact.path, err)
		}
		action := Action{
			Path:           artifact.path,
			DesiredSHA256:  artifact.hash,
			DesiredPreview: contentPreview(artifact.Content),
			DesiredMode:    artifact.mode,
			CurrentSHA256:  observation.hash,
			CurrentMode:    observation.mode,
			expectedExists: observation.exists,
		}
		entry, managed := locked[artifact.path]
		if managed {
			action.LockedSHA256 = entry.SHA256
		}
		switch {
		case !observation.exists && managed:
			action.Kind = ActionConflict
			action.Reason = "managed file is missing; its recorded content cannot be proven unchanged"
		case !observation.exists:
			action.Kind = ActionCreate
			action.Reason = "destination does not exist"
		case managed && observation.hash == artifact.hash && observation.mode == artifact.mode:
			action.Kind = ActionUnchanged
			action.Reason = "managed file already matches the desired content and mode"
		case managed && observation.hash == artifact.hash:
			action.Kind = ActionUpdate
			action.Reason = "managed content matches but its file mode drifted"
		case managed && observation.hash != entry.SHA256:
			action.Kind = ActionConflict
			action.Reason = "managed file differs from the hash recorded in bob.lock"
		case managed:
			action.Kind = ActionUpdate
			action.Reason = "managed file still matches bob.lock and may be updated safely"
		case observation.hash == artifact.hash && observation.mode != artifact.mode:
			action.Kind = ActionConflict
			action.Reason = "unmanaged file has identical content but a different mode"
		case observation.hash == artifact.hash:
			action.Kind = ActionAdopt
			action.Reason = "unmanaged file has identical content and can be adopted"
		default:
			action.Kind = ActionConflict
			action.Reason = "unmanaged file differs from the desired content"
		}
		if action.Kind == ActionConflict {
			result.ConflictCount++
		}
		if action.Kind == ActionUnchanged || action.Kind == ActionAdopt {
			action.DesiredPreview = ""
		}
		result.Actions = append(result.Actions, action)
	}
	sort.Slice(result.Actions, func(i, j int) bool { return result.Actions[i].Path < result.Actions[j].Path })
	return result, nil
}

func contentPreview(content []byte) string {
	const limit = 2048
	if !utf8.Valid(content) {
		return fmt.Sprintf("«binary content: %d bytes»", len(content))
	}
	if len(content) <= limit {
		return string(content)
	}
	end := limit
	for end > 0 && !utf8.RuneStart(content[end]) {
		end--
	}
	return string(content[:end]) + fmt.Sprintf("\n… preview truncated; %d total bytes", len(content))
}

// Apply plans from fresh filesystem state, refuses the entire operation set on
// any conflict, rechecks every precondition, writes each changed artifact by
// atomic replacement, and publishes bob.lock last. It never executes commands.
func Apply(root string, m manifest.Manifest, artifacts []recipe.Artifact) (ApplyResult, error) {
	canonicalRoot, err := validateRoot(root)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("apply: %w", err)
	}
	release, err := acquireApplyLock(canonicalRoot)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("apply: acquire workspace lock: %w", err)
	}
	defer release()

	plan, err := Plan(canonicalRoot, m, artifacts)
	result := ApplyResult{Plan: plan}
	if err != nil {
		return result, err
	}
	if plan.HasConflicts() {
		return result, fmt.Errorf("apply: %w", ErrPlanConflicts)
	}
	desired, err := normalizeArtifacts(canonicalRoot, artifacts)
	if err != nil {
		return result, fmt.Errorf("apply: %w", err)
	}
	byPath := make(map[string]desiredArtifact, len(desired))
	for _, artifact := range desired {
		byPath[artifact.path] = artifact
	}

	// Directory creation and staging occur only after the complete plan is known
	// to be conflict-free. Staged files are removed on every failure path.
	staged := make(map[string]string)
	defer func() {
		for _, path := range staged {
			_ = os.Remove(path)
		}
	}()
	for _, action := range plan.Actions {
		if action.Kind != ActionCreate && action.Kind != ActionUpdate {
			continue
		}
		artifact := byPath[action.Path]
		if err := ensureParentDirectories(canonicalRoot, action.Path); err != nil {
			return result, fmt.Errorf("apply: prepare %q: %w", action.Path, err)
		}
		tmp, err := stageFile(canonicalRoot, artifact)
		if err != nil {
			return result, fmt.Errorf("apply: stage %q: %w", action.Path, err)
		}
		staged[action.Path] = tmp
	}

	if err := recheckPlan(canonicalRoot, plan); err != nil {
		return result, fmt.Errorf("apply: stale plan: %w", err)
	}
	if err := recheckLock(canonicalRoot, plan); err != nil {
		return result, fmt.Errorf("apply: stale lock: %w", err)
	}

	for _, action := range plan.Actions {
		switch action.Kind {
		case ActionCreate, ActionUpdate:
			if err := publishStaged(canonicalRoot, action, staged[action.Path]); err != nil {
				return result, fmt.Errorf("apply: publish %q: %w", action.Path, err)
			}
			delete(staged, action.Path)
			result.Written = append(result.Written, action.Path)
		case ActionAdopt:
			result.Adopted = append(result.Adopted, action.Path)
		case ActionUnchanged:
			result.Unchanged = append(result.Unchanged, action.Path)
		}
	}

	lockData, err := encodeLock(plan.DesiredLock)
	if err != nil {
		return result, fmt.Errorf("apply: encode lock: %w", err)
	}
	currentLock, currentExists, err := readLockBytes(canonicalRoot)
	if err != nil {
		return result, fmt.Errorf("apply: read lock before publish: %w", err)
	}
	if !currentExists || string(currentLock) != string(lockData) {
		if err := writeAtomic(filepath.Join(canonicalRoot, LockFilename), lockData, 0o644, false); err != nil {
			return result, fmt.Errorf("apply: write lock: %w", err)
		}
		result.LockWritten = true
	}
	return result, nil
}

func normalizeArtifacts(root string, artifacts []recipe.Artifact) ([]desiredArtifact, error) {
	normalized := make([]desiredArtifact, 0, len(artifacts))
	seen := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		path, err := validateRelativePath(artifact.Path)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[path]; duplicate {
			return nil, fmt.Errorf("duplicate artifact path %q", path)
		}
		seen[path] = struct{}{}
		if err := destinationWithinRoot(root, path); err != nil {
			return nil, err
		}
		if artifact.Mode&fs.ModeType != 0 {
			return nil, fmt.Errorf("artifact %q has non-regular mode %s", path, artifact.Mode)
		}
		mode := artifact.Mode.Perm()
		if mode == 0 {
			mode = 0o644
		}
		copyContent := append([]byte(nil), artifact.Content...)
		normalized = append(normalized, desiredArtifact{
			Artifact: recipe.Artifact{Path: path, Mode: mode, Content: copyContent},
			path:     path,
			hash:     hashBytes(copyContent),
			mode:     mode,
		})
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].path < normalized[j].path })
	return normalized, nil
}

func desiredLock(m manifest.Manifest, artifacts []desiredArtifact) LockFile {
	lock := LockFile{
		SchemaVersion: LockSchemaVersion,
		Recipe:        LockRecipe{ID: m.Recipe, Version: RecipeVersion},
		Files:         make([]LockEntry, 0, len(artifacts)),
	}
	for _, artifact := range artifacts {
		lock.Files = append(lock.Files, LockEntry{Path: artifact.path, SHA256: artifact.hash})
	}
	return lock
}

func recheckPlan(root string, plan PlanResult) error {
	for _, action := range plan.Actions {
		observation, err := inspectDestination(root, action.Path)
		if err != nil {
			return fmt.Errorf("inspect %q: %w", action.Path, err)
		}
		if observation.exists != action.expectedExists {
			return fmt.Errorf("%q existence changed after planning", action.Path)
		}
		if observation.exists && (observation.hash != action.CurrentSHA256 || observation.mode != action.CurrentMode) {
			return fmt.Errorf("%q content or mode changed after planning", action.Path)
		}
	}
	return nil
}

func recheckLock(root string, plan PlanResult) error {
	data, exists, err := readLockBytes(root)
	if err != nil {
		return err
	}
	if exists != plan.lockExists {
		return errors.New("bob.lock existence changed after planning")
	}
	if exists && hashBytes(data) != plan.lockSHA256 {
		return errors.New("bob.lock changed after planning")
	}
	return nil
}
