package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMCPHubLocalAgentScopeRoute is opt-in because it exercises the user's
// installed Bob binary and MCPHub's local-agent-scoped configuration. It does
// not start Local Agent or test its outer namespacing and approval policy.
func TestMCPHubLocalAgentScopeRoute(t *testing.T) {
	if os.Getenv("BOB_TEST_MCPHUB") != "1" {
		t.Skip("set BOB_TEST_MCPHUB=1 to test the installed local-agent gateway route")
	}
	root := mcpWorkspace(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	dbPath := filepath.Join(t.TempDir(), "mcphub.db")
	cmd := exec.Command("mcphub", "--db", dbPath, "mcp", "serve", "--agent", "local-agent")
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "bob-mcphub-test", Version: "0"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd, TerminateDuration: 3 * time.Second}, nil)
	if err != nil {
		t.Fatalf("connect to MCPHub local-agent route: %v", err)
	}
	defer func() { _ = session.Close() }()

	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"bob__bob_inspect": false, "bob__bob_plan": false}
	for _, tool := range listed.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("local-agent gateway omitted pinned tool %s", name)
		}
	}
	for _, name := range []string{"bob__bob_inspect", "bob__bob_plan"} {
		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: name, Arguments: map[string]any{"workspace": root},
		})
		if err != nil || result.IsError {
			t.Fatalf("call %s through MCPHub: result=%#v err=%v", name, result, err)
		}
	}
}
