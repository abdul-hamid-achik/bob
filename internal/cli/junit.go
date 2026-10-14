package cli

import (
	"encoding/xml"
	"fmt"
	"io"

	"github.com/abdul-hamid-achik/bob/internal/engine"
)

// JUnit XML output contract. Bob emits a drop-in artifact for CI test-reporting
// frontends: one testcase per action, with a <failure> element on every
// non-converged action (create, update, retire, conflict) so a failing plan or
// check surfaces as a failing test run.
type junitTestSuites struct {
	XMLName    xml.Name         `xml:"testsuites"`
	TestSuites []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	TestCases []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure"`
}

type junitFailure struct {
	Type    string `xml:"type,attr"`
	Message string `xml:"message,attr"`
}

// junitIsFailure reports whether an action kind is a non-converged finding.
// create, update, retire, and conflict actions all fail the check gate;
// unchanged and adopt describe convergence and pass.
func junitIsFailure(kind engine.ActionKind) bool {
	switch kind {
	case engine.ActionCreate, engine.ActionUpdate, engine.ActionRetire, engine.ActionConflict:
		return true
	default:
		return false
	}
}

// emitJUnit writes plan as a JUnit XML report to w. It mirrors the JSON and
// SARIF output discipline: stdout carries only this document.
func emitJUnit(w io.Writer, command string, plan engine.PlanResult) error {
	data, err := xml.MarshalIndent(buildJUnit(command, plan), "", "  ")
	if err != nil {
		return fmt.Errorf("encode JUnit: %w", err)
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return fmt.Errorf("write JUnit header: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write JUnit: %w", err)
	}
	return nil
}

// buildJUnit reduces a plan to its JUnit projection: one testcase per action,
// keyed by the repository-relative path, with a failure element carrying the
// action's stable code and human reason. The suite is named after the command
// that produced it so a single upload can hold both plan and check runs.
func buildJUnit(command string, plan engine.PlanResult) junitTestSuites {
	suite := junitTestSuite{
		Name:      "bob " + command,
		Tests:     len(plan.Actions),
		TestCases: make([]junitTestCase, 0, len(plan.Actions)),
	}
	for _, action := range plan.Actions {
		tc := junitTestCase{
			Name:      action.Path,
			ClassName: string(action.Kind),
			Time:      "0",
		}
		if junitIsFailure(action.Kind) {
			suite.Failures++
			tc.Failure = &junitFailure{Type: action.Code, Message: action.Reason}
		}
		suite.TestCases = append(suite.TestCases, tc)
	}
	return junitTestSuites{TestSuites: []junitTestSuite{suite}}
}
