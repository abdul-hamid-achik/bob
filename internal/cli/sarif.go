package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/version"
)

// SARIF 2.1.0 output contract. Bob emits a drop-in artifact for GitHub code
// scanning and other SARIF consumers: one result per non-converged action,
// keyed by the action's stable code, with a severity derived from its kind.
const (
	sarifSchemaURI = "https://json.schemastore.org/sarif-2.1.0.json"
	sarifVersion   = "2.1.0"
)

// sarifDocument is the root SARIF log object.
type sarifDocument struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri"`
	Version        string      `json:"version"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string       `json:"id"`
	ShortDescription sarifMessage `json:"shortDescription"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

// sarifLevel maps an action kind to its SARIF result level. Unchanged and
// adopt actions are not reported, so they return the empty string and are
// skipped by buildSARIF.
func sarifLevel(kind engine.ActionKind) string {
	switch kind {
	case engine.ActionConflict:
		return "error"
	case engine.ActionUpdate, engine.ActionRetire:
		return "warning"
	case engine.ActionCreate:
		return "note"
	default:
		return ""
	}
}

// emitSARIF writes plan as a SARIF 2.1.0 log to w. It mirrors the JSON-output
// discipline: stdout carries only this document, with 2-space indentation and
// no HTML escaping.
func emitSARIF(w io.Writer, plan engine.PlanResult) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(buildSARIF(plan)); err != nil {
		return fmt.Errorf("encode SARIF: %w", err)
	}
	return nil
}

// buildSARIF reduces a plan to its SARIF projection. Actions of kind
// unchanged and adopt are omitted: they describe convergence, not a finding,
// and seed-once artifacts surface through those same kinds. Rules are
// derived from the codes present in the reported results and sorted for a
// deterministic document.
func buildSARIF(plan engine.PlanResult) sarifDocument {
	doc := sarifDocument{
		Schema:  sarifSchemaURI,
		Version: sarifVersion,
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name:           "bob",
				InformationURI: "https://bobcli.dev",
				Version:        version.Version,
			}},
		}},
	}
	run := &doc.Runs[0]
	seen := make(map[string]struct{})
	run.Results = make([]sarifResult, 0, len(plan.Actions))
	for _, action := range plan.Actions {
		level := sarifLevel(action.Kind)
		if level == "" {
			continue
		}
		run.Results = append(run.Results, sarifResult{
			RuleID:  action.Code,
			Level:   level,
			Message: sarifMessage{Text: action.Reason},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: action.Path},
				},
			}},
		})
		seen[action.Code] = struct{}{}
	}
	codes := make([]string, 0, len(seen))
	for code := range seen {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	run.Tool.Driver.Rules = make([]sarifRule, 0, len(codes))
	for _, code := range codes {
		run.Tool.Driver.Rules = append(run.Tool.Driver.Rules, sarifRule{
			ID:               code,
			ShortDescription: sarifMessage{Text: code},
		})
	}
	return doc
}
