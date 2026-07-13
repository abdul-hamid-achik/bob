package studio

import (
	"strings"
	"testing"
)

func TestValidateTerminal(t *testing.T) {
	for _, tc := range []struct {
		name              string
		stdin, stdout     bool
		terminal          string
		wantErrorContains string
	}{
		{name: "interactive", stdin: true, stdout: true, terminal: "xterm-256color"},
		{name: "piped stdin", stdout: true, terminal: "xterm", wantErrorContains: "interactive terminal"},
		{name: "piped stdout", stdin: true, terminal: "xterm", wantErrorContains: "interactive terminal"},
		{name: "dumb", stdin: true, stdout: true, terminal: "dumb", wantErrorContains: "TERM=dumb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTerminal(tc.stdin, tc.stdout, tc.terminal)
			if tc.wantErrorContains == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantErrorContains != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErrorContains)) {
				t.Fatalf("error = %v, want %q", err, tc.wantErrorContains)
			}
		})
	}
}
