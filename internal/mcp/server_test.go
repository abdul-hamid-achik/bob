package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type offlineRunner struct {
	mu       sync.Mutex
	runCalls int
}

func (*offlineRunner) LookPath(name string) (string, error) { return "/bin/" + name, nil }

func (r *offlineRunner) Run(context.Context, string, string, ...string) inspectpkg.CommandResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runCalls++
	return inspectpkg.CommandResult{}
}

func (r *offlineRunner) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runCalls
}

func TestServerExposesTwoTypedReadOnlyTools(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	runner := &offlineRunner{}
	session := connect(t, root, runner)
	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 2 {
		t.Fatalf("tool count = %d, want 2", len(listed.Tools))
	}
	want := map[string]bool{"bob_inspect": true, "bob_plan": true}
	for _, tool := range listed.Tools {
		if !want[tool.Name] {
			t.Fatalf("unexpected tool %q", tool.Name)
		}
		if tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Fatalf("tool %s omitted a typed schema", tool.Name)
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || !tool.Annotations.IdempotentHint || tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
			t.Fatalf("unsafe annotations for %s: %#v", tool.Name, tool.Annotations)
		}
	}

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_inspect", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("inspect: result=%#v err=%v", result, err)
	}
	var output InspectOutput
	decodeStructured(t, result, &output)
	if !output.OK || output.Report == nil || output.Report.Workspace == "" {
		t.Fatalf("unexpected inspect output: %#v", output)
	}
	if runner.calls() != 0 {
		t.Fatalf("read-only MCP inspect ran specialist probes %d time(s)", runner.calls())
	}
}

func TestPlanReturnsCompactActionsAndStructuredFailure(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	session := connect(t, root, &offlineRunner{})
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_plan", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("plan: result=%#v err=%v", result, err)
	}
	var output PlanOutput
	decodeStructured(t, result, &output)
	if !output.OK || output.Clean || output.Counts.Create == 0 || len(output.Actions) == 0 {
		t.Fatalf("unexpected plan: %#v", output)
	}
	for _, action := range output.Actions {
		if action.Path == "" || action.DesiredMode == "" {
			t.Fatalf("incomplete compact action: %#v", action)
		}
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) >= 32<<10 {
		t.Fatalf("default plan exceeds MCPHub's compact response budget: %d bytes", len(encoded))
	}

	missing := filepath.Join(t.TempDir(), "missing")
	result, err = session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "bob_plan", Arguments: map[string]any{"workspace": missing},
	})
	if err != nil || !result.IsError {
		t.Fatalf("invalid plan should be a structured tool error: result=%#v err=%v", result, err)
	}
	decodeStructured(t, result, &output)
	if output.OK || output.Error == nil || output.Error.Code != "workspace_invalid" {
		t.Fatalf("unexpected failure: %#v", output)
	}
}

func TestMCPStdioSubprocessHasCleanNewlineFraming(t *testing.T) {
	root := mcpWorkspace(t)
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), stdioHelperEnv+"=1", stdioHelperWorkspace+"="+root)
	var stderr synchronizedBuffer
	cmd.Stderr = &stderr

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "bob-stdio-test", Version: "0"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd, TerminateDuration: 3 * time.Second}, nil)
	if err != nil {
		t.Fatalf("stdio handshake: %v (stderr: %s)", err, stderr.String())
	}
	listed, err := session.ListTools(ctx, nil)
	if err != nil || len(listed.Tools) != 2 {
		t.Fatalf("list tools: count=%d err=%v stderr=%s", len(listed.Tools), err, stderr.String())
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("unexpected stderr: %q", got)
	}
}

func connect(t *testing.T, root string, runner inspectpkg.Runner) *sdkmcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()
	server, err := NewServer(root, runner)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.serve(ctx, serverTransport) }()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "bob-test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func decodeStructured(t *testing.T, result *sdkmcp.CallToolResult, dst any) {
	t.Helper()
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("decode structured content: %v (%s)", err, data)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected one JSON text block: %#v", result.Content)
	}
	text, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok || !json.Valid([]byte(text.Text)) {
		t.Fatalf("text content is not JSON: %#v", result.Content[0])
	}
}

func mcpWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, false); err != nil {
		t.Fatal(err)
	}
	return root
}

type synchronizedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}
