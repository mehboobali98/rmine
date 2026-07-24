package config

import (
	"os"
	"path/filepath"
	"testing"
)

func withXDGConfigHome(t *testing.T, dir string) {
	t.Helper()
	prevXDG, hadXDG := os.LookupEnv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", dir)
	t.Cleanup(func() {
		if hadXDG {
			os.Setenv("XDG_CONFIG_HOME", prevXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	})
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	withXDGConfigHome(t, t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.Profiles["default"] = Profile{URL: "https://redmine.example.com", APIKey: "secret"}
	cfg.CurrentProfile = "default"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config file mode = %v, want 0600", perm)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.CurrentProfile != "default" {
		t.Errorf("CurrentProfile = %q, want default", reloaded.CurrentProfile)
	}
	got := reloaded.Profiles["default"]
	if got.URL != "https://redmine.example.com" || got.APIKey != "secret" {
		t.Errorf("unexpected profile: %+v", got)
	}
}

func TestResolvePrecedence(t *testing.T) {
	dir := t.TempDir()
	withXDGConfigHome(t, dir)

	cfg := &Config{
		CurrentProfile: "default",
		Profiles: map[string]Profile{
			"default": {URL: "https://default.example.com"},
			"work":    {URL: "https://work.example.com"},
		},
		path: filepath.Join(dir, "rmine", "config.yml"),
	}

	// No override: falls back to CurrentProfile.
	p, err := cfg.Resolve("")
	if err != nil || p.URL != "https://default.example.com" {
		t.Fatalf("Resolve(\"\") = %+v, %v", p, err)
	}

	// Env var overrides CurrentProfile.
	os.Setenv("RMINE_PROFILE", "work")
	t.Cleanup(func() { os.Unsetenv("RMINE_PROFILE") })
	p, err = cfg.Resolve("")
	if err != nil || p.URL != "https://work.example.com" {
		t.Fatalf("Resolve with env var = %+v, %v", p, err)
	}

	// Explicit flag overrides everything.
	p, err = cfg.Resolve("default")
	if err != nil || p.URL != "https://default.example.com" {
		t.Fatalf("Resolve with flag = %+v, %v", p, err)
	}

	// Unknown profile is an error.
	if _, err := cfg.Resolve("nope"); err == nil {
		t.Fatal("expected error for unknown profile")
	}
}
