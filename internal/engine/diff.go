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
// large generated artifact never forces an unbounded LCS table allocation.
const diffByteLimit = 1 << 20 // 1 MiB

// diffLineLimit caps the line count PlanDiff will attempt to diff. The
// O(m*n) LCS table makes very line-dense files expensive even when they are
// under diffByteLimit.
const diffLineLimit = 8192

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
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(action.Path)))
			if err != nil {
				return nil, fmt.Errorf("plan diff: read current %q: %w", action.Path, err)
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
		if len(oldLines) > diffLineLimit || len(newLines) > diffLineLimit {
			diff.Note = fmt.Sprintf("diff skipped: content exceeds %d line limit", diffLineLimit)
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

// computeEdits produces the line-level edit script transforming oldLines
// into newLines using a longest-common-subsequence dynamic program.
func computeEdits(oldLines, newLines []string) []edit {
	m, n := len(oldLines), len(newLines)
	// Build the LCS length table bottom-up.
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	// Walk the table to produce the edit script.
	edits := make([]edit, 0, m+n)
	i, j := 0, 0
	for i < m && j < n {
		switch {
		case oldLines[i] == newLines[j]:
			edits = append(edits, edit{editContext, oldLines[i]})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
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
