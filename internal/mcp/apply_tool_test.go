package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/telemetry"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func callApply(t *testing.T, session *sdkmcp.ClientSession, arguments map[string]any) (*sdkmcp.CallToolResult, ApplyOutput) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_apply", Arguments: arguments})
	if err != nil {
		t.Fatalf("bob_apply transport error: %v", err)
	}
	var output ApplyOutput
	decodeStructured(t, result, &output)
	return result, output
}

func reviewedDigest(t *testing.T, session *sdkmcp.ClientSession, root string) string {
	t.Helper()
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "bob_plan", Arguments: map[string]any{"workspace": root},
	})
	if err != nil || result.IsError {
		t.Fatalf("plan: result=%#v err=%v", result, err)
	}
	var plan PlanOutput
	decodeStructured(t, result, &plan)
	if !strings.HasPrefix(plan.PlanDigestQualified, "sha256:") {
		t.Fatalf("plan omitted qualified digest: %#v", plan)
	}
	return plan.PlanDigestQualified
}

func TestApplyToolIsAnnotatedAsMutatingNotReadOnly(t *testing.T) {
	t.Parallel()
	session := connect(t, mcpWorkspace(t), &offlineRunner{})
	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	readOnly := 0
	var apply *sdkmcp.Tool
	for _, tool := range listed.Tools {
		if tool.Name == "bob_apply" {
			apply = tool
			continue
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || !tool.Annotations.IdempotentHint ||
			tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
			t.Fatalf("read tool %s lost its read-only annotations: %#v", tool.Name, tool.Annotations)
		}
		readOnly++
	}
	if readOnly != 9 {
		t.Fatalf("read-only tool count = %d, want 9", readOnly)
	}
	if apply == nil {
		t.Fatal("bob_apply is not registered")
	}
	a := apply.Annotations
	if a == nil || a.ReadOnlyHint || a.IdempotentHint || a.DestructiveHint == nil || !*a.DestructiveHint ||
		a.OpenWorldHint == nil || *a.OpenWorldHint {
		t.Fatalf("bob_apply must be destructive, non-idempotent, closed-world, and not read-only: %#v", a)
	}
	if strings.Contains(strings.ToLower(apply.Description), "read-only") || !strings.Contains(apply.Description, "Mutates") {
		t.Fatalf("bob_apply description must not claim read-only: %q", apply.Description)
	}
	if apply.InputSchema == nil || apply.OutputSchema == nil {
		t.Fatal("bob_apply omitted a typed schema")
	}
	schema := fmt.Sprintf("%v", apply.InputSchema)
	if !strings.Contains(schema, "expect_plan_digest") {
		t.Fatalf("input schema omits expect_plan_digest: %s", schema)
	}
}

func TestApplyWithoutApplyAuthorityWritesNothing(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	other := mcpWorkspace(t)
	runner := &offlineRunner{}
	before := fileSnapshot(t, root)

	// Neither the default exact allowlist, --allow-workspace, nor
	// --allow-any-workspace grants mutation authority.
	for name, options := range map[string]ServerOptions{
		"default":         {},
		"allow-workspace": {AllowedWorkspaces: []string{root}},
		"allow-any":       {AllowAnyWorkspace: true},
	} {
		session := connectWithOptions(t, root, runner, options)
		digest := reviewedDigest(t, session, root)
		result, output := callApply(t, session, map[string]any{"expect_plan_digest": digest})
		if !result.IsError || output.OK || output.Outcome != "apply_unauthorized" || output.Error == nil || output.Error.Code != "apply_unauthorized" {
			t.Fatalf("%s: unauthorized apply returned %#v", name, output)
		}
		if output.ApplyAuthority.Mode != "disabled" || output.ApplyAuthority.AllowedWorkspaceCount != 0 || output.Receipt != nil {
			t.Fatalf("%s: unexpected apply authority disclosure: %#v", name, output)
		}
		assertSnapshotEqual(t, before, fileSnapshot(t, root))
	}

	// An apply allowlist for one workspace does not authorize another.
	session := connectWithOptions(t, root, runner, ServerOptions{AllowedApplyWorkspaces: []string{other}, AllowAnyWorkspace: true})
	digest := reviewedDigest(t, session, root)
	_, output := callApply(t, session, map[string]any{"workspace": root, "expect_plan_digest": digest})
	if output.Outcome != "apply_unauthorized" || output.ApplyAuthority.Mode != "exact_allowlist" || output.ApplyAuthority.AllowedWorkspaceCount != 1 {
		t.Fatalf("apply outside the apply allowlist returned %#v", output)
	}
	assertSnapshotEqual(t, before, fileSnapshot(t, root))
	if _, err := os.Stat(filepath.Join(root, engine.LockFilename)); !os.IsNotExist(err) {
		t.Fatalf("unauthorized apply wrote bob.lock: %v", err)
	}
	if runner.calls() != 0 {
		t.Fatalf("apply authority checks invoked subprocess runner %d time(s)", runner.calls())
	}
}

func TestApplyRejectsNonExactDigestWithoutWriting(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	session := connectWithOptions(t, root, &offlineRunner{}, ServerOptions{AllowedApplyWorkspaces: []string{root}})
	digest := reviewedDigest(t, session, root)
	before := fileSnapshot(t, root)
	for _, candidate := range []string{
		strings.TrimPrefix(digest, "sha256:"),
		strings.ToUpper(digest),
		" " + digest,
		"sha256:" + strings.Repeat("0", 63),
		"sha512:" + strings.Repeat("0", 64),
	} {
		_, output := callApply(t, session, map[string]any{"expect_plan_digest": candidate})
		if output.Outcome != "input_invalid" || output.Error == nil || output.Error.Code != "input_invalid" {
			t.Fatalf("digest %q: unexpected output %#v", candidate, output)
		}
	}
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_apply", Arguments: map[string]any{}})
	if err == nil && !result.IsError {
		t.Fatalf("missing expect_plan_digest must fail: %#v", result)
	}
	assertSnapshotEqual(t, before, fileSnapshot(t, root))
}

func TestApplyWithReviewedDigestReturnsReceiptThenReportsConvergedMismatch(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	store, err := telemetry.Open(telemetry.Config{
		StateDir: t.TempDir(), Enabled: true, RetentionDays: 30, MaxEventsPerDay: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	session := connectWithOptions(t, root, &offlineRunner{}, ServerOptions{
		AllowedApplyWorkspaces: []string{root}, Recorder: store, Telemetry: store,
	})
	digest := reviewedDigest(t, session, root)

	result, output := callApply(t, session, map[string]any{"expect_plan_digest": digest})
	if result.IsError || !output.OK || output.Outcome != "applied" || output.Error != nil || output.Receipt == nil {
		t.Fatalf("reviewed apply failed: %#v", output)
	}
	receipt := output.Receipt
	if receipt.ExpectedPlanDigest != digest || receipt.AppliedPlanDigest != digest || !receipt.ConvergedAfterApply ||
		!receipt.LockWritten || receipt.WrittenCount == 0 || receipt.SchemaVersion != engine.ApplyReceiptSchemaVersion {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
	if output.ApplyAuthority.Mode != "exact_allowlist" || output.Workspace != root || len(output.NextActions) != 1 {
		t.Fatalf("unexpected applied output: %#v", output)
	}
	if _, err := os.Stat(filepath.Join(root, engine.LockFilename)); err != nil {
		t.Fatalf("apply did not publish bob.lock: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, engine.ApplyLockFilename)); !os.IsNotExist(err) {
		t.Fatalf("apply left its workspace lock behind: %v", err)
	}

	checkResult, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_check", Arguments: map[string]any{}})
	if err != nil || checkResult.IsError {
		t.Fatalf("check: result=%#v err=%v", checkResult, err)
	}
	var check CheckOutput
	decodeStructured(t, checkResult, &check)
	if !check.Clean {
		t.Fatalf("workspace not converged after MCP apply: %#v", check)
	}

	// Replaying the now-stale reviewed digest is a typed mismatch. Bob keeps
	// no history, so it reports only that the current plan is converged.
	after := fileSnapshot(t, root)
	result, output = callApply(t, session, map[string]any{"expect_plan_digest": digest})
	if !result.IsError || output.Outcome != "plan_digest_mismatch" || output.Error == nil || output.Error.Code != "plan_digest_mismatch" {
		t.Fatalf("replayed digest: %#v", output)
	}
	if output.Converged == nil || !*output.Converged || output.ExpectedPlanDigest != digest ||
		output.ActualPlanDigest != check.PlanDigestQualified || output.ActualPlanDigest == digest || output.Receipt != nil {
		t.Fatalf("replayed digest mismatch details: %#v", output)
	}
	if strings.Contains(strings.ToLower(output.Error.Message), "already applied") {
		t.Fatalf("mismatch must never claim the plan was already applied: %q", output.Error.Message)
	}
	assertSnapshotEqual(t, after, fileSnapshot(t, root))

	stats, err := store.Aggregate(context.Background(), telemetry.Query{})
	if err != nil {
		t.Fatal(err)
	}
	applyEvents := 0
	for _, operation := range stats.ByOperation {
		if operation.Operation == telemetry.OperationApply {
			applyEvents = operation.Events
		}
	}
	if applyEvents != 2 {
		t.Fatalf("apply telemetry events = %d, want 2: %#v", applyEvents, stats.ByOperation)
	}
}

func TestApplyStaleDigestReportsUnconvergedMismatchWithoutWriting(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	session := connectWithOptions(t, root, &offlineRunner{}, ServerOptions{AllowedApplyWorkspaces: []string{root}})
	reviewed := reviewedDigest(t, session, root)

	// The workspace changes after review: bob.yaml now describes a new plan.
	manifestPath := filepath.Join(root, "bob.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(data), "Acme", "Acme changed", 1)
	if changed == string(data) {
		t.Fatalf("fixture manifest has no description to change:\n%s", data)
	}
	if err := os.WriteFile(manifestPath, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	current := reviewedDigest(t, session, root)
	before := fileSnapshot(t, root)

	result, output := callApply(t, session, map[string]any{"expect_plan_digest": reviewed})
	if !result.IsError || output.Outcome != "plan_digest_mismatch" || output.Converged == nil || *output.Converged {
		t.Fatalf("stale digest: %#v", output)
	}
	if output.ExpectedPlanDigest != reviewed || output.ActualPlanDigest != current || output.Retryable || len(output.NextActions) != 1 {
		t.Fatalf("stale digest details: %#v", output)
	}
	assertSnapshotEqual(t, before, fileSnapshot(t, root))
}

func TestApplyConflictedWorkspaceReturnsConflictsAndWritesNothing(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	applyWorkspace(t, root)
	lock, err := engine.LoadLock(root)
	if err != nil || len(lock.Files) == 0 {
		t.Fatalf("load lock: %v (%d files)", err, len(lock.Files))
	}
	blocked := lock.Files[0].Path
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(blocked)), []byte("human edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	session := connectWithOptions(t, root, &offlineRunner{}, ServerOptions{AllowedApplyWorkspaces: []string{root}})
	digest := reviewedDigest(t, session, root)
	before := fileSnapshot(t, root)

	result, output := callApply(t, session, map[string]any{"expect_plan_digest": digest})
	if !result.IsError || output.OK || output.Outcome != "conflicts" || output.Error == nil || output.Error.Code != "conflicts" {
		t.Fatalf("conflicted apply: %#v", output)
	}
	if len(output.Conflicts) != 1 || output.Conflicts[0].Path != blocked || output.Conflicts[0].Code == "" || output.Receipt != nil {
		t.Fatalf("unexpected conflicts: %#v", output.Conflicts)
	}
	assertSnapshotEqual(t, before, fileSnapshot(t, root))
}

func TestApplyLockContentionFailsImmediatelyAsRetryableBusy(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	session := connectWithOptions(t, root, &offlineRunner{}, ServerOptions{AllowedApplyWorkspaces: []string{root}})
	digest := reviewedDigest(t, session, root)
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	// A lock owned by this live process is never stale, so the engine's
	// single exclusive create fails without waiting.
	lockPath := filepath.Join(root, engine.ApplyLockFilename)
	if err := os.WriteFile(lockPath, []byte(fmt.Sprintf("pid: %d\nhost: %s\n", os.Getpid(), host)), 0o600); err != nil {
		t.Fatal(err)
	}
	before := fileSnapshot(t, root)
	result, output := callApply(t, session, map[string]any{"expect_plan_digest": digest})
	if !result.IsError || output.Outcome != "workspace_busy" || !output.Retryable || output.Error == nil || output.Error.Code != "workspace_busy" {
		t.Fatalf("contended apply: %#v", output)
	}
	assertSnapshotEqual(t, before, fileSnapshot(t, root))
}

func TestApplyHonorsCancellationOnlyBeforeTheEngineStarts(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	server, err := NewServerWithOptions(root, &offlineRunner{}, ServerOptions{AllowedApplyWorkspaces: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	plan, _, err := buildPlan(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := fileSnapshot(t, root)
	result, output, err := server.handleApply(ctx, nil, ApplyInput{ExpectPlanDigest: engine.DigestPlan(plan).Qualified()})
	if err != nil || !result.IsError || output.Outcome != "cancelled" {
		t.Fatalf("cancelled apply: result=%#v output=%#v err=%v", result, output, err)
	}
	assertSnapshotEqual(t, before, fileSnapshot(t, root))
}

func TestApplyAuthorityImpliesReadButReadNeverImpliesApply(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	other := mcpWorkspace(t)
	session := connectWithOptions(t, root, &offlineRunner{}, ServerOptions{AllowedApplyWorkspaces: []string{other}})

	// The apply workspace must be reviewable, so it joins the read allowlist.
	digest := reviewedDigest(t, session, other)
	_, output := callApply(t, session, map[string]any{"workspace": other, "expect_plan_digest": digest})
	if output.Outcome != "applied" || output.Receipt == nil {
		t.Fatalf("apply on the apply-allowed workspace: %#v", output)
	}

	// The startup workspace stays readable but is not mutable.
	before := fileSnapshot(t, root)
	rootDigest := reviewedDigest(t, session, root)
	_, output = callApply(t, session, map[string]any{"expect_plan_digest": rootDigest})
	if output.Outcome != "apply_unauthorized" {
		t.Fatalf("startup workspace must not be mutable without apply authority: %#v", output)
	}
	assertSnapshotEqual(t, before, fileSnapshot(t, root))
}
