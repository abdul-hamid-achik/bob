package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

// observation is a read-only snapshot of one destination path. conflictCode
// and conflictReason are set instead of returning a hard error when the
// destination (or a path component leading to it) is a symlink or another
// non-regular file: those are workspace states Plan reports as per-path
// conflicts, not reasons to abort the entire plan.
type observation struct {
	exists         bool
	hash           string
	mode           fs.FileMode
	conflictCode   string
	conflictReason string
}

// pathConflict signals that an ancestor directory component of a destination
// is a symlink or a non-directory, blocking safe traversal.
type pathConflict struct {
	code   string
	reason string
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validateRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("workspace root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	abs = filepath.Clean(abs)
	info, err := os.Lstat(abs)
	if err != nil {
		return "", fmt.Errorf("inspect workspace root: %w", err)
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("workspace root %s is not a regular directory", abs)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root symlinks: %w", err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve canonical workspace root: %w", err)
	}
	return filepath.Clean(resolved), nil
}

func validateRelativePath(path string) (string, error) {
	original := path
	if path == "" || strings.ContainsRune(path, '\x00') {
		return "", fmt.Errorf("unsafe artifact path %q", original)
	}
	if filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return "", fmt.Errorf("unsafe artifact path %q", original)
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe artifact path %q", original)
	}
	clean = filepath.ToSlash(clean)
	if clean == ".git" || strings.HasPrefix(clean, ".git/") ||
		clean == manifest.Filename || strings.HasPrefix(clean, manifest.Filename+"/") ||
		clean == LockFilename || strings.HasPrefix(clean, LockFilename+"/") ||
		clean == ApplyLockFilename || strings.HasPrefix(clean, ApplyLockFilename+"/") {
		return "", fmt.Errorf("artifact path %q is reserved", original)
	}
	return clean, nil
}

func destinationWithinRoot(root, relative string) error {
	destination := filepath.Join(root, filepath.FromSlash(relative))
	rel, err := filepath.Rel(root, destination)
	if err != nil {
		return fmt.Errorf("resolve artifact path %q: %w", relative, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("artifact path %q escapes workspace root", relative)
	}
	return nil
}

// inspectDestination observes one destination path without mutating it. A
// symlink or non-regular file at the destination, or a symlinked or
// non-directory ancestor component, is reported as a conflict observation
// rather than a hard error: Plan turns these into per-path ActionConflict
// actions instead of aborting. Genuinely unrecoverable conditions (an
// unreadable ancestor, a destination that changes identity mid-read) still
// return an error.
func inspectDestination(root, relative string) (observation, error) {
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
	if info.Mode()&fs.ModeSymlink != 0 {
		return observation{exists: true, mode: info.Mode(), conflictCode: CodeSymlink, conflictReason: "destination is a symlink"}, nil
	}
	if !info.Mode().IsRegular() {
		return observation{exists: true, mode: info.Mode(), conflictCode: CodeSpecialFile, conflictReason: "destination is not a regular file"}, nil
	}
	file, err := os.Open(destination)
	if err != nil {
		return observation{}, err
	}
	defer func() { _ = file.Close() }()
	openedInfo, err := file.Stat()
	if err != nil {
		return observation{}, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return observation{}, errors.New("destination changed while it was being opened")
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return observation{}, err
	}
	return observation{
		exists: true,
		hash:   hex.EncodeToString(hasher.Sum(nil)),
		mode:   info.Mode().Perm(),
	}, nil
}

// inspectPathComponents flags every existing symlink and non-directory
// ancestor as a conflict instead of erroring. The destination itself is
// inspected separately.
func inspectPathComponents(root, destination string) (*pathConflict, error) {
	relative, err := filepath.Rel(root, destination)
	if err != nil {
		return nil, err
	}
	components := strings.Split(relative, string(os.PathSeparator))
	current := root
	for _, component := range components[:len(components)-1] {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return &pathConflict{code: CodeSymlink, reason: fmt.Sprintf("path component %s is a symlink", current)}, nil
		}
		if !info.IsDir() {
			return &pathConflict{code: CodeSpecialFile, reason: fmt.Sprintf("path component %s is not a directory", current)}, nil
		}
	}
	return nil, nil
}

// readCurrentPreview returns a bounded preview of a regular file's current
// bytes, or "" when the path is missing, not a regular file, or unreadable.
// It is called only for conflict/update actions on paths inspectDestination
// already proved are regular files, so it never follows a symlink or opens a
// special file; callers still guard on observation.conflictCode == "" as
// defense in depth.
func readCurrentPreview(destination string) string {
	const limit = 2048
	file, err := os.Open(destination)
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return ""
	}
	if !utf8.Valid(data) {
		return fmt.Sprintf("«binary content: %d bytes»", info.Size())
	}
	if int64(len(data)) <= limit {
		return string(data)
	}
	end := limit
	for end > 0 && !utf8.RuneStart(data[end]) {
		end--
	}
	return string(data[:end]) + fmt.Sprintf("\n… preview truncated; %d total bytes", info.Size())
}

func ensureParentDirectories(root, relative string) error {
	parent := filepath.Dir(filepath.Join(root, filepath.FromSlash(relative)))
	relativeParent, err := filepath.Rel(root, parent)
	if err != nil {
		return err
	}
	if relativeParent == "." {
		return nil
	}
	current := root
	for _, component := range strings.Split(relativeParent, string(os.PathSeparator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(current, 0o755); err != nil {
				return err
			}
			info, err = os.Lstat(current)
		}
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("path component %s is not a regular directory", current)
		}
	}
	return nil
}

func stageFile(root string, artifact desiredArtifact) (string, error) {
	destination := filepath.Join(root, filepath.FromSlash(artifact.path))
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".bob-stage-*")
	if err != nil {
		return "", err
	}
	name := tmp.Name()
	cleanup := func(cause error) (string, error) {
		_ = tmp.Close()
		_ = os.Remove(name)
		return "", cause
	}
	if err := tmp.Chmod(artifact.mode); err != nil {
		return cleanup(err)
	}
	if _, err := tmp.Write(artifact.Content); err != nil {
		return cleanup(err)
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

func publishStaged(root string, action Action, staged string) error {
	destination := filepath.Join(root, filepath.FromSlash(action.Path))
	if action.Kind == ActionCreate {
		// Link is an atomic no-replace publication: a destination that appears
		// after the final precondition check cannot be overwritten.
		if err := os.Link(staged, destination); err != nil {
			return err
		}
		if err := os.Remove(staged); err != nil {
			return err
		}
		return nil
	}
	if action.Kind != ActionUpdate {
		return fmt.Errorf("cannot publish action %s", action.Kind)
	}
	// Recheck this update immediately before atomic replacement. Rename remains
	// the platform atomicity boundary for an already-owned whole file.
	observation, err := inspectDestination(root, action.Path)
	if err != nil {
		return err
	}
	if !observation.exists || observation.hash != action.CurrentSHA256 || observation.mode != action.CurrentMode {
		return errors.New("destination changed immediately before replacement")
	}
	if err := os.Rename(staged, destination); err != nil {
		return err
	}
	return nil
}

func writeAtomic(path string, data []byte, mode fs.FileMode, noReplace bool) error {
	parent := filepath.Dir(path)
	tmp, err := os.CreateTemp(parent, ".bob-atomic-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if err := tmp.Chmod(mode.Perm()); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if noReplace {
		if err := os.Link(name, path); err != nil {
			return err
		}
		return os.Remove(name)
	}
	return os.Rename(name, path)
}
