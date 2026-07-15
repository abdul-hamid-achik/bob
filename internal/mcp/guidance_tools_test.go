package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGuidanceToolsProjectSharedReadOnlyServices(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	before := fileSnapshot(t, root)
	runner := &offlineRunner{}
	session := connect(t, root, runner)

	contextResult, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_context", Arguments: map[string]any{}})
	if err != nil || contextResult.IsError {
		t.Fatalf("context: result=%#v err=%v", contextResult, err)
	}
	var contextOutput ContextOutput
	decodeStructured(t, contextResult, &contextOutput)
	if !contextOutput.OK || contextOutput.Context == nil || contextOutput.Context.Profile != "compact" || contextOutput.Context.Recipe.ID != "go-agent-tool" {
		t.Fatalf("context output = %#v", contextOutput)
	}
	contextBytes, _ := json.Marshal(contextOutput)
	if len(contextBytes) >= 8<<10 {
		t.Fatalf("compact MCP context = %d bytes", len(contextBytes))
	}
	contextResultBytes := assertCallToolResponseBound(t, "compact context", contextResult, 8<<10)
	assertContextTextProjection(t, contextResult, contextOutput)
	t.Logf("compact MCP context: output=%d call_tool_response=%d", len(contextBytes), contextResultBytes)

	pathResult, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_path", Arguments: map[string]any{"path": "internal/cli/root.go"}})
	if err != nil || pathResult.IsError {
		t.Fatalf("path: result=%#v err=%v", pathResult, err)
	}
	var pathOutput PathOutput
	decodeStructuredExact(t, pathResult, &pathOutput)
	if !pathOutput.OK || pathOutput.Path == nil || pathOutput.Path.Classification != "managed" || pathOutput.Path.State != "managed_missing" {
		t.Fatalf("path output = %#v", pathOutput)
	}
	pathBytes, _ := json.Marshal(pathOutput)
	if len(pathBytes) >= 8<<10 {
		t.Fatalf("MCP path = %d bytes", len(pathBytes))
	}
	assertCallToolResponseBound(t, "path", pathResult, 8<<10)

	playbookResult, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_playbook", Arguments: map[string]any{"operation": "list"}})
	if err != nil || playbookResult.IsError {
		t.Fatalf("playbook: result=%#v err=%v", playbookResult, err)
	}
	var playbookOutput PlaybookOutput
	decodeStructuredExact(t, playbookResult, &playbookOutput)
	if !playbookOutput.OK || playbookOutput.List == nil || len(playbookOutput.List.Playbooks) == 0 {
		t.Fatalf("playbook output = %#v", playbookOutput)
	}
	playbookBytes, _ := json.Marshal(playbookOutput)
	if len(playbookBytes) >= 8<<10 {
		t.Fatalf("MCP playbook list = %d bytes", len(playbookBytes))
	}
	assertCallToolResponseBound(t, "playbook list", playbookResult, 8<<10)
	for _, call := range []struct {
		operation string
		arguments map[string]any
	}{
		{operation: "show", arguments: map[string]any{"operation": "show", "id": "add-cli-command"}},
		{operation: "plan", arguments: map[string]any{"operation": "plan", "id": "add-cli-command", "values": map[string]string{"command_name": "hello"}}},
	} {
		result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_playbook", Arguments: call.arguments})
		if err != nil || result.IsError {
			t.Fatalf("playbook %s: result=%#v err=%v", call.operation, result, err)
		}
		var output PlaybookOutput
		decodeStructuredExact(t, result, &output)
		data, _ := json.Marshal(output)
		if len(data) >= 24<<10 {
			t.Fatalf("MCP playbook %s = %d bytes", call.operation, len(data))
		}
		assertCallToolResponseBound(t, "playbook "+call.operation, result, 24<<10)
	}

	assertSnapshotEqual(t, before, fileSnapshot(t, root))
	if runner.calls() != 0 {
		t.Fatalf("guidance tools invoked specialist runner %d time(s)", runner.calls())
	}
}

func TestCompactContextMaximumRecipeMCPWireBudget(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	m.Integrations.BrowserVerification = "cairntrace"
	m.Integrations.Secrets = "tinyvault"
	m.Integrations.Artifacts = "fcheap"
	m.Distribution.Homebrew = true
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, true); err != nil {
		t.Fatal(err)
	}
	session := connectWithOptions(t, root, &offlineRunner{}, ServerOptions{lookPath: func(string) (string, error) {
		return "", errors.New("not found")
	}})
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_context", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("context: result=%#v err=%v", result, err)
	}
	var output ContextOutput
	decodeStructured(t, result, &output)
	if output.Context == nil || len(output.Context.Capabilities) != 14 || output.Context.Profile != "compact" {
		t.Fatalf("maximum compact context = %#v", output.Context)
	}
	data, err := json.Marshal(output.Context)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 6144 {
		t.Fatalf("maximum compact context data = %d bytes", len(data))
	}
	wireBytes := assertCallToolResponseBound(t, "maximum compact context", result, 8<<10)
	t.Logf("maximum compact MCP context: data=%d call_tool_response=%d", len(data), wireBytes)
}

func TestGuidanceToolsRejectBoundsEnumsAndUnauthorizedWorkspaces(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	session := connect(t, root, &offlineRunner{})
	unauthorizedRoot := mcpWorkspace(t)
	tooManyValues := make(map[string]string, maxGuidancePlaybookInputCount+1)
	for i := 0; i <= maxGuidancePlaybookInputCount; i++ {
		tooManyValues[fmt.Sprintf("key-%02d", i)] = "value"
	}
	tests := []struct {
		name, tool, code string
		arguments        map[string]any
	}{
		{name: "context profile", tool: "bob_context", code: "input_invalid", arguments: map[string]any{"profile": "tiny"}},
		{name: "absolute path", tool: "bob_path", code: "input_invalid", arguments: map[string]any{"path": "/tmp/file"}},
		{name: "empty path", tool: "bob_path", code: "input_invalid", arguments: map[string]any{"path": ""}},
		{name: "path length", tool: "bob_path", code: "input_invalid", arguments: map[string]any{"path": strings.Repeat("p", maxGuidancePathBytes+1)}},
		{name: "playbook operation", tool: "bob_playbook", code: "input_invalid", arguments: map[string]any{"operation": "run"}},
		{name: "playbook id containing manifest", tool: "bob_playbook", code: "input_invalid", arguments: map[string]any{"operation": "show", "id": "manifest"}},
		{name: "playbook missing id", tool: "bob_playbook", code: "input_invalid", arguments: map[string]any{"operation": "show"}},
		{name: "playbook id length", tool: "bob_playbook", code: "input_invalid", arguments: map[string]any{"operation": "show", "id": strings.Repeat("p", maxGuidancePlaybookIDBytes+1)}},
		{name: "playbook input count", tool: "bob_playbook", code: "input_invalid", arguments: map[string]any{"operation": "plan", "id": "add-cli-command", "values": tooManyValues}},
		{name: "playbook key length", tool: "bob_playbook", code: "input_invalid", arguments: map[string]any{"operation": "plan", "id": "add-cli-command", "values": map[string]string{strings.Repeat("k", maxGuidanceInputKeyBytes+1): "value"}}},
		{name: "playbook value length", tool: "bob_playbook", code: "input_invalid", arguments: map[string]any{"operation": "plan", "id": "add-cli-command", "values": map[string]string{"command_name": strings.Repeat("v", maxGuidanceInputValueBytes+1)}}},
		{name: "oversized request", tool: "bob_playbook", code: "input_invalid", arguments: map[string]any{"operation": "plan", "id": "add-cli-command", "values": map[string]string{"command_name": strings.Repeat("a", maxGuidanceRequestBytes)}}},
		{name: "unauthorized context", tool: "bob_context", code: "workspace_unauthorized", arguments: map[string]any{"workspace": unauthorizedRoot}},
		{name: "unauthorized path", tool: "bob_path", code: "workspace_unauthorized", arguments: map[string]any{"workspace": unauthorizedRoot, "path": "README.md"}},
		{name: "unauthorized playbook", tool: "bob_playbook", code: "workspace_unauthorized", arguments: map[string]any{"workspace": unauthorizedRoot, "operation": "list"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: tc.tool, Arguments: tc.arguments})
			if err != nil || !result.IsError {
				t.Fatalf("result=%#v err=%v", result, err)
			}
			var output struct {
				Error *ErrorInfo `json:"error,omitempty"`
			}
			decodeStructuredExact(t, result, &output)
			if output.Error == nil || output.Error.Code != tc.code {
				t.Fatalf("error = %#v, want %s", output.Error, tc.code)
			}
		})
	}
}

func TestMCPPathRejectsSymlinkTraversalWithoutReadingTarget(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	session := connect(t, root, &offlineRunner{})
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_path", Arguments: map[string]any{"path": "link/secret"}})
	if err != nil || result.IsError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	var output PathOutput
	decodeStructuredExact(t, result, &output)
	if output.Path == nil || output.Path.Exists || output.Path.State != "symlink" || output.Path.HumanEditEffect != "unsafe" || output.Path.Ownership.CurrentSHA256 != "" {
		t.Fatalf("path output = %#v", output.Path)
	}
}

func decodeStructuredExact(t *testing.T, result *sdkmcp.CallToolResult, dst any) {
	t.Helper()
	decodeStructured(t, result, dst)
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].(*sdkmcp.TextContent).Text
	var structuredValue, textValue any
	if err := json.Unmarshal(structured, &structuredValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(text), &textValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(structuredValue, textValue) {
		t.Fatalf("structured and text content differ:\nstructured=%s\ntext=%s", structured, text)
	}
}

func assertContextTextProjection(t *testing.T, result *sdkmcp.CallToolResult, output ContextOutput) {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("context text content = %#v", result.Content)
	}
	text, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("context text content type = %T", result.Content[0])
	}
	var projection struct {
		SchemaVersion int  `json:"schema_version"`
		OK            bool `json:"ok"`
		Context       struct {
			ContractDigest string `json:"contract_digest"`
			ContextDigest  string `json:"context_digest"`
			Detail         string `json:"detail_location"`
			Repository     struct {
				State      string `json:"state"`
				PlanDigest string `json:"plan_digest"`
			} `json:"repository"`
		} `json:"context"`
	}
	if err := json.Unmarshal([]byte(text.Text), &projection); err != nil {
		t.Fatal(err)
	}
	if !projection.OK || projection.SchemaVersion != output.SchemaVersion || output.Context == nil ||
		projection.Context.Detail != "structuredContent" || projection.Context.ContractDigest != output.Context.ContractDigest ||
		projection.Context.ContextDigest != output.Context.ContextDigest || projection.Context.Repository.State != output.Context.Repository.State ||
		projection.Context.Repository.PlanDigest != output.Context.Repository.PlanDigest {
		t.Fatalf("context text projection = %#v", projection)
	}
}

func assertCallToolResponseBound(t *testing.T, name string, result *sdkmcp.CallToolResult, limit int) int {
	t.Helper()
	wire := struct {
		JSONRPC string                 `json:"jsonrpc"`
		ID      int                    `json:"id"`
		Result  *sdkmcp.CallToolResult `json:"result"`
	}{JSONRPC: "2.0", ID: 1, Result: result}
	data, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) >= limit {
		t.Fatalf("%s MCP JSON-RPC response = %d bytes, limit < %d", name, len(data), limit)
	}
	t.Logf("%s MCP JSON-RPC response: %d bytes", name, len(data))
	return len(data)
}
