// Package workspace resolves repository roots without following a symlink at
// the selected boundary.
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abdul-hamid-achik/bob/internal/fsutil"
)

// Resolve rejects a symlink at the selected workspace boundary and resolves
// symlinks only in already-existing ancestors. Missing descendants are
// appended to that canonical ancestor without following a selected link.
func Resolve(path string, mustExist bool) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("workspace path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	info, err := os.Lstat(abs)
	if err == nil {
		if fsutil.IsSymlinkOrNotDir(info) {
			return "", fmt.Errorf("workspace %s is not a regular directory", abs)
		}
		return filepath.EvalSymlinks(abs)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if mustExist {
		return "", fmt.Errorf("workspace %s does not exist: %w", abs, err)
	}

	ancestor := abs
	for {
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", fmt.Errorf("no existing ancestor for workspace %s", abs)
		}
		ancestor = parent
		info, err = os.Lstat(ancestor)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	if fsutil.IsSymlinkOrNotDir(info) {
		return "", fmt.Errorf("workspace ancestor %s is not a regular directory", ancestor)
	}
	resolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	remainder, err := filepath.Rel(ancestor, abs)
	if err != nil || remainder == ".." || strings.HasPrefix(remainder, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("workspace %s escapes its canonical ancestor", abs)
	}
	return filepath.Join(resolved, remainder), nil
}
