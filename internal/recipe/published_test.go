package recipe

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

// publishedGoldens pins the artifact digest of every published recipe contract
// at its current version, across the manifest variants that change output. A
// digest covers each artifact's path, mode, seed flag, and exact content, so
// any template edit that alters published bytes fails here. Changing a value
// is never a routine fix: publish a new recipe version instead.
var publishedGoldens = map[string]string{
	"files@1/default":                          "ef66e2736fed3d2558f0d6facac064a9377cf892fd8a0264782db0359d3d22b0",
	"go-agent-tool@6/default":                  "13edacf3bbab2295521bb7b3a61f9d4f0493cea1ba29bf6b69f820092f14e325",
	"python-tool@1/default":                    "d324f5ffd1c2967eb02f123e55e03c9f78a55b7359614a848aa09fa345cf33a4",
	"rust-tool@1/default":                      "847db1457716c4dd626101b3d3b6464348ebb389a9b44cce8fe44c191ea9546b",
	"ts-tool@1/default":                        "6095b3c7a5e7590c57062f9badd6af35bc667b9e23fca24a12a0cc53db3458e2",
	"go-agent-tool@6/full":                     "fb94326b50520ccf535a6dc61918a69031dbd377015a2dda11fcc269b1a706f5",
	"go-agent-tool@6/no-actions-no-goreleaser": "507f22189d6d20c8feac650690339d7f7c8def9408da93d29bf7ea6a8270cd9d",
	"go-agent-tool@6/no-actions-goreleaser":    "f5c1be4b7705f7e9791f8e44647885f5efa13937479e5634ab47cbc7b30d5180",
	"go-hygiene@2/default":                     "9c95ddfa81352eaa3573303ba1bc53fdb3da226864cbf66ab52b94b3f1f7b2c2",
	"go-hygiene@2/kind-lib":                    "9c95ddfa81352eaa3573303ba1bc53fdb3da226864cbf66ab52b94b3f1f7b2c2",
	"go-hygiene@2/kind-app":                    "9c95ddfa81352eaa3573303ba1bc53fdb3da226864cbf66ab52b94b3f1f7b2c2",
	"java-app@2/default":                       "e24563dbc137592c83687602cdf1ad3d03040ba9c0c4cf10bfa749a42a8e7225",
	"java-app@2/kind-library":                  "e24563dbc137592c83687602cdf1ad3d03040ba9c0c4cf10bfa749a42a8e7225",
	"cpp-cmake@2/default":                      "96e94a70c34bb8d062c1a5543f907b02dea3e0b1f32d0662ab6d5c8bf935a82f",
	"cpp-cmake@2/kind-library":                 "96e94a70c34bb8d062c1a5543f907b02dea3e0b1f32d0662ab6d5c8bf935a82f",
	"docker-app@2/default":                     "1264f74000546e0c2fc784d3b2d097da41f07b3258db7863edca3833ac0506f9",
	"nix-app@2/default":                        "65b045c8c14873b4b084e934a3132fcb56783dcfc8c8ba21a4f7488d81a7a0f9",
	"nix-app@2/kind-package":                   "65b045c8c14873b4b084e934a3132fcb56783dcfc8c8ba21a4f7488d81a7a0f9",
	"terraform-app@2/default":                  "acf5b96776e6d20f6ff02a47648838097f1248490fb13bc71acd004e6e7e3815",
	"terraform-app@2/kind-stack":               "acf5b96776e6d20f6ff02a47648838097f1248490fb13bc71acd004e6e7e3815",
	"deno-app@2/default":                       "4ba81769fda39e8765b9aa3b794da36adea14cc3189969beef98cd55840189a0",
	"deno-app@2/kind-library":                  "4ba81769fda39e8765b9aa3b794da36adea14cc3189969beef98cd55840189a0",
	"dotnet-app@2/default":                     "abebbfb0f9542947feb33f57427644968bd50493556931b2c3479ff94867d582",
	"dotnet-app@2/kind-library":                "abebbfb0f9542947feb33f57427644968bd50493556931b2c3479ff94867d582",
	"kotlin-app@2/default":                     "56af62d7826f8368e98ce7881408505448f34cfb51fbe0189ef078d4a87e9596",
	"kotlin-app@2/kind-library":                "56af62d7826f8368e98ce7881408505448f34cfb51fbe0189ef078d4a87e9596",
	"php-app@2/default":                        "815027f7281f111ae7e8d4570743f4397a16bc95c4fb73eb16b74fddc38c0408",
	"php-app@2/kind-library":                   "815027f7281f111ae7e8d4570743f4397a16bc95c4fb73eb16b74fddc38c0408",
	"js-app@2/default":                         "34bc930637961d20c422ead5d07bdf85ba97551f4bbeb408293f3e4b94c9a763",
	"js-app@2/kind-monorepo":                   "34bc930637961d20c422ead5d07bdf85ba97551f4bbeb408293f3e4b94c9a763",
	"js-app@2/pm-bun":                          "83612e06a98ea841d9bf24cfa793077d3cc5645eb1727d27099847f2f0302a56",
	"lua-lib@2/default":                        "cfea73d8619afd707f24904f6e373edc59ffe7f00a4c2023aab53a88384af618",
	"lua-lib@2/kind-plugin":                    "01cd5c1fd0042a07977b2421ce8afd5d636de455af9dd31a209bb6c88a19f5a9",
	"nuxt-app@2/default":                       "713e8c1b05d747da48719b1822a3d38d03fc8351384b461808a93462f156c77f",
	"nuxt-app@2/pm-npm":                        "a67664cb2a0a20e14bcbaebb4360900021f03d3aac0f495c90ec7f9d4689c7e3",
	"python-app@2/default":                     "116968c48992cc11cc01b774887a46f3dd141d0774c9a833c57bfa5ce9c33f99",
	"ruby-app@2/default":                       "ac386edf0602ae67b5865c0aac44a0d2c36bb353697c2d071f591713ba8df213",
	"ruby-app@2/kind-gem":                      "a4e5b7e1e8f895a82b2d36d8a7ce59cec83607fe6683365a8aed5bbb737f6ea2",
	"rust-cli@2/default":                       "3d52a23c701159b548913adce01988356fd3ea90b73bc7a31bfc09731a0e9be8",
	"rust-cli@2/kind-lib":                      "3d52a23c701159b548913adce01988356fd3ea90b73bc7a31bfc09731a0e9be8",
	"rust-cli@2/kind-workspace":                "3d52a23c701159b548913adce01988356fd3ea90b73bc7a31bfc09731a0e9be8",
	"static-web@2/default":                     "3adae01cd2f027252981b731d61c43cde376690cd9915a3147aaaa0e69e64d12",
	"swift-package@2/default":                  "3ad0a2c3d5777a47f986fe838e026f8d7208fc34baca075ae28c5b165e7b5e14",
	"elixir-app@2/default":                     "74fe805dd9b49014c15ac7aa79b4b40a32819e14be948d54e8335651669e0a75",
	"elixir-app@2/kind-umbrella":               "74fe805dd9b49014c15ac7aa79b4b40a32819e14be948d54e8335651669e0a75",
	"ts-app@2/default":                         "8c199039564618509d63a3319473f121ba8467d4b41ba3b19618997e6887bec6",
	"ts-app@2/kind-monorepo":                   "8c199039564618509d63a3319473f121ba8467d4b41ba3b19618997e6887bec6",
	"ts-app@2/pm-pnpm":                         "4c0678d765ed277bcef2fd7587ee83a7c977f9f859c1fff0895f1f524b927a25",
	"vue-app@2/default":                        "310f1f36832c1da92ac54c178b5b7fd07a28c0ff515dace3178e13bb4ba03f51",
	"vue-app@2/pm-yarn":                        "11da8d2491c8d4c51fb5357ee632c75cfdb851aa3d3da275e0d07e43e5f22be8",
}

type publishedFixture struct {
	id      string
	variant string
	build   func() (manifest.Manifest, error)
}

// publishedFixtures returns one fixed, representative manifest per pinned
// variant. Every value is literal so the digests depend only on the recipe.
func publishedFixtures() []publishedFixture {
	fixtures := []publishedFixture{
		{id: "files", variant: "default", build: func() (manifest.Manifest, error) {
			return manifest.Manifest{
				SchemaVersion: manifest.SchemaVersion,
				Recipe:        manifest.RecipeFiles,
				Product:       manifest.Product{Name: "demo-app", Description: "A demo app"},
				Vars:          map[string]string{"name": "bob", "port": "8080"},
				Files: []manifest.FileDecl{
					{Path: "config/app.env", Mode: "0600", Content: "NAME=${vars.name}\nPORT=${vars.port}\n"},
					{Path: "docs/hello.txt", Content: "hello ${vars.name}\n"},
					{Path: "run.sh", Mode: "0755", Content: "echo ${FOO} ${vars.name}\n"},
				},
			}, nil
		}},
		{id: "go-agent-tool", variant: "default", build: func() (manifest.Manifest, error) {
			return manifest.Default("acme-tool", "github.com/acme/acme-tool", "Build useful things."), nil
		}},
		{id: "rust-tool", variant: "default", build: func() (manifest.Manifest, error) {
			return manifest.DefaultRustTool("acme-tool", "github.com/acme/acme-tool", "Build useful things."), nil
		}},
		{id: "python-tool", variant: "default", build: func() (manifest.Manifest, error) {
			return manifest.DefaultPythonTool("acme-tool", "github.com/acme/acme-tool", "Build useful things."), nil
		}},
		{id: "ts-tool", variant: "default", build: func() (manifest.Manifest, error) {
			return manifest.DefaultTSTool("acme-tool", "github.com/acme/acme-tool", "Build useful things."), nil
		}},
		{id: "go-agent-tool", variant: "full", build: func() (manifest.Manifest, error) {
			m := manifest.Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
			m.Integrations.BrowserVerification = "cairntrace"
			m.Integrations.Secrets = "tinyvault"
			m.Integrations.Artifacts = "fcheap"
			m.Distribution.Homebrew = true
			m.Distribution.Docs = "markdown"
			return m, nil
		}},
		{id: "go-agent-tool", variant: "no-actions-no-goreleaser", build: func() (manifest.Manifest, error) {
			m := manifest.Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
			m.Distribution.GitHubActions = false
			m.Distribution.GoReleaser = false
			return m, nil
		}},
		{id: "go-agent-tool", variant: "no-actions-goreleaser", build: func() (manifest.Manifest, error) {
			m := manifest.Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
			m.Distribution.GitHubActions = false
			return m, nil
		}},
	}
	for _, id := range manifest.StackRecipeIDs() {
		runtime, _ := manifest.StackRecipeRuntime(id)
		fixtures = append(fixtures, publishedFixture{id: id, variant: "default", build: stackFixture(id, "")})
		for _, kind := range runtime.Kinds[1:] {
			fixtures = append(fixtures, publishedFixture{id: id, variant: "kind-" + kind, build: stackFixture(id, kind)})
		}
	}
	for _, pm := range []struct{ id, manager string }{
		{manifest.RecipeTSApp, "pnpm"},
		{manifest.RecipeJSApp, "bun"},
		{manifest.RecipeVueApp, "yarn"},
		{manifest.RecipeNuxtApp, "npm"},
	} {
		fixtures = append(fixtures, publishedFixture{id: pm.id, variant: "pm-" + pm.manager, build: func() (manifest.Manifest, error) {
			m, err := manifest.DefaultStack(pm.id, "demo", "", "A local-first, agent-ready repository.", "")
			m.Runtime.PackageManager = pm.manager
			return m, err
		}})
	}
	return fixtures
}

func stackFixture(id, kind string) func() (manifest.Manifest, error) {
	return func() (manifest.Manifest, error) {
		return manifest.DefaultStack(id, "demo", "", "A local-first, agent-ready repository.", kind)
	}
}

// publishedDigest hashes path, mode, seed flag, and content of every artifact
// in order. Lengths are prefixed so content bytes cannot blur field boundaries.
func publishedDigest(artifacts []Artifact) string {
	h := sha256.New()
	for _, artifact := range artifacts {
		_, _ = fmt.Fprintf(h, "%s\x00%o\x00%t\x00%d\x00", artifact.Path, artifact.Mode, artifact.Seed, len(artifact.Content))
		_, _ = h.Write(artifact.Content)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// publishedKey names a golden for one fixture at the version the product
// currently renders for that recipe id.
func publishedKey(id, variant string) (string, error) {
	version, err := Version(id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s@%d/%s", id, version, variant), nil
}

// unpinnedRecipes reports the "id@version/default" golden key of every
// built-in recipe that has no pinned default contract.
func unpinnedRecipes(goldens map[string]string) []string {
	var missing []string
	for _, id := range IDs() {
		key, err := publishedKey(id, "default")
		if err != nil {
			missing = append(missing, id)
			continue
		}
		if _, ok := goldens[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing
}

func TestPublishedRecipeContractsMatchGoldens(t *testing.T) {
	t.Parallel()
	seen := make(map[string]struct{})
	for _, fixture := range publishedFixtures() {
		key, err := publishedKey(fixture.id, fixture.variant)
		if err != nil {
			t.Fatal(err)
		}
		seen[key] = struct{}{}
		want, ok := publishedGoldens[key]
		if !ok {
			t.Errorf("%s has no published golden; pin it before shipping", key)
			continue
		}
		m, err := fixture.build()
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		artifacts, err := Render(m)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if got := publishedDigest(artifacts); got != want {
			t.Errorf("%s digest = %s, want %s", key, got, want)
		}
	}
	for key := range publishedGoldens {
		if _, ok := seen[key]; !ok {
			t.Errorf("golden %s has no fixture; remove it or restore the variant", key)
		}
	}
}

func TestPublishedRecipeEveryIDHasGolden(t *testing.T) {
	t.Parallel()
	if missing := unpinnedRecipes(publishedGoldens); len(missing) > 0 {
		t.Fatalf("built-in recipes without a published golden: %v", missing)
	}
}

func TestPublishedRecipeGuardDetectsMissingGolden(t *testing.T) {
	t.Parallel()
	goldens := make(map[string]string, len(publishedGoldens))
	for key, digest := range publishedGoldens {
		goldens[key] = digest
	}
	removed := "ts-app@2/default"
	delete(goldens, removed)
	missing := unpinnedRecipes(goldens)
	if len(missing) != 1 || missing[0] != removed {
		t.Fatalf("unpinnedRecipes after removing %s = %v, want exactly [%s]", removed, missing, removed)
	}
}
