package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mehboobali98/rmine/internal/config"
)

// defaultProjectServer resolves one project and records issue creations.
func defaultProjectServer(t *testing.T, body *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/web.json":
			json.NewEncoder(w).Encode(map[string]any{
				"project": map[string]any{"id": 7, "name": "Web", "identifier": "web"},
			})
		case "/projects.json":
			json.NewEncoder(w).Encode(map[string]any{
				"projects":    []map[string]any{{"id": 7, "name": "Web", "identifier": "web"}},
				"total_count": 1,
			})
		case "/issues.json":
			var wrapper map[string]map[string]any
			json.NewDecoder(r.Body).Decode(&wrapper)
			if body != nil {
				*body = wrapper["issue"]
			}
			json.NewEncoder(w).Encode(map[string]any{"issue": map[string]any{"id": 1, "subject": "ok"}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestSetDefaultProjectStoresIt(t *testing.T) {
	srv := defaultProjectServer(t, nil)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "config", "set-default-project", "web")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if got := cfg.Profiles["test"].DefaultProject; got != "web" {
		t.Errorf("default project = %q, want web", got)
	}
}

// Storing a typo would surface much later, on some unrelated create.
func TestSetDefaultProjectRejectsUnknownProject(t *testing.T) {
	srv := defaultProjectServer(t, nil)
	defer srv.Close()
	setupTestProfile(t, srv)

	if _, _, err := runCLIErr(t, "config", "set-default-project", "No Such Project"); err == nil {
		t.Fatal("expected an error for an unknown project")
	}

	cfg, _ := config.Load()
	if got := cfg.Profiles["test"].DefaultProject; got != "" {
		t.Errorf("a rejected project was stored anyway: %q", got)
	}
}

func TestSetDefaultProjectClears(t *testing.T) {
	srv := defaultProjectServer(t, nil)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "config", "set-default-project", "web")
	runCLI(t, "config", "set-default-project", "")

	cfg, _ := config.Load()
	if got := cfg.Profiles["test"].DefaultProject; got != "" {
		t.Errorf("default project = %q, want it cleared", got)
	}
}

func TestIssueCreateUsesDefaultProject(t *testing.T) {
	var got map[string]any
	srv := defaultProjectServer(t, &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "config", "set-default-project", "web")
	runCLI(t, "issue", "create", "--subject", "no project flag")

	if got["project_id"] != "7" {
		t.Errorf("project_id = %#v, want \"7\" from the profile default", got["project_id"])
	}
}

func TestIssueCreateWithoutProjectOrDefaultErrors(t *testing.T) {
	srv := defaultProjectServer(t, nil)
	defer srv.Close()
	setupTestProfile(t, srv)

	_, _, err := runCLIErr(t, "issue", "create", "--subject", "x")
	if err == nil {
		t.Fatal("expected an error with no --project and no default")
	}
	if !strings.Contains(err.Error(), "set-default-project") {
		t.Errorf("error should point at the default-project command, got: %v", err)
	}
}

// listProjectServer resolves one project and records the project_id each
// listing query was scoped to.
func listProjectServer(t *testing.T, gotProject *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/web.json":
			json.NewEncoder(w).Encode(map[string]any{
				"project": map[string]any{"id": 7, "name": "Web", "identifier": "web"},
			})
		case "/time_entries.json":
			*gotProject = r.URL.Query().Get("project_id")
			json.NewEncoder(w).Encode(map[string]any{"time_entries": []any{}, "total_count": 0})
		default:
			*gotProject = r.URL.Query().Get("project_id")
			json.NewEncoder(w).Encode(map[string]any{"issues": []any{}, "total_count": 0})
		}
	}))
}

func TestDefaultProjectScopesListings(t *testing.T) {
	var gotProject string
	srv := listProjectServer(t, &gotProject)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "config", "set-default-project", "web")

	for _, args := range [][]string{{"issue", "list"}, {"time", "list"}} {
		gotProject = ""
		runCLI(t, args...)
		if gotProject != "7" {
			t.Errorf("%v scoped to project_id=%q, want 7 from the profile default", args, gotProject)
		}
	}
}

// The scoping comes from stored configuration that is absent from the command
// the user typed, so it is announced — on stderr, so stdout stays parseable.
func TestDefaultProjectScopingIsAnnouncedOnStderr(t *testing.T) {
	var gotProject string
	srv := listProjectServer(t, &gotProject)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "config", "set-default-project", "web")

	stdout, stderr, err := runCLIErr(t, "issue", "list", "-o", "json")
	if err != nil {
		t.Fatalf("issue list: %v", err)
	}
	if !strings.Contains(stderr, "default project") || !strings.Contains(stderr, "--all-projects") {
		t.Errorf("stderr should explain the scoping and how to widen it, got: %q", stderr)
	}
	if strings.Contains(stdout, "default project") {
		t.Errorf("the notice leaked into stdout, breaking JSON output: %q", stdout)
	}
}

func TestAllProjectsIgnoresTheDefault(t *testing.T) {
	var gotProject string
	srv := listProjectServer(t, &gotProject)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "config", "set-default-project", "web")

	gotProject = "unset"
	runCLI(t, "issue", "list", "--all-projects")
	if gotProject != "" {
		t.Errorf("--all-projects still scoped to project_id=%q", gotProject)
	}
}

// An explicit --project alongside --all-projects asks for two contradictory
// things; picking one silently would be a guess.
func TestProjectAndAllProjectsConflict(t *testing.T) {
	var gotProject string
	srv := listProjectServer(t, &gotProject)
	defer srv.Close()
	setupTestProfile(t, srv)

	_, _, err := runCLIErr(t, "issue", "list", "--project", "web", "--all-projects")
	if err == nil {
		t.Fatal("expected an error when --project and --all-projects are combined")
	}
	if !strings.Contains(err.Error(), "--all-projects") {
		t.Errorf("error should name the conflicting flags, got: %v", err)
	}
}

// With no default configured, a listing is unscoped as before.
func TestListingsUnscopedWithoutADefault(t *testing.T) {
	var gotProject string
	srv := listProjectServer(t, &gotProject)
	defer srv.Close()
	setupTestProfile(t, srv)

	gotProject = "unset"
	runCLI(t, "issue", "list")
	if gotProject != "" {
		t.Errorf("issue list scoped to project_id=%q with no default set", gotProject)
	}
}
