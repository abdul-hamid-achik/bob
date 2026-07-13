package recipe

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

// FilesRecipeVersion is the current files recipe contract version.
const FilesRecipeVersion = 1

// filesVarPattern matches exactly ${vars.<key>} where key is a declared-shape
// variable name. This is a single literal-replacement regex pass, not a
// template language: text that does not match (a shell's own ${FOO}, for
// example) passes through untouched.
var filesVarPattern = regexp.MustCompile(`\$\{vars\.([a-z][a-z0-9_]*)\}`)

// renderFiles materializes the inline file tree declared by a files-recipe
// manifest. Files are produced in declared order; Render performs the
// central path-safety check, duplicate detection, and final sort.
func renderFiles(m manifest.Manifest) ([]Artifact, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("render files: %w", err)
	}

	artifacts := make([]Artifact, 0, len(m.Files))
	var unresolved []string
	for _, decl := range m.Files {
		mode, err := manifest.ParseFileMode(decl.Mode)
		if err != nil {
			return nil, fmt.Errorf("render files: %s: %w", decl.Path, err)
		}
		content, missing := substituteVars(decl.Content, m.Vars)
		for _, key := range missing {
			unresolved = append(unresolved, fmt.Sprintf("%s: ${vars.%s}", decl.Path, key))
		}
		artifacts = append(artifacts, Artifact{Path: decl.Path, Mode: mode, Content: []byte(content)})
	}
	if len(unresolved) > 0 {
		sort.Strings(unresolved)
		unresolved = dedupeSorted(unresolved)
		return nil, fmt.Errorf("render files: unresolved variable reference(s): %s", strings.Join(unresolved, "; "))
	}
	return artifacts, nil
}

// substituteVars performs the one regex pass over content, replacing every
// ${vars.<key>} occurrence with its declared value. References to a key not
// present in vars are left untouched in the output and reported back so the
// caller can collect every unresolved reference across every file.
func substituteVars(content string, vars map[string]string) (string, []string) {
	var missing []string
	result := filesVarPattern.ReplaceAllStringFunc(content, func(match string) string {
		submatch := filesVarPattern.FindStringSubmatch(match)
		key := submatch[1]
		value, ok := vars[key]
		if !ok {
			missing = append(missing, key)
			return match
		}
		return value
	})
	return result, missing
}

// dedupeSorted removes adjacent duplicates from an already-sorted slice.
func dedupeSorted(values []string) []string {
	if len(values) == 0 {
		return values
	}
	out := values[:1]
	for _, v := range values[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}
