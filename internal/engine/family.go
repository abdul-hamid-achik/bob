package engine

// ActionFamily is the closed classification vocabulary that groups plan
// actions by cause rather than by per-path code. It is shared by the context
// verdict, the human plan renderer, and apply guidance so every consumer
// branches on one vocabulary instead of re-deriving it from codes.
type ActionFamily string

const (
	// FamilyOwnershipHazard marks an unsafe destination regardless of content:
	// an existing symlink or special file Bob must never write through.
	FamilyOwnershipHazard ActionFamily = "ownership_hazard"
	// FamilyContractDrift marks a Bob-owned file that drifted from the hash
	// recorded in bob.lock (or a retired file the lock still owns).
	FamilyContractDrift ActionFamily = "contract_drift"
	// FamilyUnmanagedDivergence marks a recipe proposal over a file Bob never
	// owned: the repository evolved independently of the recipe.
	FamilyUnmanagedDivergence ActionFamily = "unmanaged_divergence"
	// FamilyScaffold marks a create of a file that does not exist yet,
	// including seed creates.
	FamilyScaffold ActionFamily = "scaffold"
	// FamilyConvergence marks conflict-free actions: safe lock-proven
	// updates, adoptions, unchanged, and seed-satisfied destinations.
	FamilyConvergence ActionFamily = "convergence"
)

// ConflictClass values are the closed classification of a plan's conflicts as
// a whole: no conflicts, exactly one family, or mixed.
const (
	ConflictClassNone                = "none"
	ConflictClassOwnershipHazard     = "ownership_hazard"
	ConflictClassContractDrift       = "contract_drift"
	ConflictClassUnmanagedDivergence = "unmanaged_divergence"
	ConflictClassMixed               = "mixed"
)

// familyByCode is the total map from the closed action-code vocabulary to a
// family. A new code added to the const block in engine.go REQUIRES a new
// entry here AND in family_test.go's closedActionCodes mirror: Go cannot
// enumerate consts, so the discipline note is the real guard, and
// TestFamilyByCodeMatchesClosedVocabularyMirror fails loudly on the
// half-update. A missing entry silently skips ConflictFamilyCounts, can make
// ConflictClass report "none" with ConflictCount > 0, and makes context emit
// a malformed "conflict_" reason code.
var familyByCode = map[string]ActionFamily{
	CodeSymlink:              FamilyOwnershipHazard,
	CodeSpecialFile:          FamilyOwnershipHazard,
	CodeManagedHashMismatch:  FamilyContractDrift,
	CodeManagedMissing:       FamilyContractDrift,
	CodeRetiredOwned:         FamilyContractDrift,
	CodeUnmanagedDiffers:     FamilyUnmanagedDivergence,
	CodeUnmanagedModeDiffers: FamilyUnmanagedDivergence,
	CodeMissing:              FamilyScaffold,
	CodeModeDrift:            FamilyConvergence,
	CodeContentUpdate:        FamilyConvergence,
	CodeInSync:               FamilyConvergence,
	CodeIdenticalContent:     FamilyConvergence,
	CodeSeedExists:           FamilyConvergence,
}

// Family classifies the action by its closed code vocabulary. An unknown code
// yields the empty family; the planner only emits closed codes, so the empty
// value surfaces a vocabulary gap instead of guessing.
func (a Action) Family() ActionFamily { return familyByCode[a.Code] }

// ActionCounts is the per-kind action tally shared by read-only projections.
type ActionCounts struct {
	Create    int `json:"create"`
	Update    int `json:"update"`
	Adopt     int `json:"adopt"`
	Unchanged int `json:"unchanged"`
	Conflict  int `json:"conflict"`
}

// ActionCounts tallies every action by kind.
func (p PlanResult) ActionCounts() ActionCounts {
	var counts ActionCounts
	for _, action := range p.Actions {
		switch action.Kind {
		case ActionCreate:
			counts.Create++
		case ActionUpdate:
			counts.Update++
		case ActionAdopt:
			counts.Adopt++
		case ActionUnchanged:
			counts.Unchanged++
		case ActionConflict:
			counts.Conflict++
		}
	}
	return counts
}

// ConflictFamilyCounts reports the conflict count per conflict family. The
// map always carries all three conflict families so consumers never branch on
// key presence.
func (p PlanResult) ConflictFamilyCounts() map[string]int {
	counts := map[string]int{
		string(FamilyOwnershipHazard):     0,
		string(FamilyContractDrift):       0,
		string(FamilyUnmanagedDivergence): 0,
	}
	for _, action := range p.Actions {
		if action.Kind != ActionConflict {
			continue
		}
		if _, known := counts[string(action.Family())]; known {
			counts[string(action.Family())]++
		}
	}
	return counts
}

// ConflictClass classifies the plan's conflicts as a whole: none when
// conflict-free, the single family name when every conflict shares one
// family, and mixed otherwise.
func (p PlanResult) ConflictClass() string {
	counts := p.ConflictFamilyCounts()
	present := 0
	class := ConflictClassNone
	for _, family := range []ActionFamily{FamilyOwnershipHazard, FamilyContractDrift, FamilyUnmanagedDivergence} {
		if counts[string(family)] > 0 {
			present++
			class = string(family)
		}
	}
	if present > 1 {
		return ConflictClassMixed
	}
	return class
}

// DominantConflictFamily reports the most severe conflict family present,
// with precedence ownership_hazard > contract_drift > unmanaged_divergence.
// It is empty when the plan has no conflicts.
func (p PlanResult) DominantConflictFamily() ActionFamily {
	counts := p.ConflictFamilyCounts()
	for _, family := range []ActionFamily{FamilyOwnershipHazard, FamilyContractDrift, FamilyUnmanagedDivergence} {
		if counts[string(family)] > 0 {
			return family
		}
	}
	return ""
}

// LockExists reports whether a coherent bob.lock snapshot was observed when
// this plan was calculated. LockChanged is true both for lock drift and for
// "the recipe was never applied"; LockExists distinguishes the two.
func (p PlanResult) LockExists() bool { return p.lockExists }
