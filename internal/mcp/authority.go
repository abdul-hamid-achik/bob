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

// applyAuthority is the separate, empty-by-default mutation allowlist for
// bob_apply. It is never implied by the read allowlist or by allowAny: only
// workspaces named explicitly with --allow-apply-workspace may be mutated.
type applyAuthority struct {
	allowed map[string]struct{}
}

// ApplyAuthorityInfo discloses the mutation authority without listing paths.
type ApplyAuthorityInfo struct {
	Mode                  string `json:"mode"`
	AllowedWorkspaceCount int    `json:"allowed_workspace_count"`
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
		canonical, err := canonicalAllowedWorkspace(defaultWorkspace, candidate)
		if err != nil {
			return workspaceAuthority{}, fmt.Errorf("resolve allowed workspace: %w", err)
		}
		authority.allowed[canonical] = struct{}{}
	}
	return authority, nil
}

// newApplyAuthority resolves the explicit mutation allowlist. An empty list
// disables bob_apply entirely. Each apply workspace is also added to the read
// allowlist, because a caller must be able to review the plan (bob_plan or
// bob_check) whose digest it then applies; the reverse is never true.
func newApplyAuthority(read *workspaceAuthority, candidates []string) (applyAuthority, error) {
	authority := applyAuthority{allowed: map[string]struct{}{}}
	for _, candidate := range candidates {
		canonical, err := canonicalAllowedWorkspace(read.defaultWorkspace, candidate)
		if err != nil {
			return applyAuthority{}, fmt.Errorf("resolve apply workspace: %w", err)
		}
		authority.allowed[canonical] = struct{}{}
		read.allowed[canonical] = struct{}{}
	}
	return authority, nil
}

func canonicalAllowedWorkspace(defaultWorkspace, candidate string) (string, error) {
	if strings.TrimSpace(candidate) == "" {
		return "", fmt.Errorf("path is required")
	}
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(defaultWorkspace, candidate)
	}
	return workspace.Resolve(candidate, true)
}

// resolveApply canonicalizes the requested workspace exactly like the read
// tools and then requires membership in the explicit apply allowlist. The
// read allowlist and --allow-any-workspace never grant mutation authority.
func (a workspaceAuthority) resolveApply(apply applyAuthority, input string) (string, *authorityError) {
	canonical, authErr := a.canonicalize(input)
	if authErr != nil {
		return "", authErr
	}
	if _, allowed := apply.allowed[canonical]; !allowed {
		message := "workspace is outside the MCP server's apply allowlist"
		if len(apply.allowed) == 0 {
			message = "bob_apply is disabled: start bob mcp serve with --allow-apply-workspace to authorize mutation"
		}
		return "", &authorityError{code: "apply_unauthorized", message: message}
	}
	return canonical, nil
}

func (a workspaceAuthority) canonicalize(input string) (string, *authorityError) {
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
	return canonical, nil
}

func (a workspaceAuthority) resolve(input string) (string, *authorityError) {
	canonical, authErr := a.canonicalize(input)
	if authErr != nil {
		return "", authErr
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

func (a applyAuthority) info() ApplyAuthorityInfo {
	mode := "disabled"
	if len(a.allowed) > 0 {
		mode = "exact_allowlist"
	}
	return ApplyAuthorityInfo{Mode: mode, AllowedWorkspaceCount: len(a.allowed)}
}
