//go:build !unix

package engine

// processAlive conservatively reports every pid as alive on platforms without
// a cheap liveness probe, so stale apply locks are never reclaimed there.
func processAlive(int) bool { return true }
