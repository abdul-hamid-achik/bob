package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

const PlanDigestVersion = 1

const planDigestPrefix = "sha256:"

var (
	// ErrInvalidPlanDigest reports an expected digest that is not the exact
	// lowercase, algorithm-qualified version-1 representation accepted by
	// digest-gated apply.
	ErrInvalidPlanDigest = errors.New("invalid plan digest")
	planDigestPattern    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// PlanDigest identifies the complete semantic plan independently of output
// filtering and bounded content previews. SHA256 is lowercase hexadecimal so
// existing MCP consumers retain the version-1 wire value unchanged.
type PlanDigest struct {
	Version int    `json:"version"`
	SHA256  string `json:"sha256"`
}

// Qualified returns the stable algorithm-qualified representation accepted
// by digest-gated apply. The version travels separately in structured output.
func (d PlanDigest) Qualified() string { return planDigestPrefix + d.SHA256 }

// ValidateExpectedPlanDigest accepts exactly "sha256:" followed by 64
// lowercase hexadecimal characters. It deliberately performs no case or
// whitespace normalization so reviewed authority cannot bind ambiguously.
func ValidateExpectedPlanDigest(value string) error {
	if !planDigestPattern.MatchString(value) {
		return fmt.Errorf("%w: expected sha256: followed by 64 lowercase hexadecimal characters", ErrInvalidPlanDigest)
	}
	return nil
}

type digestAction struct {
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	Code          string `json:"code,omitempty"`
	CurrentSHA256 string `json:"current_sha256,omitempty"`
	DesiredSHA256 string `json:"desired_sha256,omitempty"`
	LockedSHA256  string `json:"locked_sha256,omitempty"`
	CurrentMode   string `json:"current_mode,omitempty"`
	DesiredMode   string `json:"desired_mode,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

// DigestPlan implements the original MCP version-1 plan identity. Keep this
// projection stable: previews, private planner fields, conflict counts, and
// output filtering are intentionally excluded.
func DigestPlan(plan PlanResult) PlanDigest {
	actions := make([]digestAction, 0, len(plan.Actions))
	for _, action := range plan.Actions {
		projected := digestAction{
			Path: action.Path, Kind: string(action.Kind), Code: action.Code,
			CurrentSHA256: action.CurrentSHA256, DesiredSHA256: action.DesiredSHA256,
			LockedSHA256: action.LockedSHA256,
			DesiredMode:  fmt.Sprintf("%04o", action.DesiredMode.Perm()),
			Reason:       action.Reason,
		}
		if action.CurrentMode != 0 {
			projected.CurrentMode = fmt.Sprintf("%04o", action.CurrentMode.Perm())
		}
		actions = append(actions, projected)
	}
	identity := struct {
		SchemaVersion int            `json:"schema_version"`
		Recipe        LockRecipe     `json:"recipe"`
		LockChanged   bool           `json:"lock_changed"`
		DesiredLock   LockFile       `json:"desired_lock"`
		Actions       []digestAction `json:"actions"`
	}{
		SchemaVersion: plan.SchemaVersion,
		Recipe:        plan.Recipe,
		LockChanged:   plan.LockChanged,
		DesiredLock:   plan.DesiredLock,
		Actions:       actions,
	}
	data, _ := json.Marshal(identity)
	digest := sha256.Sum256(data)
	return PlanDigest{Version: PlanDigestVersion, SHA256: hex.EncodeToString(digest[:])}
}
