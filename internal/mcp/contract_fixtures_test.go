package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestConsumerContractFixturesMatchRealOutputs keeps the public consumer
// corpus tied to the exact structuredContent emitted by the typed MCP tools.
// Absolute temporary workspace paths are the only normalized values.
func TestConsumerContractFixturesMatchRealOutputs(t *testing.T) {
	t.Parallel()
	lookPath := func(name string) (string, error) { return "/bin/" + name, nil }

	cleanRoot := mcpWorkspace(t)
	applyWorkspace(t, cleanRoot)
	driftRoot := mcpWorkspace(t)
	conflictRoot := mcpWorkspace(t)
	applyWorkspace(t, conflictRoot)
	if err := os.WriteFile(filepath.Join(conflictRoot, "internal/cli/root.go"), []byte("package cli\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cleanSession := connectWithOptions(t, cleanRoot, &offlineRunner{}, ServerOptions{lookPath: lookPath})
	driftSession := connectWithOptions(t, driftRoot, &offlineRunner{}, ServerOptions{lookPath: lookPath})
	conflictSession := connectWithOptions(t, conflictRoot, &offlineRunner{}, ServerOptions{lookPath: lookPath})

	cleanContext := fixtureCall(t, cleanSession, cleanRoot, "bob_context", map[string]any{}, false)
	fixtures := map[string][]byte{
		"context-clean-v1.json":            cleanContext,
		"context-drift-v1.json":            fixtureCall(t, driftSession, driftRoot, "bob_context", map[string]any{}, false),
		"context-conflict-v1.json":         fixtureCall(t, conflictSession, conflictRoot, "bob_context", map[string]any{}, false),
		"path-managed-v1.json":             fixtureCall(t, cleanSession, cleanRoot, "bob_path", map[string]any{"path": "internal/cli/root.go"}, false),
		"path-extension-v1.json":           fixtureCall(t, cleanSession, cleanRoot, "bob_path", map[string]any{"path": "internal/cli/hello.go"}, false),
		"playbook-ready-v1.json":           fixtureCall(t, cleanSession, cleanRoot, "bob_playbook", map[string]any{"operation": "plan", "id": "add-cli-command", "values": map[string]string{"command_name": "hello"}}, false),
		"playbook-missing-input-v1.json":   fixtureCall(t, cleanSession, cleanRoot, "bob_playbook", map[string]any{"operation": "plan", "id": "add-cli-command"}, true),
		"error-unsupported-schema-v1.json": futureSchemaFixture(t, cleanContext),
	}

	for name, actual := range fixtures {
		name, actual := name, actual
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join("..", "..", "testdata", "contracts", name)
			if os.Getenv("BOB_UPDATE_CONTRACT_FIXTURES") == "1" {
				if err := os.WriteFile(path, actual, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			published, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal(published, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(actual, &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("published fixture differs from real MCP structuredContent; replace %s with:\n%s", path, actual)
			}
		})
	}
}

func fixtureCall(t *testing.T, session *sdkmcp.ClientSession, root, name string, arguments map[string]any, wantError bool) []byte {
	t.Helper()
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError != wantError {
		t.Fatalf("%s IsError = %v, want %v", name, result.IsError, wantError)
	}
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.ReplaceAll(string(data), root, "/workspace"))
	var normalized any
	if err := json.Unmarshal(data, &normalized); err != nil {
		t.Fatal(err)
	}
	compact, err := json.Marshal(normalized)
	if err != nil {
		t.Fatal(err)
	}
	return append(compact, '\n')
}

func futureSchemaFixture(t *testing.T, clean []byte) []byte {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(clean, &value); err != nil {
		t.Fatal(err)
	}
	value["schema_version"] = float64(2)
	contextValue, ok := value["context"].(map[string]any)
	if !ok {
		t.Fatal("clean fixture omitted context object")
	}
	contextValue["schema_version"] = float64(2)
	compact, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return append(compact, '\n')
}
