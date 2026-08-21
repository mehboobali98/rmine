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

// A stored default must not silently narrow a search: the same command would
// then mean different things depending on configuration the user cannot see.
func TestDefaultProjectDoesNotFilterListings(t *testing.T) {
	var gotProject string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/web.json":
			json.NewEncoder(w).Encode(map[string]any{
				"project": map[string]any{"id": 7, "name": "Web", "identifier": "web"},
			})
		default:
			gotProject = r.URL.Query().Get("project_id")
			json.NewEncoder(w).Encode(map[string]any{"issues": []any{}, "total_count": 0})
		}
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "config", "set-default-project", "web")
	runCLI(t, "issue", "list")

	if gotProject != "" {
		t.Errorf("issue list was scoped to project_id=%q by a stored default", gotProject)
	}
}
