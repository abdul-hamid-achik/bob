package strsim

import "testing"

func TestDistance(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"codemap", "codemap", 0},
		{"codemp", "codemap", 1},
		{"go_agent_tool", "go-agent-tool", 2},
		{"kitten", "sitting", 3},
	}
	for _, tc := range cases {
		if got := Distance(tc.a, tc.b); got != tc.want {
			t.Errorf("Distance(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestClosest(t *testing.T) {
	t.Parallel()
	candidates := []string{"none", "codemap"}
	got, ok := Closest("codemp", candidates, 2)
	if !ok || got != "codemap" {
		t.Fatalf("Closest() = (%q, %v), want (codemap, true)", got, ok)
	}
	if _, ok := Closest("wildly-different-value", candidates, 2); ok {
		t.Fatal("expected no suggestion for a distant value")
	}
	if _, ok := Closest("anything", nil, 2); ok {
		t.Fatal("expected no suggestion with no candidates")
	}
}
