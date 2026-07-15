package recipe

import (
	"reflect"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

func TestMatchExtensionPointsUsesWholePathTemplatesAndForbiddenPaths(t *testing.T) {
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	metadata, err := ResolveMetadata(m)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		path string
		want []string
	}{
		{"internal/domain/service.go", []string{"domain.packages"}},
		{"internal/domain/service_test.go", []string{"domain.packages"}},
		{"internal/cli/hello.go", []string{"cli.command_files"}},
		{"internal/cli/hello_test.go", []string{"cli.command_files"}},
		{"internal/cli/root.go", []string{}},
		{"internal/cli/registry.go", []string{}},
		{"internal/domain/nested/service.go", []string{}},
		{"internal/domain/service.txt", []string{}},
	}
	for _, tc := range tests {
		matches := MatchExtensionPoints(metadata, tc.path)
		ids := []string{}
		for _, match := range matches {
			ids = append(ids, match.ID)
		}
		if !reflect.DeepEqual(ids, tc.want) {
			t.Fatalf("%s = %#v, want %#v", tc.path, ids, tc.want)
		}
	}
}
