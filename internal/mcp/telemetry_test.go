package mcp

import (
	"context"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/telemetry"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStatsReportsMCPCallsFromEnabledLocalStore(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	store, err := telemetry.Open(telemetry.Config{
		StateDir: t.TempDir(), Enabled: true, RetentionDays: 30, MaxEventsPerDay: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	session := connectWithOptions(t, root, &offlineRunner{}, ServerOptions{
		Recorder: store, Telemetry: store,
	})
	plan, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_plan", Arguments: map[string]any{}})
	if err != nil || plan.IsError {
		t.Fatalf("plan: result=%#v err=%v", plan, err)
	}
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_stats", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("stats: result=%#v err=%v", result, err)
	}
	var output StatsOutput
	decodeStructured(t, result, &output)
	if !output.OK || !output.Enabled || !output.LocalOnly || output.Stats.Events != 1 {
		t.Fatalf("unexpected stats output: %#v", output)
	}
	if len(output.Stats.ByOperation) != 1 || output.Stats.ByOperation[0].Operation != telemetry.OperationPlan {
		t.Fatalf("unexpected per-operation stats: %#v", output.Stats.ByOperation)
	}
}

func TestStatsIsStableWhenLocalTelemetryIsDisabled(t *testing.T) {
	t.Parallel()
	session := connect(t, mcpWorkspace(t), &offlineRunner{})
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_stats", Arguments: map[string]any{"all": true}})
	if err != nil || result.IsError {
		t.Fatalf("stats: result=%#v err=%v", result, err)
	}
	var output StatsOutput
	decodeStructured(t, result, &output)
	if !output.OK || output.Enabled || !output.LocalOnly || output.Stats.Events != 0 {
		t.Fatalf("unexpected disabled stats output: %#v", output)
	}
}
