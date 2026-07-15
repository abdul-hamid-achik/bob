package mcp

import (
	stdcontext "context"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	contextpkg "github.com/abdul-hamid-achik/bob/internal/context"
	"github.com/abdul-hamid-achik/bob/internal/guidance"
	"github.com/abdul-hamid-achik/bob/internal/pathinfo"
	"github.com/abdul-hamid-achik/bob/internal/playbook"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxGuidanceRequestBytes = 64 << 10

const (
	maxGuidancePathBytes          = 4 << 10
	maxGuidancePlaybookIDBytes    = 128
	maxGuidancePlaybookInputCount = 32
	maxGuidanceInputKeyBytes      = 128
	maxGuidanceInputValueBytes    = 4 << 10
)

type ContextInput struct {
	Workspace string `json:"workspace,omitempty" jsonschema:"existing repository directory authorized at server startup; defaults to the startup workspace"`
	Profile   string `json:"profile,omitempty" jsonschema:"bounded context profile: compact, standard, or full; defaults to compact"`
}

type PathInput struct {
	Workspace string `json:"workspace,omitempty" jsonschema:"existing repository directory authorized at server startup; defaults to the startup workspace"`
	Path      string `json:"path" jsonschema:"repository-relative UTF-8 path containing at most 4096 bytes"`
}

type PlaybookInput struct {
	Workspace string            `json:"workspace,omitempty" jsonschema:"existing repository directory authorized at server startup; defaults to the startup workspace"`
	Operation string            `json:"operation" jsonschema:"closed operation: list, show, or plan"`
	ID        string            `json:"id,omitempty" jsonschema:"stable playbook ID containing at most 128 bytes; required for show and plan"`
	Values    map[string]string `json:"values,omitempty" jsonschema:"closed typed input values for plan; at most 32 entries, 128-byte keys, and 4096-byte values"`
}

type ContextOutput struct {
	SchemaVersion int                `json:"schema_version"`
	OK            bool               `json:"ok"`
	Authority     AuthorityInfo      `json:"authority"`
	Context       *contextpkg.Result `json:"context,omitempty"`
	Error         *ErrorInfo         `json:"error,omitempty"`
}

type PathOutput struct {
	SchemaVersion int              `json:"schema_version"`
	OK            bool             `json:"ok"`
	Authority     AuthorityInfo    `json:"authority"`
	Path          *pathinfo.Result `json:"path,omitempty"`
	Error         *ErrorInfo       `json:"error,omitempty"`
}

type PlaybookOutput struct {
	SchemaVersion int                  `json:"schema_version"`
	OK            bool                 `json:"ok"`
	Authority     AuthorityInfo        `json:"authority"`
	Operation     string               `json:"operation,omitempty"`
	List          *playbook.ListResult `json:"list,omitempty"`
	Show          *playbook.ShowResult `json:"show,omitempty"`
	Plan          *playbook.Guide      `json:"plan,omitempty"`
	Error         *ErrorInfo           `json:"error,omitempty"`
}

func (s *Server) handleContext(_ stdcontext.Context, _ *sdkmcp.CallToolRequest, in ContextInput) (*sdkmcp.CallToolResult, *ContextOutput, error) {
	if err := validateContextInput(in); err != nil {
		return s.contextFailure("", "input_invalid", err)
	}
	profile := contextpkg.Profile(in.Profile)
	if profile == "" {
		profile = contextpkg.ProfileCompact
	}
	if profile != contextpkg.ProfileCompact && profile != contextpkg.ProfileStandard && profile != contextpkg.ProfileFull {
		return s.contextFailure("", "input_invalid", fmt.Errorf("profile must be one of compact, standard, full (got %q)", in.Profile))
	}
	root, authErr := s.authority.resolve(in.Workspace)
	if authErr != nil {
		return s.contextFailure("", authErr.code, authErr)
	}
	result, err := contextpkg.Load(root, contextpkg.Options{Profile: profile, LookPath: s.lookPath})
	if err != nil {
		return s.contextFailure(root, guidanceErrorCode(err, "context_failed"), err)
	}
	out := &ContextOutput{SchemaVersion: toolSchemaVersion, OK: true, Authority: s.authority.info(root), Context: &result}
	return &sdkmcp.CallToolResult{Content: contextTextContent(out)}, out, nil
}

func (s *Server) handlePath(_ stdcontext.Context, _ *sdkmcp.CallToolRequest, in PathInput) (*sdkmcp.CallToolResult, *PathOutput, error) {
	if err := validatePathInput(in); err != nil {
		return s.pathFailure("", "input_invalid", err)
	}
	root, authErr := s.authority.resolve(in.Workspace)
	if authErr != nil {
		return s.pathFailure("", authErr.code, authErr)
	}
	result, err := pathinfo.Load(root, in.Path)
	if err != nil {
		return s.pathFailure(root, guidanceErrorCode(err, "path_failed"), err)
	}
	out := &PathOutput{SchemaVersion: toolSchemaVersion, OK: true, Authority: s.authority.info(root), Path: &result}
	return &sdkmcp.CallToolResult{}, out, nil
}

func (s *Server) handlePlaybook(_ stdcontext.Context, _ *sdkmcp.CallToolRequest, in PlaybookInput) (*sdkmcp.CallToolResult, *PlaybookOutput, error) {
	if err := validatePlaybookInput(in); err != nil {
		return s.playbookFailure("", in.Operation, "input_invalid", err)
	}
	if in.Operation != "list" && in.Operation != "show" && in.Operation != "plan" {
		return s.playbookFailure("", in.Operation, "input_invalid", errors.New("operation must be one of list, show, plan"))
	}
	if in.Operation == "list" && (in.ID != "" || len(in.Values) > 0) {
		return s.playbookFailure("", in.Operation, "input_invalid", errors.New("list does not accept id or values"))
	}
	if (in.Operation == "show" || in.Operation == "plan") && in.ID == "" {
		return s.playbookFailure("", in.Operation, "input_invalid", errors.New("id is required for show and plan"))
	}
	if in.Operation == "show" && len(in.Values) > 0 {
		return s.playbookFailure("", in.Operation, "input_invalid", errors.New("show does not accept values"))
	}
	root, authErr := s.authority.resolve(in.Workspace)
	if authErr != nil {
		return s.playbookFailure("", in.Operation, authErr.code, authErr)
	}
	out := &PlaybookOutput{SchemaVersion: toolSchemaVersion, OK: true, Authority: s.authority.info(root), Operation: in.Operation}
	switch in.Operation {
	case "list":
		result, err := playbook.List(root)
		if err != nil {
			return s.playbookFailure(root, in.Operation, guidanceErrorCode(err, "playbook_failed"), err)
		}
		out.List = &result
	case "show":
		result, err := playbook.ShowWithOptions(root, in.ID, playbook.Options{LookPath: s.lookPath})
		if err != nil {
			return s.playbookFailure(root, in.Operation, guidanceErrorCode(err, "playbook_failed"), err)
		}
		out.Show = &result
	case "plan":
		result, err := playbook.PlanWithOptions(root, in.ID, in.Values, playbook.Options{LookPath: s.lookPath})
		if err != nil {
			return s.playbookFailure(root, in.Operation, guidanceErrorCode(err, "playbook_failed"), err)
		}
		out.Plan = &result
	}
	return &sdkmcp.CallToolResult{}, out, nil
}

func validateGuidanceRequest(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	if len(data) > maxGuidanceRequestBytes {
		return fmt.Errorf("request exceeds the %d-byte input limit", maxGuidanceRequestBytes)
	}
	return nil
}

func validateContextInput(in ContextInput) error {
	if !utf8.ValidString(in.Workspace) {
		return errors.New("workspace must contain valid UTF-8")
	}
	return validateGuidanceRequest(in)
}

func validatePathInput(in PathInput) error {
	if !utf8.ValidString(in.Workspace) || !utf8.ValidString(in.Path) {
		return errors.New("workspace and path must contain valid UTF-8")
	}
	if len(in.Path) == 0 || len(in.Path) > maxGuidancePathBytes {
		return fmt.Errorf("path must contain 1 to %d bytes", maxGuidancePathBytes)
	}
	return validateGuidanceRequest(in)
}

func validatePlaybookInput(in PlaybookInput) error {
	if !utf8.ValidString(in.Workspace) || !utf8.ValidString(in.Operation) || !utf8.ValidString(in.ID) {
		return errors.New("workspace, operation, and id must contain valid UTF-8")
	}
	if len(in.ID) > maxGuidancePlaybookIDBytes {
		return fmt.Errorf("playbook id must contain at most %d bytes", maxGuidancePlaybookIDBytes)
	}
	if len(in.Values) > maxGuidancePlaybookInputCount {
		return fmt.Errorf("playbook accepts at most %d input values", maxGuidancePlaybookInputCount)
	}
	for key, value := range in.Values {
		if !utf8.ValidString(key) || !utf8.ValidString(value) {
			return errors.New("playbook input keys and values must contain valid UTF-8")
		}
		if len(key) == 0 || len(key) > maxGuidanceInputKeyBytes {
			return fmt.Errorf("playbook input keys must contain 1 to %d bytes", maxGuidanceInputKeyBytes)
		}
		if len(value) > maxGuidanceInputValueBytes {
			return fmt.Errorf("playbook input values must contain at most %d bytes", maxGuidanceInputValueBytes)
		}
	}
	return validateGuidanceRequest(in)
}

func guidanceErrorCode(err error, fallback string) string {
	if code, ok := guidance.ErrorCode(err); ok {
		return code
	}
	return fallback
}

// contextTextContent avoids serializing the complete compact contract twice in
// one MCP result. structuredContent remains the exact typed contract; the text
// block is a deterministic identity/state projection for text-only clients.
func contextTextContent(out *ContextOutput) []sdkmcp.Content {
	type textContext struct {
		SchemaVersion  int                      `json:"schema_version"`
		Profile        contextpkg.Profile       `json:"profile"`
		Workspace      string                   `json:"workspace"`
		ContractDigest string                   `json:"contract_digest"`
		ContextDigest  string                   `json:"context_digest"`
		Recipe         recipe.MetadataRecipeRef `json:"recipe"`
		Repository     contextpkg.Repository    `json:"repository"`
		Truncation     contextpkg.Truncation    `json:"truncation"`
		Detail         string                   `json:"detail_location"`
	}
	type textOutput struct {
		SchemaVersion int           `json:"schema_version"`
		OK            bool          `json:"ok"`
		Authority     AuthorityInfo `json:"authority"`
		Context       textContext   `json:"context"`
	}
	projection := textOutput{SchemaVersion: out.SchemaVersion, OK: out.OK, Authority: out.Authority}
	if out.Context != nil {
		projection.Context = textContext{
			SchemaVersion: out.Context.SchemaVersion, Profile: out.Context.Profile, Workspace: out.Context.Workspace,
			ContractDigest: out.Context.ContractDigest, ContextDigest: out.Context.ContextDigest,
			Recipe: out.Context.Recipe, Repository: out.Context.Repository, Truncation: out.Context.Truncation,
			Detail: "structuredContent",
		}
	}
	data, err := json.Marshal(projection)
	if err != nil {
		return nil
	}
	return []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}}
}

func (s *Server) contextFailure(root, code string, err error) (*sdkmcp.CallToolResult, *ContextOutput, error) {
	out := &ContextOutput{SchemaVersion: toolSchemaVersion, Authority: s.authority.info(root), Error: &ErrorInfo{Code: code, Message: err.Error()}}
	return &sdkmcp.CallToolResult{IsError: true}, out, nil
}

func (s *Server) pathFailure(root, code string, err error) (*sdkmcp.CallToolResult, *PathOutput, error) {
	out := &PathOutput{SchemaVersion: toolSchemaVersion, Authority: s.authority.info(root), Error: &ErrorInfo{Code: code, Message: err.Error()}}
	return &sdkmcp.CallToolResult{IsError: true}, out, nil
}

func (s *Server) playbookFailure(root, operation, code string, err error) (*sdkmcp.CallToolResult, *PlaybookOutput, error) {
	out := &PlaybookOutput{SchemaVersion: toolSchemaVersion, Authority: s.authority.info(root), Operation: operation, Error: &ErrorInfo{Code: code, Message: err.Error()}}
	return &sdkmcp.CallToolResult{IsError: true}, out, nil
}
