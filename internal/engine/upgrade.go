package engine

import (
	"errors"
	"fmt"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

// ErrUpgradeNoLock reports an upgrade against a workspace that has no
// bob.lock, so there is no recorded recipe version to migrate from.
var ErrUpgradeNoLock = errors.New("no lock file; run bob apply first")

// UpgradeOptions controls Upgrade's safety behavior. The embedded
// ApplyOptions carries the optional reviewed-plan digest constraint; DryRun
// plans the migration without writing anything.
type UpgradeOptions struct {
	ApplyOptions
	// DryRun performs every version and ownership check and reports what a
	// real run would write, but mutates nothing.
	DryRun bool
}

// UpgradeResult reports one recipe-version migration. Plan carries the fresh
// plan so transports can surface conflict detail; it is deliberately excluded
// from the wire format, which stays bounded to the migration summary.
type UpgradeResult struct {
	FromVersion int        `json:"from_version"`
	ToVersion   int        `json:"to_version"`
	Recipe      string     `json:"recipe"`
	Applied     bool       `json:"applied"`
	Actions     int        `json:"actions"`
	Written     []string   `json:"written,omitempty"`
	Plan        PlanResult `json:"-"`
}

// UpgradeStatus is the read-only recipe-version check. It returns the lock's
// recorded recipe version (from), the current binary's supported version
// (to), and whether a migration is needed. It errors when no lock exists, the
// lock names a different recipe, or the lock is newer than the binary
// supports.
func UpgradeStatus(root string, m manifest.Manifest) (from, to int, needsUpgrade bool, err error) {
	if err := m.Validate(); err != nil {
		return 0, 0, false, fmt.Errorf("upgrade: validate manifest: %w", err)
	}
	recipeVersion, err := recipe.Version(m.Recipe)
	if err != nil {
		return 0, 0, false, fmt.Errorf("upgrade: %w", err)
	}
	canonicalRoot, err := validateRoot(root)
	if err != nil {
		return 0, 0, false, fmt.Errorf("upgrade: %w", err)
	}
	lock, lockExists, _, err := loadLock(canonicalRoot)
	if err != nil {
		return 0, 0, false, fmt.Errorf("upgrade: %w", err)
	}
	if !lockExists {
		return 0, 0, false, fmt.Errorf("upgrade: %w", ErrUpgradeNoLock)
	}
	if lock.Recipe.ID != m.Recipe {
		return 0, 0, false, fmt.Errorf("upgrade: lock recipe %s@%d does not match %s", lock.Recipe.ID, lock.Recipe.Version, m.Recipe)
	}
	if lock.Recipe.Version > recipeVersion {
		return 0, 0, false, fmt.Errorf("upgrade: lock recipe %s@%d is newer than supported %s@%d", lock.Recipe.ID, lock.Recipe.Version, m.Recipe, recipeVersion)
	}
	return lock.Recipe.Version, recipeVersion, lock.Recipe.Version < recipeVersion, nil
}

// Upgrade migrates a workspace's bob.lock from an older supported recipe
// version to the current one by re-applying with the current recipe contract.
// It is a no-op when the lock is already at the current version, refuses a
// lock newer than the binary supports, and refuses the entire migration on any
// ownership conflict, exactly like Apply. Under DryRun it plans the migration
// and reports what would change without writing.
func Upgrade(root string, opts UpgradeOptions) (*UpgradeResult, error) {
	if err := validateApplyOptions(opts.ApplyOptions); err != nil {
		return nil, fmt.Errorf("upgrade: %w", err)
	}
	canonicalRoot, err := validateRoot(root)
	if err != nil {
		return nil, fmt.Errorf("upgrade: %w", err)
	}
	m, err := manifest.Load(canonicalRoot)
	if err != nil {
		return nil, fmt.Errorf("upgrade: %w: %w", ErrWorkspaceContract, err)
	}
	from, to, needsUpgrade, err := UpgradeStatus(canonicalRoot, m)
	if err != nil {
		return nil, err
	}
	result := &UpgradeResult{
		FromVersion: from,
		ToVersion:   to,
		Recipe:      m.Recipe,
		Written:     []string{},
	}
	if !needsUpgrade {
		return result, nil
	}
	if opts.DryRun {
		artifacts, renderErr := recipe.Render(m)
		if renderErr != nil {
			return nil, fmt.Errorf("upgrade: %w: render recipe: %w", ErrWorkspaceContract, renderErr)
		}
		plan, planErr := Plan(canonicalRoot, m, artifacts)
		result.Plan = plan
		if planErr != nil {
			return result, fmt.Errorf("upgrade: %w", planErr)
		}
		digest := DigestPlan(plan)
		if opts.ExpectedPlanDigest != "" && opts.ExpectedPlanDigest != digest.Qualified() {
			return result, &PlanDigestMismatchError{
				ExpectedPlanDigest: opts.ExpectedPlanDigest,
				ActualPlanDigest:   digest.Qualified(),
			}
		}
		if plan.HasConflicts() {
			return result, fmt.Errorf("upgrade: %w", ErrPlanConflicts)
		}
		for _, action := range plan.Actions {
			if action.Kind == ActionCreate || action.Kind == ActionUpdate {
				result.Written = append(result.Written, action.Path)
			}
		}
		result.Actions = len(result.Written)
		return result, nil
	}
	applyResult, applyErr := ApplyWorkspaceWithOptions(canonicalRoot, opts.ApplyOptions)
	result.Plan = applyResult.Plan
	if applyErr != nil {
		if errors.Is(applyErr, ErrPlanConflicts) {
			return result, fmt.Errorf("upgrade: %w", ErrPlanConflicts)
		}
		return result, fmt.Errorf("upgrade: %w", applyErr)
	}
	result.Applied = true
	result.Written = applyResult.Written
	result.Actions = len(applyResult.Written)
	return result, nil
}
