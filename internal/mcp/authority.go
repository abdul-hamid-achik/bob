package mcp

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/abdul-hamid-achik/bob/internal/workspace"
)

const maxWorkspaceInputBytes = 4096

type workspaceAuthority struct {
	defaultWorkspace string
	allowed          map[string]struct{}
	allowAny         bool
}

type AuthorityInfo struct {
	Mode                  string `json:"mode"`
	DefaultWorkspace      string `json:"default_workspace"`
	SelectedWorkspace     string `json:"selected_workspace,omitempty"`
	AllowedWorkspaceCount int    `json:"allowed_workspace_count"`
}

type authorityError struct {
	code    string
	message string
}

func (e *authorityError) Error() string { return e.message }

func newWorkspaceAuthority(defaultWorkspace string, additional []string, allowAny bool) (workspaceAuthority, error) {
	authority := workspaceAuthority{
		defaultWorkspace: defaultWorkspace,
		allowed:          map[string]struct{}{defaultWorkspace: {}},
		allowAny:         allowAny,
	}
	for _, candidate := range additional {
		if strings.TrimSpace(candidate) == "" {
			return workspaceAuthority{}, fmt.Errorf("resolve allowed workspace: path is required")
		}
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(defaultWorkspace, candidate)
		}
		canonical, err := workspace.Resolve(candidate, true)
		if err != nil {
			return workspaceAuthority{}, fmt.Errorf("resolve allowed workspace: %w", err)
		}
		authority.allowed[canonical] = struct{}{}
	}
	return authority, nil
}

func (a workspaceAuthority) resolve(input string) (string, *authorityError) {
	if len(input) > maxWorkspaceInputBytes {
		return "", &authorityError{code: "input_invalid", message: "workspace path exceeds the 4096-byte input limit"}
	}
	candidate := input
	if candidate == "" {
		candidate = a.defaultWorkspace
	} else if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(a.defaultWorkspace, candidate)
	}
	canonical, err := workspace.Resolve(candidate, true)
	if err != nil {
		return "", &authorityError{code: "workspace_invalid", message: err.Error()}
	}
	if !a.allowAny {
		if _, allowed := a.allowed[canonical]; !allowed {
			return "", &authorityError{
				code:    "workspace_unauthorized",
				message: "workspace is outside the MCP server's exact allowlist",
			}
		}
	}
	return canonical, nil
}

func (a workspaceAuthority) info(selected string) AuthorityInfo {
	mode := "exact_allowlist"
	if a.allowAny {
		mode = "any_workspace"
	}
	return AuthorityInfo{
		Mode: mode, DefaultWorkspace: a.defaultWorkspace, SelectedWorkspace: selected,
		AllowedWorkspaceCount: len(a.allowed),
	}
}
