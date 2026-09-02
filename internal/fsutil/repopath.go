package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Repository control files Bob owns outright. Every package that validates an
// artifact path (manifest, recipe, engine) shares these names through
// ValidateArtifactPath so the "never own bob.yaml/bob.lock" invariant has one
// implementation.
const (
	// ManifestFilename is the human-owned workspace contract.
	ManifestFilename = "bob.yaml"
	// LockFilename is Bob's exact whole-file ownership registry.
	LockFilename = "bob.lock"
	// ApplyLockFilename serializes mutating operations in one workspace.
	ApplyLockFilename = ".bob.apply.lock"
)

// ErrUnsafeArtifactPath reports a path that is empty, absolute, contains a
// NUL byte or volume name, or escapes the workspace root.
var ErrUnsafeArtifactPath = errors.New("unsafe artifact path")

// ErrReservedArtifactPath reports a path Bob refuses to let any recipe or
// manifest own: .git, bob.yaml, bob.lock, .bob.apply.lock, and their children.
var ErrReservedArtifactPath = errors.New("artifact path is reserved")

// ValidateArtifactPath canonicalizes a workspace-relative artifact path and
// rejects anything Bob must never write. The returned path is cleaned and
// slash-separated. It is the single source of truth for the path-safety
// invariant documented in AGENTS.md: no absolute paths, no escaping the
// workspace, no .git, and no ownership of Bob's own control files.
func ValidateArtifactPath(path string) (string, error) {
	original := path
	if path == "" || strings.ContainsRune(path, '\x00') {
		return "", fmt.Errorf("%w %q", ErrUnsafeArtifactPath, original)
	}
	if filepath.IsAbs(path) || filepath.VolumeName(path) != "" || strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return "", fmt.Errorf("%w %q", ErrUnsafeArtifactPath, original)
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w %q", ErrUnsafeArtifactPath, original)
	}
	clean = filepath.ToSlash(clean)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%w %q", ErrUnsafeArtifactPath, original)
	}
	for _, reserved := range []string{".git", ManifestFilename, LockFilename, ApplyLockFilename} {
		if clean == reserved || strings.HasPrefix(clean, reserved+"/") {
			return "", fmt.Errorf("%w: %q", ErrReservedArtifactPath, original)
		}
	}
	return clean, nil
}
