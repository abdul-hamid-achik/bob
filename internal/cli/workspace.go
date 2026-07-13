package cli

import (
	"github.com/abdul-hamid-achik/bob/internal/workspace"
)

// safeWorkspacePath rejects a symlink at the selected workspace boundary and
// resolves symlinks only in already-existing ancestors. Missing descendants
// are appended to that canonical ancestor without following a selected link.
func safeWorkspacePath(path string, mustExist bool) (string, error) {
	return workspace.Resolve(path, mustExist)
}
