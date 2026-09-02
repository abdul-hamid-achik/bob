//go:build unix

package engine

import (
	"errors"
	"syscall"
)

// processAlive reports whether pid names a running process. Signal 0 performs
// only the existence and permission checks: ESRCH proves the process is gone;
// success or EPERM means it exists (possibly owned by another user), so the
// lock is kept.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return !errors.Is(err, syscall.ESRCH)
}
