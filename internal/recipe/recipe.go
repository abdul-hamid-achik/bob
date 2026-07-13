package recipe

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

// Artifact is one deterministic file produced by a recipe.
type Artifact struct {
	Path    string      `json:"path"`
	Mode    fs.FileMode `json:"mode"`
	Content []byte      `json:"-"`
}

// Render compiles a manifest into the complete set of Bob-owned artifacts.
func Render(m manifest.Manifest) ([]Artifact, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if m.Recipe != "go-agent-tool" {
		return nil, fmt.Errorf("unsupported recipe %q", m.Recipe)
	}
	artifacts, err := renderGoAgentTool(m)
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
