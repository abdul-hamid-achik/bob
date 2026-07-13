// Package strsim provides small, dependency-free string similarity helpers
// used to build "did you mean" suggestions for a low-capability coding agent
// that mistyped a closed-vocabulary value (a recipe id, an enum choice).
package strsim

// Distance returns the Levenshtein edit distance between a and b: the
// minimum number of single-rune insertions, deletions, or substitutions
// needed to turn a into b.
func Distance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			best := prev[j] + 1 // deletion
			if ins := curr[j-1] + 1; ins < best {
				best = ins // insertion
			}
			if sub := prev[j-1] + cost; sub < best {
				best = sub // substitution
			}
			curr[j] = best
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}

// Closest returns the candidate nearest to target by edit distance, when
// that distance is at most maxDistance. It reports false when candidates is
// empty or no candidate is within maxDistance.
func Closest(target string, candidates []string, maxDistance int) (string, bool) {
	best := ""
	bestDistance := maxDistance + 1
	for _, candidate := range candidates {
		if d := Distance(target, candidate); d < bestDistance {
			bestDistance = d
			best = candidate
		}
	}
	if bestDistance > maxDistance {
		return "", false
	}
	return best, true
}
