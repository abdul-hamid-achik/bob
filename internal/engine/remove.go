package engine

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

// ErrNoLock reports a remove against a workspace that has no bob.lock, so
// there is no recorded ownership to remove.
var ErrNoLock = errors.New("no lock file; nothing to remove")

// RemoveOptions controls Remove's safety behavior.
type RemoveOptions struct {
	// Force removes managed files even when their current content no longer
	// matches the hash recorded in bob.lock. It never touches unmanaged
	// files, symlinks, or special files.
	Force bool
	// DryRun performs every ownership and hash check but removes nothing.
	DryRun bool
	// beforeDelete is a test seam for proving the final precondition check. It
	// is deliberately unexported so callers cannot insert behavior into the
	// destructive path.
	beforeDelete func(string)
}

type removeDecision uint8

const (
	removeMissing removeDecision = iota
	removeEligible
	removeSkipped
	removeConflict
)

// RemoveResult reports exactly what Remove changed. Under DryRun, Removed,
// Skipped, Conflicts, and LockRemoved describe what a real run would do;
// nothing is written. The path slices are sorted because they follow the
// deterministic lock order.
type RemoveResult struct {
	Removed     []string `json:"removed"`
	Skipped     []string `json:"skipped"`
	Conflicts   []string `json:"conflicts"`
	LockRemoved bool     `json:"lock_removed"`
}

// Remove deletes every Bob-managed file recorded in bob.lock whose current
// content still proves ownership, then removes bob.lock itself. Seed-once files
// are not lock-owned and therefore remain. Remove never touches unmanaged files,
// bob.yaml, symlinks, or special files; a managed file whose content drifted is
// skipped without Force and reported in Skipped. bob.lock is removed only when
// nothing was skipped or conflicted, so a partial remove keeps its ownership
// record for a later --force retry. Empty directories left behind by removed
// files are cleaned up bottom-up, never removing the workspace root or a
// directory that still holds other files.
func Remove(root string, opts RemoveOptions) (*RemoveResult, error) {
	canonicalRoot, err := validateRoot(root)
	if err != nil {
		return nil, fmt.Errorf("remove: %w", err)
	}
	release, err := acquireApplyLock(canonicalRoot)
	if err != nil {
		return nil, fmt.Errorf("remove: acquire workspace lock: %w", err)
	}
	defer release()

	lock, lockExists, lockBytes, err := loadLock(canonicalRoot)
	if err != nil {
		return nil, fmt.Errorf("remove: %w", err)
	}
	if !lockExists {
		return nil, fmt.Errorf("remove: %w", ErrNoLock)
	}

	m, err := manifest.Load(canonicalRoot)
	if err != nil {
		return nil, fmt.Errorf("remove: %w: %w", ErrWorkspaceContract, err)
	}
	recipeVersion, err := recipe.Version(m.Recipe)
	if err != nil {
		return nil, fmt.Errorf("remove: %w: %w", ErrWorkspaceContract, err)
	}
	if lock.Recipe.ID != m.Recipe {
		return nil, fmt.Errorf("remove: lock recipe %s@%d does not match %s", lock.Recipe.ID, lock.Recipe.Version, m.Recipe)
	}
	if lock.Recipe.Version > recipeVersion {
		return nil, fmt.Errorf("remove: lock recipe %s@%d is newer than supported %s@%d", lock.Recipe.ID, lock.Recipe.Version, m.Recipe, recipeVersion)
	}

	result := &RemoveResult{
		Removed:   []string{},
		Skipped:   []string{},
		Conflicts: []string{},
	}
	for _, entry := range lock.Files {
		path, err := validateRelativePath(entry.Path)
		if err != nil {
			return nil, fmt.Errorf("remove: %w", err)
		}
		if err := destinationWithinRoot(canonicalRoot, path); err != nil {
			return nil, fmt.Errorf("remove: %w", err)
		}
		observation, err := inspectDestination(canonicalRoot, path)
		if err != nil {
			return nil, fmt.Errorf("remove: inspect %q: %w", path, err)
		}
		decision := decideRemoval(observation, entry.SHA256, opts.Force)
		if recordNonRemoval(result, path, decision) {
			continue
		}
		if !opts.DryRun {
			if opts.beforeDelete != nil {
				opts.beforeDelete(path)
			}
			// Editors commonly save by replacing a file after the initial hash
			// check. Re-observe immediately before unlink so a newly written human
			// version is preserved unless --force explicitly authorized its removal.
			observation, err = inspectDestination(canonicalRoot, path)
			if err != nil {
				return result, fmt.Errorf("remove: recheck %q: %w", path, err)
			}
			decision = decideRemoval(observation, entry.SHA256, opts.Force)
			if recordNonRemoval(result, path, decision) {
				continue
			}
			destination := filepath.Join(canonicalRoot, filepath.FromSlash(path))
			if err := os.Remove(destination); err != nil {
				return nil, fmt.Errorf("remove: delete %q: %w", path, err)
			}
		}
		result.Removed = append(result.Removed, path)
	}

	// The lock is removed only when every managed path was fully cleared. A
	// skipped or conflicted file keeps its ownership record so a later
	// `remove --force` can still prove and delete it.
	canRemoveLock := len(result.Skipped) == 0 && len(result.Conflicts) == 0
	if opts.DryRun {
		result.LockRemoved = canRemoveLock
		return result, nil
	}
	removeEmptyDirectories(canonicalRoot, result.Removed)
	if canRemoveLock {
		currentLock, exists, err := readLockBytes(canonicalRoot)
		if err != nil {
			return result, fmt.Errorf("remove: recheck %s: %w", LockFilename, err)
		}
		if !exists || !bytes.Equal(currentLock, lockBytes) {
			return result, fmt.Errorf("remove: %s changed during removal; retained ownership state", LockFilename)
		}
		if err := os.Remove(filepath.Join(canonicalRoot, LockFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return result, fmt.Errorf("remove: delete %s: %w", LockFilename, err)
		}
		result.LockRemoved = true
	}
	return result, nil
}

func decideRemoval(observation observation, lockedSHA256 string, force bool) removeDecision {
	if !observation.exists {
		return removeMissing
	}
	if observation.conflictCode != "" {
		return removeConflict
	}
	if observation.hash != lockedSHA256 && !force {
		return removeSkipped
	}
	return removeEligible
}

// recordNonRemoval records a terminal non-delete decision and reports whether
// the caller should continue to the next managed path.
func recordNonRemoval(result *RemoveResult, path string, decision removeDecision) bool {
	switch decision {
	case removeMissing:
		return true
	case removeSkipped:
		result.Skipped = append(result.Skipped, path)
		return true
	case removeConflict:
		result.Conflicts = append(result.Conflicts, path)
		return true
	default:
		return false
	}
}

// removeEmptyDirectories removes directories left empty after file removal,
// walking deepest-first so nested directories clear before their parents. It
// only considers ancestors of removed paths and never the workspace root; a
// directory still holding unmanaged or skipped files stays in place because
// os.Remove refuses non-empty directories. Cleanup is best-effort: a leftover
// empty directory is harmless, so removal errors are ignored.
func removeEmptyDirectories(root string, removed []string) {
	dirs := make(map[string]struct{})
	for _, path := range removed {
		for dir := filepath.Dir(path); dir != "." && dir != string(os.PathSeparator); dir = filepath.Dir(dir) {
			dirs[dir] = struct{}{}
		}
	}
	sorted := make([]string, 0, len(dirs))
	for dir := range dirs {
		sorted = append(sorted, dir)
	}
	sort.Slice(sorted, func(i, j int) bool {
		depthI, depthJ := strings.Count(sorted[i], "/"), strings.Count(sorted[j], "/")
		if depthI != depthJ {
			return depthI > depthJ
		}
		return sorted[i] > sorted[j]
	})
	for _, dir := range sorted {
		_ = os.Remove(filepath.Join(root, filepath.FromSlash(dir)))
	}
}
