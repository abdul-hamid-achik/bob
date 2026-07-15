package mcp_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var (
	errUnsupportedContractSchema = errors.New("unsupported Bob consumer contract schema")
	qualifiedDigestPattern       = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type consumerEnvelope struct {
	SchemaVersion int             `json:"schema_version"`
	OK            bool            `json:"ok"`
	Authority     json.RawMessage `json:"authority"`
	Operation     string          `json:"operation"`
	Context       json.RawMessage `json:"context"`
	Path          json.RawMessage `json:"path"`
	Plan          json.RawMessage `json:"plan"`
	Error         *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestPublishedConsumerContractFixtures(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "..", "testdata", "contracts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			got = append(got, entry.Name())
		}
	}
	sort.Strings(got)
	want := []string{
		"context-clean-v1.json",
		"context-conflict-v1.json",
		"context-drift-v1.json",
		"error-unsupported-schema-v1.json",
		"path-extension-v1.json",
		"path-managed-v1.json",
		"playbook-missing-input-v1.json",
		"playbook-ready-v1.json",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("contract fixtures = %v, want %v", got, want)
	}

	for _, name := range got {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(data) {
				t.Fatal("fixture is not valid JSON")
			}
			switch {
			case strings.HasPrefix(name, "context-") && len(data) >= 8<<10:
				t.Fatalf("compact context fixture is %d bytes", len(data))
			case strings.HasPrefix(name, "path-") && len(data) >= 8<<10:
				t.Fatalf("path fixture is %d bytes", len(data))
			case strings.HasPrefix(name, "playbook-") && len(data) >= 24<<10:
				t.Fatalf("playbook fixture is %d bytes", len(data))
			}
			if bytes.Contains(data, []byte("/Users/")) || bytes.Contains(data, []byte("current_preview")) || bytes.Contains(data, []byte("desired_preview")) {
				t.Fatal("fixture contains a private path or raw-content preview field")
			}

			envelope, err := decodeConsumerV1(data)
			if name == "error-unsupported-schema-v1.json" {
				if !errors.Is(err, errUnsupportedContractSchema) {
					t.Fatalf("future schema error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			validateAuthority(t, envelope.Authority)
			switch name {
			case "context-clean-v1.json":
				validateContext(t, envelope, "clean", true)
			case "context-drift-v1.json":
				validateContext(t, envelope, "drifted", false)
			case "context-conflict-v1.json":
				validateContext(t, envelope, "conflicted", false)
			case "path-managed-v1.json":
				validatePath(t, envelope, "managed", "managed_in_sync")
			case "path-extension-v1.json":
				validatePath(t, envelope, "extension_point", "extension_point")
			case "playbook-ready-v1.json":
				validateReadyPlaybook(t, envelope)
			case "playbook-missing-input-v1.json":
				if envelope.OK || envelope.Operation != "plan" || envelope.Error == nil || envelope.Error.Code != "input_invalid" || envelope.Error.Message != "missing required inputs: command_name" || len(envelope.Plan) != 0 {
					t.Fatalf("missing-input contract = %#v", envelope)
				}
			}
		})
	}
}

func decodeConsumerV1(data []byte) (consumerEnvelope, error) {
	var envelope consumerEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return consumerEnvelope{}, fmt.Errorf("decode consumer envelope: %w", err)
	}
	if envelope.SchemaVersion != 1 {
		return consumerEnvelope{}, fmt.Errorf("%w: %d", errUnsupportedContractSchema, envelope.SchemaVersion)
	}
	return envelope, nil
}

func validateAuthority(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var authority struct {
		Mode                  string `json:"mode"`
		DefaultWorkspace      string `json:"default_workspace"`
		SelectedWorkspace     string `json:"selected_workspace"`
		AllowedWorkspaceCount int    `json:"allowed_workspace_count"`
	}
	if err := json.Unmarshal(raw, &authority); err != nil {
		t.Fatal(err)
	}
	if authority.Mode != "exact_allowlist" || authority.DefaultWorkspace != "/workspace" || authority.SelectedWorkspace != "/workspace" || authority.AllowedWorkspaceCount != 1 {
		t.Fatalf("authority = %#v", authority)
	}
}

func validateContext(t *testing.T, envelope consumerEnvelope, state string, clean bool) {
	t.Helper()
	if !envelope.OK || len(envelope.Context) == 0 {
		t.Fatalf("context envelope = %#v", envelope)
	}
	var context struct {
		SchemaVersion  int    `json:"schema_version"`
		Profile        string `json:"profile"`
		Workspace      string `json:"workspace"`
		ContractDigest string `json:"contract_digest"`
		ContextDigest  string `json:"context_digest"`
		Recipe         struct {
			ID      string `json:"id"`
			Version int    `json:"version"`
		} `json:"recipe"`
		Product struct {
			Name       string `json:"name"`
			Module     string `json:"module"`
			Runtime    string `json:"runtime"`
			Kind       string `json:"kind"`
			Visibility string `json:"visibility"`
		} `json:"product"`
		Repository struct {
			State             string `json:"state"`
			Clean             bool   `json:"clean"`
			LockChanged       bool   `json:"lock_changed"`
			ConflictCount     int    `json:"conflict_count"`
			ManagedFiles      int    `json:"managed_files"`
			PlanDigestVersion int    `json:"plan_digest_version"`
			PlanDigest        string `json:"plan_digest"`
		} `json:"repository"`
		Capabilities []struct {
			ID              string `json:"id"`
			Selection       string `json:"selection"`
			Materialization string `json:"materialization"`
			Availability    string `json:"availability"`
			Verification    string `json:"verification"`
		} `json:"capabilities"`
		EntryPoints []struct {
			ID        string `json:"id"`
			Path      string `json:"path"`
			Ownership string `json:"ownership"`
		} `json:"entry_points"`
		ExtensionPoints []struct {
			ID             string   `json:"id"`
			Ownership      string   `json:"ownership"`
			CreatePatterns []string `json:"create_patterns"`
		} `json:"extension_points"`
		Invariants []struct {
			ID        string `json:"id"`
			Statement string `json:"statement"`
		} `json:"invariants"`
		Playbooks []struct {
			ID             string   `json:"id"`
			Applicable     bool     `json:"applicable"`
			Available      bool     `json:"available"`
			BlockedBy      []string `json:"blocked_by"`
			RequiredInputs []string `json:"required_inputs"`
			ScopeClass     string   `json:"scope_class"`
			Risk           string   `json:"risk"`
		} `json:"playbooks"`
		Notices []json.RawMessage `json:"notices"`
		Actions []struct {
			ID         string   `json:"id"`
			Kind       string   `json:"kind"`
			Effect     string   `json:"effect"`
			CWD        string   `json:"cwd"`
			Argv       []string `json:"argv"`
			ReasonCode string   `json:"reason_code"`
			BlockedBy  []string `json:"blocked_by"`
		} `json:"actions"`
		Truncation struct {
			Profile   string         `json:"profile"`
			ByteLimit int            `json:"byte_limit"`
			Truncated bool           `json:"truncated"`
			Omitted   map[string]int `json:"omitted"`
		} `json:"truncation"`
	}
	if err := json.Unmarshal(envelope.Context, &context); err != nil {
		t.Fatal(err)
	}
	if context.SchemaVersion != 1 || context.Profile != "compact" || context.Workspace != "/workspace" || context.Recipe.ID != "go-agent-tool" || context.Recipe.Version != 4 {
		t.Fatalf("context identity = %#v", context)
	}
	if context.Product.Name != "acme" || context.Product.Module != "github.com/acme/acme" || context.Product.Runtime != "go" || context.Product.Kind != "cli" || context.Product.Visibility != "public" {
		t.Fatalf("context product = %#v", context.Product)
	}
	if context.Repository.State != state || context.Repository.Clean != clean || context.Repository.PlanDigestVersion != 1 {
		t.Fatalf("repository state = %#v", context.Repository)
	}
	if context.Repository.ManagedFiles == 0 || context.Repository.LockChanged != (state == "drifted") || context.Repository.ConflictCount != map[string]int{"clean": 0, "drifted": 0, "conflicted": 1}[state] {
		t.Fatalf("repository details = %#v", context.Repository)
	}
	for _, digest := range []string{context.ContractDigest, context.ContextDigest, context.Repository.PlanDigest} {
		if !qualifiedDigestPattern.MatchString(digest) {
			t.Fatalf("digest = %q", digest)
		}
	}
	if len(context.Capabilities) != 14 || len(context.EntryPoints) != 2 || len(context.ExtensionPoints) != 3 || len(context.Invariants) != 5 || len(context.Playbooks) != 7 {
		t.Fatalf("context catalog sizes: capabilities=%d entry_points=%d extension_points=%d invariants=%d playbooks=%d", len(context.Capabilities), len(context.EntryPoints), len(context.ExtensionPoints), len(context.Invariants), len(context.Playbooks))
	}
	for _, capability := range context.Capabilities {
		if capability.ID == "" || capability.Selection == "" || capability.Materialization == "" || capability.Availability == "" || capability.Verification != "not_assessed" {
			t.Fatalf("capability state = %#v", capability)
		}
	}
	for _, entry := range context.EntryPoints {
		if entry.ID == "" || entry.Path == "" || entry.Ownership == "" {
			t.Fatalf("entry point = %#v", entry)
		}
	}
	for _, extension := range context.ExtensionPoints {
		if extension.ID == "" || extension.Ownership != "human" || len(extension.CreatePatterns) == 0 {
			t.Fatalf("extension point = %#v", extension)
		}
	}
	for _, invariant := range context.Invariants {
		if invariant.ID == "" || invariant.Statement == "" {
			t.Fatalf("invariant = %#v", invariant)
		}
	}
	for _, playbook := range context.Playbooks {
		if playbook.ID == "" || playbook.ScopeClass == "" || playbook.Risk == "" || !playbook.Applicable {
			t.Fatalf("playbook summary = %#v", playbook)
		}
	}
	if state == "clean" && len(context.Actions) != 0 || state != "clean" && (len(context.Actions) != 1 || context.Actions[0].ID == "" || context.Actions[0].Effect != "read_only" || context.Actions[0].CWD != "/workspace" || len(context.Actions[0].Argv) == 0 || context.Actions[0].ReasonCode == "") {
		t.Fatalf("context actions = %#v", context.Actions)
	}
	if context.Truncation.Profile != "compact" || context.Truncation.ByteLimit != 6144 || context.Truncation.Truncated || context.Truncation.Omitted == nil {
		t.Fatalf("context truncation = %#v", context.Truncation)
	}
}

func validatePath(t *testing.T, envelope consumerEnvelope, classification, state string) {
	t.Helper()
	if !envelope.OK || len(envelope.Path) == 0 || len(envelope.Path) >= 8<<10 {
		t.Fatalf("path envelope = %#v", envelope)
	}
	var path struct {
		SchemaVersion   int      `json:"schema_version"`
		Workspace       string   `json:"workspace"`
		Path            string   `json:"path"`
		Exists          bool     `json:"exists"`
		Classification  string   `json:"classification"`
		State           string   `json:"state"`
		HumanEditEffect string   `json:"human_edit_effect"`
		ExtensionPoints []string `json:"extension_points"`
		Ownership       struct {
			Recipe struct {
				ID      string `json:"id"`
				Version int    `json:"version"`
			} `json:"recipe"`
			LockedSHA256  string `json:"locked_sha256"`
			CurrentSHA256 string `json:"current_sha256"`
		} `json:"ownership"`
		PlanAction *struct {
			Kind string `json:"kind"`
			Code string `json:"code"`
		} `json:"plan_action"`
		Artifact *struct {
			ID            string   `json:"id"`
			Roles         []string `json:"roles"`
			CapabilityIDs []string `json:"capability_ids"`
		} `json:"artifact"`
		RelatedPlaybooks []string          `json:"related_playbooks"`
		Notices          []json.RawMessage `json:"notices"`
		Actions          []struct {
			ID     string   `json:"id"`
			Effect string   `json:"effect"`
			CWD    string   `json:"cwd"`
			Argv   []string `json:"argv"`
		} `json:"actions"`
		Truncation struct {
			Profile   string         `json:"profile"`
			ByteLimit int            `json:"byte_limit"`
			Truncated bool           `json:"truncated"`
			Omitted   map[string]int `json:"omitted"`
		} `json:"truncation"`
	}
	if err := json.Unmarshal(envelope.Path, &path); err != nil {
		t.Fatal(err)
	}
	if path.SchemaVersion != 1 || path.Workspace != "/workspace" || path.Path == "" || path.Classification != classification || path.State != state || path.HumanEditEffect == "" || path.Ownership.Recipe.ID != "go-agent-tool" || path.Ownership.Recipe.Version != 4 {
		t.Fatalf("path contract = %#v", path)
	}
	if len(path.RelatedPlaybooks) != 1 || path.RelatedPlaybooks[0] != "add-cli-command" || len(path.Actions) != 1 || path.Actions[0].ID != "show_playbook:add-cli-command" || path.Actions[0].Effect != "read_only" || path.Actions[0].CWD != "/workspace" || len(path.Actions[0].Argv) == 0 {
		t.Fatalf("path guidance = %#v", path)
	}
	if path.Truncation.Profile != "path" || path.Truncation.ByteLimit != 8192 || path.Truncation.Truncated || path.Truncation.Omitted == nil {
		t.Fatalf("path truncation = %#v", path.Truncation)
	}
	if classification == "extension_point" && (len(path.ExtensionPoints) != 1 || path.ExtensionPoints[0] != "cli.command_files") {
		t.Fatalf("extension points = %v", path.ExtensionPoints)
	}
	if classification == "managed" {
		if !path.Exists || path.PlanAction == nil || path.PlanAction.Kind != "unchanged" || path.PlanAction.Code != "in_sync" || path.Artifact == nil || path.Artifact.ID != "cli.root" || len(path.Artifact.Roles) == 0 || len(path.Artifact.CapabilityIDs) == 0 || len(path.Ownership.LockedSHA256) != 64 || path.Ownership.LockedSHA256 != path.Ownership.CurrentSHA256 {
			t.Fatalf("managed path ownership = %#v", path)
		}
	} else if path.Exists || path.PlanAction != nil || path.Artifact != nil {
		t.Fatalf("extension path invented ownership = %#v", path)
	}
}

func validateReadyPlaybook(t *testing.T, envelope consumerEnvelope) {
	t.Helper()
	if !envelope.OK || envelope.Operation != "plan" || len(envelope.Plan) == 0 || len(envelope.Plan) >= 24<<10 {
		t.Fatalf("playbook envelope = %#v", envelope)
	}
	var plan struct {
		SchemaVersion int               `json:"schema_version"`
		Workspace     string            `json:"workspace"`
		Values        map[string]string `json:"values"`
		Observations  []json.RawMessage `json:"observations"`
		Recipe        struct {
			ID      string `json:"id"`
			Version int    `json:"version"`
		} `json:"recipe"`
		Playbook struct {
			ID         string `json:"id"`
			Applicable bool   `json:"applicable"`
			Available  bool   `json:"available"`
			ScopeClass string `json:"scope_class"`
			Risk       string `json:"risk"`
			Inputs     []struct {
				Name       string `json:"name"`
				Required   bool   `json:"required"`
				Type       string `json:"type"`
				Validation string `json:"validation"`
			} `json:"inputs"`
			Preconditions []string `json:"preconditions"`
			Boundary      struct {
				Create    []string `json:"create"`
				Modify    []string `json:"modify"`
				Forbidden []string `json:"forbidden"`
			} `json:"boundary"`
			Steps []struct {
				ID                        string   `json:"id"`
				Kind                      string   `json:"kind"`
				Effect                    string   `json:"effect"`
				Summary                   string   `json:"summary"`
				Paths                     []string `json:"paths"`
				Argv                      []string `json:"argv"`
				DependsOn                 []string `json:"depends_on"`
				RequiresExplicitAuthority bool     `json:"requires_explicit_authority"`
				SuccessCondition          string   `json:"success_condition"`
				BlockedBy                 []string `json:"blocked_by"`
			} `json:"steps"`
			VerificationHints []string `json:"verification_hints"`
			FailureModes      []string `json:"failure_modes"`
			CapabilityIDs     []string `json:"capability_ids"`
			ExtensionPointIDs []string `json:"extension_point_ids"`
		} `json:"playbook"`
		Truncation struct {
			Profile   string         `json:"profile"`
			ByteLimit int            `json:"byte_limit"`
			Truncated bool           `json:"truncated"`
			Omitted   map[string]int `json:"omitted"`
		} `json:"truncation"`
	}
	if err := json.Unmarshal(envelope.Plan, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.SchemaVersion != 1 || plan.Workspace != "/workspace" || plan.Recipe.ID != "go-agent-tool" || plan.Recipe.Version != 4 || plan.Playbook.ID != "add-cli-command" || !plan.Playbook.Applicable || !plan.Playbook.Available || plan.Playbook.ScopeClass != "small" || plan.Playbook.Risk != "medium" || plan.Values["command_name"] != "hello" || len(plan.Playbook.Steps) != 5 {
		t.Fatalf("playbook plan = %#v", plan)
	}
	if len(plan.Playbook.Inputs) != 1 || plan.Playbook.Inputs[0].Name != "command_name" || !plan.Playbook.Inputs[0].Required || plan.Playbook.Inputs[0].Type != "identifier" || plan.Playbook.Inputs[0].Validation != "lowercase-kebab" || len(plan.Playbook.Boundary.Create) != 2 || len(plan.Playbook.Boundary.Forbidden) < 3 || len(plan.Playbook.VerificationHints) == 0 || len(plan.Playbook.FailureModes) == 0 || len(plan.Playbook.CapabilityIDs) != 2 || len(plan.Playbook.ExtensionPointIDs) != 1 {
		t.Fatalf("playbook definition = %#v", plan.Playbook)
	}
	for _, step := range plan.Playbook.Steps {
		if step.ID == "" || step.Kind == "" || step.Effect == "" || step.Summary == "" || step.SuccessCondition == "" {
			t.Fatalf("invalid step = %#v", step)
		}
	}
	if plan.Truncation.Profile != "plan" || plan.Truncation.ByteLimit != 24<<10 || plan.Truncation.Truncated || plan.Truncation.Omitted == nil {
		t.Fatalf("playbook truncation = %#v", plan.Truncation)
	}
}
