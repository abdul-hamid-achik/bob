package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

const (
	ApplyReceiptSchemaVersion = 1
	// ApplyReceiptByteLimit bounds the encoded receipt independently of the
	// number of files in a files@1 manifest. CLI envelopes add a small fixed
	// amount around this data object.
	ApplyReceiptByteLimit = 16 << 10
	// ApplyReceiptPathLimit also bounds pretty-printed transport overhead,
	// which grows per retained list item even when paths are very short.
	ApplyReceiptPathLimit = 256
)

var (
	ErrPlanDigestMismatch = errors.New("reviewed plan digest does not match current plan")
	// ErrWorkspaceContract marks a manifest load or recipe render failure in
	// the lock-owned apply path so transports can preserve invalid-input exit
	// semantics without owning workspace contract logic.
	ErrWorkspaceContract = errors.New("workspace contract is invalid")
)

// ApplyOptions adds optional approval constraints without changing ordinary
// Apply behavior. ExpectedPlanDigest must use the exact sha256:<hex> form.
type ApplyOptions struct {
	ExpectedPlanDigest string
}

// PlanDigestMismatchError retains both plan identities for typed CLI and
// future transport projections while keeping the sentinel branch stable.
type PlanDigestMismatchError struct {
	ExpectedPlanDigest string
	ActualPlanDigest   string
}

func (e *PlanDigestMismatchError) Error() string {
	return "the reviewed plan no longer matches the current workspace"
}

func (e *PlanDigestMismatchError) Unwrap() error { return ErrPlanDigestMismatch }

// ApplyNextCheck is a non-executing continuation. Engine returns argv rather
// than a shell string; callers remain responsible for authority and execution.
type ApplyNextCheck struct {
	Argv []string `json:"argv" yaml:"argv"`
}

// ApplyReceiptTruncation describes deterministic suffix omission from the
// path lists. Counts always describe the complete reconciliation.
type ApplyReceiptTruncation struct {
	ByteLimit int            `json:"byte_limit" yaml:"byte_limit"`
	Truncated bool           `json:"truncated" yaml:"truncated"`
	Omitted   map[string]int `json:"omitted" yaml:"omitted"`
}

// ApplyReceipt is the bounded, immediate identity of one reconciliation. It
// is returned to the caller and never persisted. It proves repository
// reconciliation only, not generated application behavior.
type ApplyReceipt struct {
	SchemaVersion       int                    `json:"schema_version" yaml:"schema_version"`
	PlanDigestVersion   int                    `json:"plan_digest_version" yaml:"plan_digest_version"`
	ExpectedPlanDigest  string                 `json:"expected_plan_digest,omitempty" yaml:"expected_plan_digest,omitempty"`
	AppliedPlanDigest   string                 `json:"applied_plan_digest" yaml:"applied_plan_digest"`
	Written             []string               `json:"written" yaml:"written"`
	Adopted             []string               `json:"adopted" yaml:"adopted"`
	Unchanged           []string               `json:"unchanged" yaml:"unchanged"`
	WrittenCount        int                    `json:"written_count" yaml:"written_count"`
	AdoptedCount        int                    `json:"adopted_count" yaml:"adopted_count"`
	UnchangedCount      int                    `json:"unchanged_count" yaml:"unchanged_count"`
	LockWritten         bool                   `json:"lock_written" yaml:"lock_written"`
	ConvergedAfterApply bool                   `json:"converged_after_apply" yaml:"converged_after_apply"`
	NextCheck           ApplyNextCheck         `json:"next_check" yaml:"next_check"`
	Truncation          ApplyReceiptTruncation `json:"truncation" yaml:"truncation"`
}

func newApplyReceipt(root, expected string, digest PlanDigest) ApplyReceipt {
	return ApplyReceipt{
		SchemaVersion:      ApplyReceiptSchemaVersion,
		PlanDigestVersion:  digest.Version,
		ExpectedPlanDigest: expected,
		AppliedPlanDigest:  digest.Qualified(),
		Written:            []string{},
		Adopted:            []string{},
		Unchanged:          []string{},
		NextCheck:          ApplyNextCheck{Argv: []string{"bob", "check", root, "--json"}},
		Truncation:         ApplyReceiptTruncation{ByteLimit: ApplyReceiptByteLimit, Omitted: map[string]int{}},
	}
}

func (r *ApplyResult) finalizeReceipt(converged bool) {
	r.Receipt.Written = append([]string{}, r.Written...)
	r.Receipt.Adopted = append([]string{}, r.Adopted...)
	r.Receipt.Unchanged = append([]string{}, r.Unchanged...)
	r.Receipt.WrittenCount = len(r.Written)
	r.Receipt.AdoptedCount = len(r.Adopted)
	r.Receipt.UnchangedCount = len(r.Unchanged)
	r.Receipt.LockWritten = r.LockWritten
	r.Receipt.ConvergedAfterApply = converged
	r.Receipt = boundApplyReceipt(r.Receipt)
}

// boundApplyReceipt keeps deterministic sorted prefixes and omits suffixes in
// lowest-value-first order: unchanged, adopted, then written. Reconciliation
// identity, complete counts, digest fields, convergence, and next_check are
// never removed.
func boundApplyReceipt(receipt ApplyReceipt) ApplyReceipt {
	receipt = limitApplyReceiptPaths(receipt)
	if applyReceiptSize(receipt) <= ApplyReceiptByteLimit {
		return receipt
	}
	receipt.Truncation.Truncated = true
	for _, field := range []string{"unchanged", "adopted", "written"} {
		paths := receiptPaths(receipt, field)
		if len(paths) == 0 {
			continue
		}
		low, high, best := 0, len(paths), -1
		for low <= high {
			mid := low + (high-low)/2
			candidate := setReceiptPaths(receipt, field, paths[:mid])
			candidate.Truncation.Omitted[field] = receiptPathCount(candidate, field) - mid
			if applyReceiptSize(candidate) <= ApplyReceiptByteLimit {
				best = mid
				low = mid + 1
			} else {
				high = mid - 1
			}
		}
		if best >= 0 {
			receipt = setReceiptPaths(receipt, field, paths[:best])
			receipt.Truncation.Omitted[field] = receiptPathCount(receipt, field) - best
			return receipt
		}
		receipt = setReceiptPaths(receipt, field, []string{})
		receipt.Truncation.Omitted[field] = receiptPathCount(receipt, field)
	}
	return receipt
}

func limitApplyReceiptPaths(receipt ApplyReceipt) ApplyReceipt {
	remaining := ApplyReceiptPathLimit
	for _, field := range []string{"written", "adopted", "unchanged"} {
		paths := receiptPaths(receipt, field)
		keep := min(len(paths), remaining)
		receipt = setReceiptPaths(receipt, field, paths[:keep])
		remaining -= keep
		if omitted := receiptPathCount(receipt, field) - keep; omitted > 0 {
			receipt.Truncation.Truncated = true
			receipt.Truncation.Omitted[field] = omitted
		}
	}
	return receipt
}

func receiptPathCount(receipt ApplyReceipt, field string) int {
	switch field {
	case "unchanged":
		return receipt.UnchangedCount
	case "adopted":
		return receipt.AdoptedCount
	default:
		return receipt.WrittenCount
	}
}

func receiptPaths(receipt ApplyReceipt, field string) []string {
	switch field {
	case "unchanged":
		return receipt.Unchanged
	case "adopted":
		return receipt.Adopted
	default:
		return receipt.Written
	}
}

func setReceiptPaths(receipt ApplyReceipt, field string, paths []string) ApplyReceipt {
	copyPaths := append([]string{}, paths...)
	switch field {
	case "unchanged":
		receipt.Unchanged = copyPaths
	case "adopted":
		receipt.Adopted = copyPaths
	default:
		receipt.Written = copyPaths
	}
	return receipt
}

func applyReceiptSize(receipt ApplyReceipt) int {
	data, err := json.Marshal(receipt)
	if err != nil {
		return ApplyReceiptByteLimit + 1
	}
	return len(data)
}

func recheckManifestSource(root string, source []byte) error {
	if source == nil {
		return nil
	}
	_, current, err := manifest.LoadWithSource(root)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, source) {
		return errors.New("bob.yaml changed after planning")
	}
	return nil
}

func planIsConverged(plan PlanResult) bool {
	if plan.HasConflicts() || plan.LockChanged {
		return false
	}
	for _, action := range plan.Actions {
		if action.Kind != ActionUnchanged {
			return false
		}
	}
	return true
}

func validateApplyOptions(options ApplyOptions) error {
	if options.ExpectedPlanDigest == "" {
		return nil
	}
	if err := ValidateExpectedPlanDigest(options.ExpectedPlanDigest); err != nil {
		return fmt.Errorf("validate expected plan digest: %w", err)
	}
	return nil
}
