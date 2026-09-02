package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

// diffByteLimit caps the content size PlanDiff will attempt to diff. Files
// whose old or new content exceeds this limit are skipped with a note so a
// large generated artifact never forces unbounded work.
const diffByteLimit = 1 << 20 // 1 MiB

// diffCellLimit bounds the longest-common-subsequence table computeEdits may
// allocate, measured in cells over the changed region after the common prefix
// and suffix are trimmed. Each cell is an int32, so the table never exceeds
// 16 MiB regardless of how many lines the files contain; a file with a
// localized change diffs in full at any length, and only a change region that
// is itself huge on both sides is skipped with a note.
const diffCellLimit = 1 << 22

// FileDiff is the presentation-layer diff for one create or update action.
// It is computed after planning and never influences the plan digest.
type FileDiff struct {
	Path     string   `json:"path"`
	Kind     string   `json:"kind"`                // "create" or "update"
	OldLines []string `json:"old_lines,omitempty"` // nil for create
	NewLines []string `json:"new_lines"`
	Unified  string   `json:"unified"` // unified diff format
	Note     string   `json:"note,omitempty"`
}

// PlanDiff produces content diffs for every create and update action in a
// plan. It is a read-only presentation helper: the plan digest, action list,
// and engine behavior are never affected. Artifacts supply the desired
// content that PlanResult intentionally does not carry.
func PlanDiff(root string, plan *PlanResult, artifacts []recipe.Artifact) ([]FileDiff, error) {
	contentByPath := make(map[string][]byte, len(artifacts))
	for _, artifact := range artifacts {
		contentByPath[artifact.Path] = artifact.Content
	}
	diffs := make([]FileDiff, 0)
	for _, action := range plan.Actions {
		if action.Kind != ActionCreate && action.Kind != ActionUpdate {
			continue
		}
		newContent, ok := contentByPath[action.Path]
		if !ok {
			return nil, fmt.Errorf("plan diff: no artifact content for %q", action.Path)
		}
		var oldContent []byte
		if action.Kind == ActionUpdate {
			data, exists, err := readRegularFile(filepath.Join(root, filepath.FromSlash(action.Path)), diffByteLimit)
			if err != nil {
				if strings.Contains(err.Error(), "exceeds") {
					diffs = append(diffs, FileDiff{Path: action.Path, Kind: string(action.Kind), Note: "diff skipped: content exceeds 1 MiB limit"})
					continue
				}
				return nil, fmt.Errorf("plan diff: read current %q: %w", action.Path, err)
			}
			if !exists {
				return nil, fmt.Errorf("plan diff: read current %q: %w", action.Path, os.ErrNotExist)
			}
			oldContent = data
		}
		diff := FileDiff{
			Path: action.Path,
			Kind: string(action.Kind),
		}
		if len(oldContent) > diffByteLimit || len(newContent) > diffByteLimit {
			diff.Note = "diff skipped: content exceeds 1 MiB limit"
			diffs = append(diffs, diff)
			continue
		}
		if !utf8.Valid(oldContent) || !utf8.Valid(newContent) {
			diff.Note = "diff skipped: binary content"
			diffs = append(diffs, diff)
			continue
		}
		oldStr := string(oldContent)
		newStr := string(newContent)
		oldLines := splitDiffLines(oldStr)
		newLines := splitDiffLines(newStr)
		if !diffWithinBudget(oldLines, newLines) {
			diff.Note = fmt.Sprintf("diff skipped: changed region exceeds the %d-cell diff budget", diffCellLimit)
			diffs = append(diffs, diff)
			continue
		}
		if action.Kind == ActionCreate {
			diff.NewLines = newLines
		} else {
			diff.OldLines = oldLines
			diff.NewLines = newLines
		}
		diff.Unified = formatUnifiedDiff(action.Path, oldStr, newStr)
		diffs = append(diffs, diff)
	}
	return diffs, nil
}

// splitDiffLines splits content into lines for the structured FileDiff
// projection. A trailing newline does not produce a phantom empty line.
func splitDiffLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// editKind classifies one line in a diff edit script.
type editKind int

const (
	editContext editKind = iota
	editDelete
	editInsert
)

type edit struct {
	kind editKind
	text string
}

// trimCommon returns the number of identical leading and trailing lines shared
// by both inputs. The two counts never overlap, so the remaining middle
// regions oldLines[prefix:len-suffix] and newLines[prefix:len-suffix] are
// well-formed even when one input is a prefix of the other.
func trimCommon(oldLines, newLines []string) (prefix, suffix int) {
	limit := min(len(oldLines), len(newLines))
	for prefix < limit && oldLines[prefix] == newLines[prefix] {
		prefix++
	}
	limit -= prefix
	for suffix < limit && oldLines[len(oldLines)-1-suffix] == newLines[len(newLines)-1-suffix] {
		suffix++
	}
	return prefix, suffix
}

// diffWithinBudget reports whether the changed region, after trimming the
// common prefix and suffix, fits the LCS table budget.
func diffWithinBudget(oldLines, newLines []string) bool {
	prefix, suffix := trimCommon(oldLines, newLines)
	m := len(oldLines) - prefix - suffix
	n := len(newLines) - prefix - suffix
	return (m+1)*(n+1) <= diffCellLimit
}

// computeEdits produces the line-level edit script transforming oldLines
// into newLines. Identical leading and trailing lines are emitted as context
// without entering the dynamic program; the remaining region is solved with a
// longest-common-subsequence table stored as a flat []int32. When even the
// trimmed region exceeds diffCellLimit the function degrades to a full
// replacement script rather than allocating an unbounded table; PlanDiff
// checks the budget first and skips such files with a note.
func computeEdits(oldLines, newLines []string) []edit {
	prefix, suffix := trimCommon(oldLines, newLines)
	oldMid := oldLines[prefix : len(oldLines)-suffix]
	newMid := newLines[prefix : len(newLines)-suffix]
	edits := make([]edit, 0, len(oldLines)+len(newLines)-prefix-suffix)
	for _, line := range oldLines[:prefix] {
		edits = append(edits, edit{editContext, line})
	}
	edits = append(edits, computeMiddleEdits(oldMid, newMid)...)
	for _, line := range oldLines[len(oldLines)-suffix:] {
		edits = append(edits, edit{editContext, line})
	}
	return edits
}

func computeMiddleEdits(oldLines, newLines []string) []edit {
	m, n := len(oldLines), len(newLines)
	edits := make([]edit, 0, m+n)
	if m == 0 || n == 0 || (m+1)*(n+1) > diffCellLimit {
		for _, line := range oldLines {
			edits = append(edits, edit{editDelete, line})
		}
		for _, line := range newLines {
			edits = append(edits, edit{editInsert, line})
		}
		return edits
	}
	// Flat (m+1)x(n+1) LCS length table, bottom-up; cell(i,j) = dp[i*(n+1)+j].
	width := n + 1
	dp := make([]int32, (m+1)*width)
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			switch {
			case oldLines[i] == newLines[j]:
				dp[i*width+j] = dp[(i+1)*width+j+1] + 1
			case dp[(i+1)*width+j] >= dp[i*width+j+1]:
				dp[i*width+j] = dp[(i+1)*width+j]
			default:
				dp[i*width+j] = dp[i*width+j+1]
			}
		}
	}
	i, j := 0, 0
	for i < m && j < n {
		switch {
		case oldLines[i] == newLines[j]:
			edits = append(edits, edit{editContext, oldLines[i]})
			i++
			j++
		case dp[(i+1)*width+j] >= dp[i*width+j+1]:
			edits = append(edits, edit{editDelete, oldLines[i]})
			i++
		default:
			edits = append(edits, edit{editInsert, newLines[j]})
			j++
		}
	}
	for ; i < m; i++ {
		edits = append(edits, edit{editDelete, oldLines[i]})
	}
	for ; j < n; j++ {
		edits = append(edits, edit{editInsert, newLines[j]})
	}
	return edits
}

// formatUnifiedDiff renders a standard unified diff string for one file,
// with three lines of context around each change hunk.
func formatUnifiedDiff(path, oldContent, newContent string) string {
	oldLines := splitDiffLines(oldContent)
	newLines := splitDiffLines(newContent)
	oldNoNL := len(oldContent) > 0 && !strings.HasSuffix(oldContent, "\n")
	newNoNL := len(newContent) > 0 && !strings.HasSuffix(newContent, "\n")

	edits := computeEdits(oldLines, newLines)

	// Identify change regions (runs of non-context edits).
	type region struct{ start, end int }
	var regions []region
	inChange := false
	for i, e := range edits {
		if e.kind != editContext {
			if !inChange {
				regions = append(regions, region{start: i})
				inChange = true
			}
			regions[len(regions)-1].end = i
		} else {
			inChange = false
		}
	}
	if len(regions) == 0 {
		return ""
	}

	// Expand regions with context and merge overlapping hunks.
	const contextLines = 3
	type hunkRange struct{ start, end int }
	var hunks []hunkRange
	for _, r := range regions {
		start := r.start - contextLines
		if start < 0 {
			start = 0
		}
		end := r.end + contextLines
		if end >= len(edits) {
			end = len(edits) - 1
		}
		if len(hunks) > 0 && start <= hunks[len(hunks)-1].end+1 {
			hunks[len(hunks)-1].end = end
		} else {
			hunks = append(hunks, hunkRange{start, end})
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "--- a/%s\n", path)
	fmt.Fprintf(&b, "+++ b/%s\n", path)

	for _, h := range hunks {
		// Calculate the 1-based starting line numbers by counting
		// context and delete/insert lines before this hunk.
		oldStart, newStart := 1, 1
		for i := 0; i < h.start; i++ {
			switch edits[i].kind {
			case editContext:
				oldStart++
				newStart++
			case editDelete:
				oldStart++
			case editInsert:
				newStart++
			}
		}

		oldCount, newCount := 0, 0
		type outLine struct {
			prefix byte
			text   string
			isOld  bool // touches old file lines
			isNew  bool // touches new file lines
		}
		var lines []outLine
		for i := h.start; i <= h.end; i++ {
			e := edits[i]
			switch e.kind {
			case editContext:
				lines = append(lines, outLine{' ', e.text, true, true})
				oldCount++
				newCount++
			case editDelete:
				lines = append(lines, outLine{'-', e.text, true, false})
				oldCount++
			case editInsert:
				lines = append(lines, outLine{'+', e.text, false, true})
				newCount++
			}
		}

		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)

		// Track whether we are at the last line of old/new to emit the
		// "\ No newline at end of file" marker correctly.
		oldPos, newPos := oldStart, newStart
		for _, line := range lines {
			fmt.Fprintf(&b, "%c%s\n", line.prefix, line.text)
			lastOld := line.isOld && oldPos == len(oldLines)
			lastNew := line.isNew && newPos == len(newLines)
			switch line.prefix {
			case ' ':
				if lastOld && oldNoNL && lastNew && newNoNL {
					b.WriteString("\\ No newline at end of file\n")
				} else if lastOld && oldNoNL && !newNoNL {
					b.WriteString("\\ No newline at end of file\n")
				}
				oldPos++
				newPos++
			case '-':
				if lastOld && oldNoNL {
					b.WriteString("\\ No newline at end of file\n")
				}
				oldPos++
			default:
				if lastNew && newNoNL {
					b.WriteString("\\ No newline at end of file\n")
				}
				newPos++
			}
		}
	}
	return b.String()
}
