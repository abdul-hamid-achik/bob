package recipe

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/strsim"
)

// Artifact is one deterministic file produced by a recipe.
type Artifact struct {
	Path    string      `json:"path"`
	Mode    fs.FileMode `json:"mode"`
	Content []byte      `json:"-"`
}

// goAgentToolRecipeVersion is the current go-agent-tool recipe contract
// version. engine.RecipeVersion mirrors this value as a deprecated
// compatibility alias; recipe.Version is the source of truth.
const goAgentToolRecipeVersion = 4

// Version returns the current contract version for a built-in recipe id.
func Version(recipeID string) (int, error) {
	switch recipeID {
	case "go-agent-tool":
		return goAgentToolRecipeVersion, nil
	case "files":
		return FilesRecipeVersion, nil
	default:
		return 0, fmt.Errorf("unsupported recipe %q%s", recipeID, didYouMeanRecipe(recipeID))
	}
}

// IDs returns the sorted set of built-in recipe identifiers.
func IDs() []string {
	return []string{"files", "go-agent-tool"}
}

// didYouMeanRecipe returns a ready-to-append "; did you mean ...?" suffix
// when id is a close typo of a known recipe id, or "" otherwise. It keeps
// unsupported-recipe errors self-recoverable for a weak model without an
// extra round trip to `bob recipe list`.
func didYouMeanRecipe(id string) string {
	if suggestion, ok := strsim.Closest(id, IDs(), 2); ok {
		return fmt.Sprintf("; did you mean %q?", suggestion)
	}
	return ""
}

// Render compiles a manifest into the complete set of Bob-owned artifacts.
func Render(m manifest.Manifest) ([]Artifact, error) {
	version, err := Version(m.Recipe)
	if err != nil {
		return nil, err
	}
	return RenderVersion(m, version)
}

// RenderVersion reproduces one supported immutable built-in recipe contract.
// Normal callers should use Render, which always selects the current version.
// Keeping the immediately previous go-agent-tool contract renderable lets the
// migration suite prove byte-for-byte compatibility and safe lock upgrades.
func RenderVersion(m manifest.Manifest, version int) ([]Artifact, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	var artifacts []Artifact
	var err error
	switch m.Recipe {
	case "go-agent-tool":
		if version != 3 && version != goAgentToolRecipeVersion {
			return nil, fmt.Errorf("unsupported go-agent-tool recipe version %d", version)
		}
		artifacts, err = renderGoAgentTool(m, version)
	case "files":
		if version != FilesRecipeVersion {
			return nil, fmt.Errorf("unsupported files recipe version %d", version)
		}
		artifacts, err = renderFiles(m)
	default:
		return nil, fmt.Errorf("unsupported recipe %q%s", m.Recipe, didYouMeanRecipe(m.Recipe))
	}
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(artifacts))
	for i := range artifacts {
		path, err := safePath(artifacts[i].Path)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[path]; exists {
			return nil, fmt.Errorf("recipe produced duplicate path %q", path)
		}
		seen[path] = struct{}{}
		artifacts[i].Path = path
		if artifacts[i].Mode == 0 {
			artifacts[i].Mode = 0o644
		}
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, nil
}

func safePath(path string) (string, error) {
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "." || filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, "../") {
		return "", fmt.Errorf("recipe produced unsafe path %q", path)
	}
	if path == ".git" || strings.HasPrefix(path, ".git/") || path == manifest.Filename || path == "bob.lock" {
		return "", fmt.Errorf("recipe cannot own reserved path %q", path)
	}
	return path, nil
}
