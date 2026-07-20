package version

import "testing"

func TestDefaultSentinelValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Version", Version, "dev"},
		{"Commit", Commit, "none"},
		{"Date", Date, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Errorf("%s = %q; want sentinel %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestVariablesAreNonEmpty(t *testing.T) {
	t.Parallel()
	for name, value := range map[string]string{
		"Version": Version,
		"Commit":  Commit,
		"Date":    Date,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if value == "" {
				t.Errorf("%s is empty; expected a non-empty string", name)
			}
		})
	}
}
