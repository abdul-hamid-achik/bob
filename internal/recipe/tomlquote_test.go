package recipe

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

func TestTomlQuoteMatchesStrconvQuoteForPrintableASCII(t *testing.T) {
	t.Parallel()
	corpus := []string{
		"",
		"demo",
		"a demo repository",
		`quote " and backslash \ together`,
		`"leading quote`,
		`trailing quote"`,
		`\leading backslash`,
		`trailing backslash\`,
		`multiple ""quotes"" and \\backslashes\\`,
		"punctuation: !@#$%^&*()_+-=[]{}|;:,.<>/?`~",
		"digits 0123456789 and spaces   here",
		"single' quote and back`tick",
	}
	for _, s := range corpus {
		got := tomlQuote(s)
		want := strconv.Quote(s)
		if got != want {
			t.Fatalf("tomlQuote(%q) = %s, want byte-identical strconv.Quote result %s", s, got, want)
		}
	}
}

// uEscape builds the text of a TOML basic-string uHHHH escape (uppercase
// hex, 4 digits, no surrounding quotes) for rune r.
func uEscape(r rune) string {
	return fmt.Sprintf("%c%c%04X", '\\', 'u', r)
}

func TestTomlQuoteEscapesControlCharacters(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"bell (no short TOML escape)", string(rune(0x07)), `"` + uEscape(0x07) + `"`},
		{"vertical tab (no short TOML escape)", string(rune(0x0B)), `"` + uEscape(0x0B) + `"`},
		{"backspace", "\b", `"\b"`},
		{"tab", "\t", `"\t"`},
		{"newline", "\n", `"\n"`},
		{"form feed", "\f", `"\f"`},
		{"carriage return", "\r", `"\r"`},
		{"null", string(rune(0x00)), `"` + uEscape(0x00) + `"`},
		{"unit separator", string(rune(0x1F)), `"` + uEscape(0x1F) + `"`},
		{"del", string(rune(0x7F)), `"` + uEscape(0x7F) + `"`},
		{"double quote", `"`, `"\""`},
		{"backslash", `\`, `"\\"`},
		{
			"mixed",
			"note " + string(rune(0x07)) + " end" + string(rune(0x0B)) + ".",
			`"note ` + uEscape(0x07) + ` end` + uEscape(0x0B) + `."`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tomlQuote(tc.in)
			if got != tc.want {
				t.Fatalf("tomlQuote(%q) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestTomlQuotePassesThroughPrintableNonASCII(t *testing.T) {
	t.Parallel()
	in := "cafe et resume, avec des mots non-ASCII: aeiou nihongo"
	got := tomlQuote(in)
	want := `"` + in + `"`
	if got != want {
		t.Fatalf("tomlQuote(%q) = %s, want %s (non-ASCII printable runes must pass through unchanged)", in, got, want)
	}
}

func TestTomlQuoteReplacesInvalidUTF8WithReplacementChar(t *testing.T) {
	t.Parallel()
	in := "before" + string([]byte{0xff}) + "after"
	got := tomlQuote(in)
	want := "\"before" + string(rune(0xFFFD)) + "after\""
	if got != want {
		t.Fatalf("tomlQuote(%q) = %s, want %s", in, got, want)
	}

	in2 := "x" + string([]byte{0xe2, 0x82}) + "y"
	got2 := tomlQuote(in2)
	if !strings.ContainsRune(got2, rune(0xFFFD)) {
		t.Fatalf("tomlQuote(%q) = %s, want it to contain U+FFFD for the invalid sequence", in2, got2)
	}
	if strings.IndexByte(got2, 0xe2) >= 0 || strings.IndexByte(got2, 0x82) >= 0 {
		t.Fatalf("tomlQuote(%q) = %s, want the raw invalid bytes replaced, not passed through", in2, got2)
	}
}

func TestTomlQuoteWrapsEmptyString(t *testing.T) {
	t.Parallel()
	if got := tomlQuote(""); got != `""` {
		t.Fatalf("tomlQuote(\"\") = %s, want an empty quoted string", got)
	}
}

var tomlEscapeSequence = regexp.MustCompile(`\\(["\\btnfr]|u[0-9A-Fa-f]{4}|U[0-9A-Fa-f]{8})`)

func assertValidTOMLBasicStringEscapes(t *testing.T, label, content string) {
	t.Helper()
	remaining := content
	for {
		idx := strings.IndexByte(remaining, '\\')
		if idx == -1 {
			break
		}
		tail := remaining[idx:]
		loc := tomlEscapeSequence.FindStringIndex(tail)
		if loc == nil || loc[0] != 0 {
			t.Fatalf("%s: invalid TOML escape at %q (backslash not followed by one of btnfr\"\\uU)", label, truncateForError(tail))
		}
		remaining = tail[loc[1]:]
	}
	for _, r := range content {
		if r == rune(0x07) || r == rune(0x0B) {
			t.Fatalf("%s: raw bell/vtab control character found unescaped in rendered content", label)
		}
	}
}

func truncateForError(s string) string {
	const max = 40
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

func TestRenderPythonAppEscapesAdversarialDescriptionForToml(t *testing.T) {
	t.Parallel()
	adversarial := "note " + string(rune(0x07)) + " bell " + string(rune(0x0B)) + " vtab \ttab \" quote \\ backslash end"

	m, err := manifest.DefaultStack(manifest.RecipePythonApp, "demo", "", adversarial, "")
	if err != nil {
		t.Fatalf("manifest.DefaultStack: %v", err)
	}

	v1, err := RenderVersion(m, 1)
	if err != nil {
		t.Fatalf("RenderVersion(1): %v", err)
	}
	assertValidTOMLBasicStringEscapes(t, "pyproject.toml@v1", pyprojectContent(t, v1))

	v2, err := Render(m)
	if err != nil {
		t.Fatalf("Render (latest): %v", err)
	}
	assertValidTOMLBasicStringEscapes(t, "pyproject.toml@v2", pyprojectContent(t, v2))
}

func pyprojectContent(t *testing.T, artifacts []Artifact) string {
	t.Helper()
	for _, artifact := range artifacts {
		if artifact.Path == "pyproject.toml" {
			return string(artifact.Content)
		}
	}
	t.Fatal("pyproject.toml artifact not found")
	return ""
}
