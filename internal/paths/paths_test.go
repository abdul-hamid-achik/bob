package paths

import (
	"path/filepath"
	"testing"
)

func TestResolveDefaultsUseStrictXDGSplit(t *testing.T) {
	home := t.TempDir()
	layout, err := resolve(home, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}

	want := Layout{
		ConfigDir:  filepath.Join(home, ".config", "bob"),
		ConfigFile: filepath.Join(home, ".config", "bob", "config.yaml"),
		DataDir:    filepath.Join(home, ".local", "share", "bob"),
		StateDir:   filepath.Join(home, ".local", "state", "bob"),
		CacheDir:   filepath.Join(home, ".cache", "bob"),
	}
	if layout != want {
		t.Fatalf("layout = %#v, want %#v", layout, want)
	}
}

func TestResolveHonorsXDGAndBobOverrides(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	env := map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(root, "xdg-config"),
		"XDG_DATA_HOME":   filepath.Join(root, "xdg-data"),
		"XDG_STATE_HOME":  filepath.Join(root, "xdg-state"),
		"XDG_CACHE_HOME":  filepath.Join(root, "xdg-cache"),
		"BOB_CONFIG_DIR":  filepath.Join(root, "config"),
		"BOB_DATA_DIR":    filepath.Join(root, "data"),
		"BOB_STATE_DIR":   filepath.Join(root, "state"),
		"BOB_CACHE_DIR":   filepath.Join(root, "cache"),
		"BOB_CONFIG":      filepath.Join(root, "elsewhere", "bob.yaml"),
	}

	layout, err := resolve(home, func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if layout.ConfigDir != env["BOB_CONFIG_DIR"] || layout.ConfigFile != env["BOB_CONFIG"] {
		t.Fatalf("config paths = %#v", layout)
	}
	if layout.DataDir != env["BOB_DATA_DIR"] || layout.StateDir != env["BOB_STATE_DIR"] || layout.CacheDir != env["BOB_CACHE_DIR"] {
		t.Fatalf("Bob overrides not honored: %#v", layout)
	}
}

func TestResolveUsesXDGWhenBobOverridesAreUnset(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	env := map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(root, "config"),
		"XDG_DATA_HOME":   filepath.Join(root, "data"),
		"XDG_STATE_HOME":  filepath.Join(root, "state"),
		"XDG_CACHE_HOME":  filepath.Join(root, "cache"),
	}
	layout, err := resolve(home, func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if layout.ConfigDir != filepath.Join(env["XDG_CONFIG_HOME"], "bob") ||
		layout.DataDir != filepath.Join(env["XDG_DATA_HOME"], "bob") ||
		layout.StateDir != filepath.Join(env["XDG_STATE_HOME"], "bob") ||
		layout.CacheDir != filepath.Join(env["XDG_CACHE_HOME"], "bob") {
		t.Fatalf("XDG paths not honored: %#v", layout)
	}
}

func TestResolveRejectsRelativeOverrides(t *testing.T) {
	for _, name := range []string{
		"BOB_CONFIG", "BOB_CONFIG_DIR", "BOB_DATA_DIR", "BOB_STATE_DIR", "BOB_CACHE_DIR",
		"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := resolve(t.TempDir(), func(key string) string {
				if key == name {
					return "relative/path"
				}
				return ""
			})
			if err == nil {
				t.Fatalf("expected %s to be rejected", name)
			}
		})
	}
}

func TestResolveDoesNotCreateDirectories(t *testing.T) {
	home := t.TempDir()
	layout, err := resolve(home, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{layout.ConfigDir, layout.DataDir, layout.StateDir, layout.CacheDir} {
		if _, err := filepath.Abs(path); err != nil {
			t.Fatal(err)
		}
		if matches, err := filepath.Glob(path); err != nil || len(matches) != 0 {
			t.Fatalf("path %q was unexpectedly created", path)
		}
	}
}
