package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// timeLogServer records the time entry body it is sent.
func timeLogServer(t *testing.T, body *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects.json":
			json.NewEncoder(w).Encode(map[string]any{
				"projects":    []map[string]any{{"id": 7, "name": "Web", "identifier": "web"}},
				"total_count": 1,
			})
		case "/projects/web.json":
			json.NewEncoder(w).Encode(map[string]any{
				"project": map[string]any{"id": 7, "name": "Web", "identifier": "web"},
			})
		case "/time_entries.json":
			var wrapper struct {
				TimeEntry map[string]any `json:"time_entry"`
			}
			if err := json.NewDecoder(r.Body).Decode(&wrapper); err != nil {
				t.Errorf("decoding body: %v", err)
			}
			*body = wrapper.TimeEntry
			json.NewEncoder(w).Encode(map[string]any{
				"time_entry": map[string]any{
					"id": 99, "hours": 2.0,
					"project": map[string]any{"id": 7, "name": "Web"},
				},
			})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
}

// CreateTimeEntryRequest carried a ProjectID that no flag could reach, so
// time not attached to a ticket — meetings, planning, support rotations —
// could not be logged from the CLI at all.
func TestTimeLogAgainstAProject(t *testing.T) {
	var got map[string]any
	srv := timeLogServer(t, &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "time", "log", "--project", "web", "--hours", "2")

	if got["project_id"] != "7" {
		t.Errorf("project_id = %#v, want \"7\"", got["project_id"])
	}
	if _, ok := got["issue_id"]; ok {
		t.Errorf("issue_id should not be sent when logging to a project: %v", got)
	}
	if !strings.Contains(out, "project Web") {
		t.Errorf("output = %q, want it to name the project", out)
	}
}

func TestTimeLogAgainstAnIssueStillWorks(t *testing.T) {
	var got map[string]any
	srv := timeLogServer(t, &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "time", "log", "42", "--hours", "2")

	if got["issue_id"] != float64(42) {
		t.Errorf("issue_id = %#v, want 42", got["issue_id"])
	}
	if _, ok := got["project_id"]; ok {
		t.Errorf("project_id should not be sent when logging to an issue: %v", got)
	}
	if !strings.Contains(out, "issue #42") {
		t.Errorf("output = %q, want it to name the issue", out)
	}
}

// Redmine attaches an entry to exactly one of the two, so catch the
// ambiguity here rather than letting the server decide.
func TestTimeLogRequiresExactlyOneTarget(t *testing.T) {
	var got map[string]any
	srv := timeLogServer(t, &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	cases := []struct {
		name string
		args []string
	}{
		{"neither", []string{"time", "log", "--hours", "2"}},
		{"both", []string{"time", "log", "42", "--project", "web", "--hours", "2"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := runCLIErr(t, c.args...)
			if err == nil {
				t.Fatalf("%v: expected an error", c.args)
			}
			if !strings.Contains(err.Error(), "--project") {
				t.Errorf("error should mention --project, got: %v", err)
			}
		})
	}
}
