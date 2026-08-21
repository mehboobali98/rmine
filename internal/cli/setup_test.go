package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mehboobali98/rmine/internal/config"
)

func TestNormalizeServerURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://redmine.example.com", "https://redmine.example.com"},
		{"https://redmine.example.com/", "https://redmine.example.com"},
		{" https://redmine.example.com ", "https://redmine.example.com"},
		{"redmine.example.com", "https://redmine.example.com"}, // the common slip
	}
	for _, c := range cases {
		got, err := normalizeServerURL(c.in)
		if err != nil {
			t.Errorf("normalizeServerURL(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("normalizeServerURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	for _, in := range []string{"", "   ", "ftp://redmine.example.com", "https://"} {
		if _, err := normalizeServerURL(in); err == nil {
			t.Errorf("normalizeServerURL(%q): expected an error", in)
		}
	}
}

// Setup had no path that did not involve a terminal: the URL came from a
// prompt and the key from term.ReadPassword on the raw file descriptor, so a
// container or CI runner could not configure rmine at all.
func TestAddProfileFromEnvironment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Redmine-API-Key"); got != "key-from-env" {
			t.Errorf("API key header = %q, want key-from-env", got)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"user": map[string]any{"id": 1, "login": "ci", "firstname": "CI", "lastname": "Runner"},
		})
	}))
	defer srv.Close()

	setupTestProfile(t, srv) // isolates XDG_CONFIG_HOME
	t.Setenv("RMINE_URL", srv.URL)
	t.Setenv("RMINE_API_KEY", "key-from-env")

	out := runCLI(t, "config", "add-profile", "ci")
	if !strings.Contains(out, "ci") {
		t.Errorf("output = %q, want it to name the saved profile", out)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	saved, ok := cfg.Profiles["ci"]
	if !ok {
		t.Fatalf("profile not saved; have %v", cfg.Profiles)
	}
	if saved.URL != srv.URL || saved.APIKey != "key-from-env" {
		t.Errorf("saved profile = %+v, want the values from the environment", saved)
	}
}

func TestAddProfileRejectsMissingAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	setupTestProfile(t, srv)
	t.Setenv("RMINE_URL", srv.URL)
	t.Setenv("RMINE_API_KEY", "")

	// stdin is not a terminal under `go test`, so the key prompt reads EOF.
	if _, _, err := runCLIErr(t, "config", "add-profile", "nokey"); err == nil {
		t.Error("expected an error when no API key is available")
	}
}
