package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

const (
	PathClassificationManaged   = "managed"
	PathClassificationReserved  = "reserved"
	PathClassificationUnmanaged = "unmanaged"
	PathClassificationMissing   = "missing"

	PathStateManagedInSync    = "managed_in_sync"
	PathStateManagedModified  = "managed_modified"
	PathStateManagedMissing   = "managed_missing"
	PathStateRetiredOwned     = "retired_owned"
	PathStateUnmanagedPresent = "unmanaged_present"
	PathStateUnmanagedMissing = "unmanaged_missing"
	PathStateReserved         = "reserved"
	PathStateSymlink          = "symlink"
	PathStateSpecialFile      = "special_file"

	HumanEditWillConflict     = "will_conflict"
	HumanEditOutsideOwnership = "outside_bob_ownership"
	HumanEditReservedForBob   = "reserved_for_bob"
	HumanEditRequiresManifest = "requires_manifest_change"
	HumanEditUnsafe           = "unsafe"
)

// PathClassification is the engine-owned relationship between Bob's exact
// desired/locked whole-file ownership and one repository-relative path. It
// deliberately makes no claim that an unmanaged path is safe for the product.
type PathClassification struct {
	Path            string      `json:"path"`
	Exists          bool        `json:"exists"`
	Classification  string      `json:"classification"`
	State           string      `json:"state"`
	HumanEditEffect string      `json:"human_edit_effect"`
	LockedSHA256    string      `json:"locked_sha256,omitempty"`
	CurrentSHA256   string      `json:"current_sha256,omitempty"`
	PlanAction      *PathAction `json:"plan_action,omitempty"`
}

type PathAction struct {
	Kind ActionKind `json:"kind"`
	Code string     `json:"code"`
}

// NormalizeRepositoryPath validates a caller-supplied repository-relative
// path without treating Bob's reserved control paths as ordinary artifacts.
func NormalizeRepositoryPath(path string) (string, error) {
	original := path
	if !utf8.ValidString(path) || len(path) > 4096 {
		return "", errors.New("unsafe repository path: expected valid UTF-8 containing at most 4096 bytes")
	}
	if path == "" || strings.ContainsRune(path, '\x00') {
		return "", fmt.Errorf("unsafe repository path %q", original)
	}
	if filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return "", fmt.Errorf("repository path must be relative: %q", original)
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("repository path must not escape the workspace: %q", original)
	}
	return filepath.ToSlash(clean), nil
}

// ClassifyPath applies the same lock, observation, and plan identities used by
// planning. The supplied plan must have been calculated for root immediately
// before this call; no repository state is changed.
func ClassifyPath(root string, plan PlanResult, requested string) (PathClassification, error) {
	path, err := NormalizeRepositoryPath(requested)
	if err != nil {
		return PathClassification{}, err
	}
	canonicalRoot, err := validateRoot(root)
	if err != nil {
		return PathClassification{}, fmt.Errorf("classify path: %w", err)
	}
	if plan.canonicalRoot != canonicalRoot {
		return PathClassification{}, errors.New("classify path: plan does not belong to the requested workspace")
	}
	if effect, reserved := reservedPathEffect(path); reserved {
		return PathClassification{Path: path, Exists: pathExistsNoFollow(canonicalRoot, path), Classification: PathClassificationReserved, State: PathStateReserved, HumanEditEffect: effect}, nil
	}
	action, hasAction := planActionForPath(plan, path)
	desired := desiredPath(plan, path)
	locked := false
	if plan.lockExists {
		for _, entry := range plan.observedLock.Files {
			if entry.Path == path {
				locked = true
				break
			}
		}
	}
	var observation observation
	if hasAction {
		observation = observationFromAction(action)
	} else {
		observation, err = inspectDestinationKind(canonicalRoot, path)
		if err != nil {
			return PathClassification{}, fmt.Errorf("classify path %q: %w", path, err)
		}
	}
	result := PathClassification{Path: path, Exists: observation.exists, CurrentSHA256: observation.hash}
	if hasAction {
		result.PlanAction = &PathAction{Kind: action.Kind, Code: action.Code}
		result.LockedSHA256 = action.LockedSHA256
	}
	if locked {
		for _, entry := range plan.observedLock.Files {
			if entry.Path == path {
				result.LockedSHA256 = entry.SHA256
				break
			}
		}
	}
	if observation.conflictCode == CodeSymlink {
		result.Exists = pathExistsNoFollow(canonicalRoot, path)
		result.Classification = PathClassificationUnmanaged
		if desired || locked {
			result.Classification = PathClassificationManaged
			result.HumanEditEffect = HumanEditWillConflict
		} else {
			result.HumanEditEffect = HumanEditUnsafe
		}
		result.State = PathStateSymlink
		return result, nil
	}
	if observation.conflictCode == CodeSpecialFile {
		result.Exists = pathExistsNoFollow(canonicalRoot, path)
		result.Classification = PathClassificationUnmanaged
		if desired || locked {
			result.Classification = PathClassificationManaged
			result.HumanEditEffect = HumanEditWillConflict
		} else {
			result.HumanEditEffect = HumanEditUnsafe
		}
		result.State = PathStateSpecialFile
		return result, nil
	}
	if locked && !desired {
		result.Classification = PathClassificationManaged
		result.State = PathStateRetiredOwned
		result.HumanEditEffect = HumanEditReservedForBob
		return result, nil
	}
	if desired {
		result.Classification = PathClassificationManaged
		result.HumanEditEffect = HumanEditWillConflict
		switch action.Code {
		case CodeInSync:
			result.State = PathStateManagedInSync
		case CodeMissing, CodeManagedMissing:
			result.State = PathStateManagedMissing
		default:
			result.State = PathStateManagedModified
		}
		return result, nil
	}
	result.HumanEditEffect = HumanEditOutsideOwnership
	if observation.exists {
		result.Classification = PathClassificationUnmanaged
		result.State = PathStateUnmanagedPresent
	} else {
		result.Classification = PathClassificationMissing
		result.State = PathStateUnmanagedMissing
	}
	return result, nil
}

func planActionForPath(plan PlanResult, path string) (Action, bool) {
	for _, action := range plan.Actions {
		if action.Path == path {
			return action, true
		}
	}
	return Action{}, false
}

func desiredPath(plan PlanResult, path string) bool {
	for _, entry := range plan.DesiredLock.Files {
		if entry.Path == path {
			return true
		}
	}
	return false
}

func observationFromAction(action Action) observation {
	conflictCode := ""
	if action.Code == CodeSymlink || action.Code == CodeSpecialFile {
		conflictCode = action.Code
	}
	return observation{exists: action.expectedExists, hash: action.CurrentSHA256, mode: action.CurrentMode, conflictCode: conflictCode}
}

func inspectDestinationKind(root, relative string) (observation, error) {
	if err := destinationWithinRoot(root, relative); err != nil {
		return observation{}, err
	}
	destination := filepath.Join(root, filepath.FromSlash(relative))
	conflict, err := inspectPathComponents(root, destination)
	if err != nil {
		return observation{}, err
	}
	if conflict != nil {
		return observation{exists: true, conflictCode: conflict.code, conflictReason: conflict.reason}, nil
	}
	info, err := os.Lstat(destination)
	if errors.Is(err, os.ErrNotExist) {
		return observation{}, nil
	}
	if err != nil {
		return observation{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return observation{exists: true, mode: info.Mode(), conflictCode: CodeSymlink}, nil
	}
	if !info.Mode().IsRegular() {
		return observation{exists: true, mode: info.Mode(), conflictCode: CodeSpecialFile}, nil
	}
	return observation{exists: true, mode: info.Mode().Perm()}, nil
}

func reservedPathEffect(path string) (string, bool) {
	switch {
	case path == manifest.Filename:
		return HumanEditRequiresManifest, true
	case strings.HasPrefix(path, manifest.Filename+"/"):
		return HumanEditUnsafe, true
	case path == LockFilename || strings.HasPrefix(path, LockFilename+"/"), path == ApplyLockFilename || strings.HasPrefix(path, ApplyLockFilename+"/"), path == ".git" || strings.HasPrefix(path, ".git/"):
		return HumanEditReservedForBob, true
	default:
		return "", false
	}
}

func pathExistsNoFollow(root, path string) bool {
	if err := destinationWithinRoot(root, path); err != nil {
		return false
	}
	destination := filepath.Join(root, filepath.FromSlash(path))
	conflict, err := inspectPathComponents(root, destination)
	if err != nil || conflict != nil {
		return false
	}
	_, err = os.Lstat(destination)
	return err == nil
}
