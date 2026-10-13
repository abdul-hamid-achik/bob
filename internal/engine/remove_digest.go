package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// RemovalDigestVersion identifies the removal-decision projection hashed by
// DigestRemoval. It is a separate namespace from PlanDigestVersion.
const RemovalDigestVersion = 1

// removalDigestKind domain-separates removal identities from plan identities
// so a reviewed plan digest can never authorize a removal, or vice versa.
const removalDigestKind = "remove"

var (
	// ErrInvalidRemovalDigest reports an expected removal digest that is not
	// the exact lowercase, algorithm-qualified version-1 representation.
	ErrInvalidRemovalDigest = errors.New("invalid removal digest")
)

// RemovalDigestMismatchError retains both removal identities. It unwraps to
// ErrPlanDigestMismatch so transports reuse the reviewed-authority mismatch
// classification shared with apply and upgrade.
type RemovalDigestMismatchError struct {
	ExpectedRemovalDigest string
	ActualRemovalDigest   string
}

func (e *RemovalDigestMismatchError) Error() string {
	return "the reviewed removal no longer matches the current workspace"
}

func (e *RemovalDigestMismatchError) Unwrap() error { return ErrPlanDigestMismatch }

// ValidateExpectedRemovalDigest accepts exactly "sha256:" followed by 64
// lowercase hexadecimal characters, with no normalization.
func ValidateExpectedRemovalDigest(value string) error {
	if !planDigestPattern.MatchString(value) {
		return fmt.Errorf("%w: expected sha256: followed by 64 lowercase hexadecimal characters", ErrInvalidRemovalDigest)
	}
	return nil
}

// removalDigestEntry is the per-path removal decision projection.
type removalDigestEntry struct {
	Path          string `json:"path"`
	LockedSHA256  string `json:"locked_sha256"`
	CurrentSHA256 string `json:"current_sha256"`
	CurrentMode   string `json:"current_mode"`
	Decision      string `json:"decision"`
}

func (d removeDecision) String() string {
	switch d {
	case removeMissing:
		return "missing"
	case removeEligible:
		return "remove"
	case removeSkipped:
		return "skip"
	case removeConflict:
		return "conflict"
	default:
		return "unknown"
	}
}

func newRemovalDigestEntry(path, lockedSHA256 string, observed observation, decision removeDecision) removalDigestEntry {
	entry := removalDigestEntry{
		Path:          path,
		LockedSHA256:  lockedSHA256,
		CurrentSHA256: observed.hash,
		Decision:      decision.String(),
	}
	if observed.exists && observed.conflictCode == "" {
		entry.CurrentMode = fmt.Sprintf("%04o", observed.mode.Perm())
	}
	return entry
}

// digestRemoval returns the qualified removal digest over the exact loaded
// bob.lock bytes, the force flag, and every per-path decision in lock order.
func digestRemoval(lockBytes []byte, force bool, entries []removalDigestEntry) string {
	lockSum := sha256.Sum256(lockBytes)
	if entries == nil {
		entries = []removalDigestEntry{}
	}
	identity := struct {
		Kind          string               `json:"kind"`
		SchemaVersion int                  `json:"schema_version"`
		LockSHA256    string               `json:"lock_sha256"`
		Force         bool                 `json:"force"`
		Entries       []removalDigestEntry `json:"entries"`
	}{
		Kind:          removalDigestKind,
		SchemaVersion: RemovalDigestVersion,
		LockSHA256:    hex.EncodeToString(lockSum[:]),
		Force:         force,
		Entries:       entries,
	}
	data, _ := json.Marshal(identity)
	sum := sha256.Sum256(data)
	return planDigestPrefix + hex.EncodeToString(sum[:])
}
