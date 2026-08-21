package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// assigneeServer serves a project, its members, the current user, and
// records whatever issue body it is sent.
func assigneeServer(t *testing.T, body *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/users/current.json":
			json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"id": 1, "login": "me"}})
		case r.URL.Path == "/projects/web.json":
			json.NewEncoder(w).Encode(map[string]any{
				"project": map[string]any{"id": 7, "name": "Web", "identifier": "web"},
			})
		case strings.HasSuffix(r.URL.Path, "/memberships.json"):
			json.NewEncoder(w).Encode(map[string]any{
				"memberships": []map[string]any{
					{"id": 1, "user": map[string]any{"id": 3, "name": "Jane Doe"}},
				},
				"total_count": 1,
			})
		case r.URL.Path == "/issues/1234.json" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{
				"issue": map[string]any{"id": 1234, "project": map[string]any{"id": 7, "name": "Web"}},
			})
		default:
			var wrapper map[string]map[string]any
			json.NewDecoder(r.Body).Decode(&wrapper)
			if issue, ok := wrapper["issue"]; ok {
				*body = issue
			}
			json.NewEncoder(w).Encode(map[string]any{"issue": map[string]any{"id": 1, "subject": "ok"}})
		}
	}))
}

func TestIssueCreateAssignsByName(t *testing.T) {
	var got map[string]any
	srv := assigneeServer(t, &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "issue", "create", "--project", "web", "--subject", "x", "--assignee", "Jane Doe")

	if got["assigned_to_id"] != float64(3) {
		t.Errorf("assigned_to_id = %#v, want 3", got["assigned_to_id"])
	}
}

// A write needs a real ID: Redmine understands "me" in a filter but not in
// an issue payload.
func TestIssueCreateResolvesMeToAnID(t *testing.T) {
	var got map[string]any
	srv := assigneeServer(t, &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "issue", "create", "--project", "web", "--subject", "x", "--assignee", "me")

	if got["assigned_to_id"] != float64(1) {
		t.Errorf("assigned_to_id = %#v, want the current user's id 1", got["assigned_to_id"])
	}
}

// On update the project comes from the issue itself.
func TestIssueUpdateAssignsByName(t *testing.T) {
	var got map[string]any
	srv := assigneeServer(t, &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "issue", "update", "1234", "--assignee", "Jane Doe")

	if got["assigned_to_id"] != float64(3) {
		t.Errorf("assigned_to_id = %#v, want 3", got["assigned_to_id"])
	}
}

func TestIssueUpdateUnassignStillClears(t *testing.T) {
	var got map[string]any
	srv := assigneeServer(t, &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "issue", "update", "1234", "--assignee", "0")

	if got["assigned_to_id"] != "" {
		t.Errorf("assigned_to_id = %#v, want an empty string to unassign", got["assigned_to_id"])
	}
}

func TestIssueListFiltersByAssigneeName(t *testing.T) {
	var gotAssignee string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/projects/web.json":
			json.NewEncoder(w).Encode(map[string]any{
				"project": map[string]any{"id": 7, "name": "Web", "identifier": "web"},
			})
		case strings.HasSuffix(r.URL.Path, "/memberships.json"):
			json.NewEncoder(w).Encode(map[string]any{
				"memberships": []map[string]any{{"id": 1, "user": map[string]any{"id": 3, "name": "Jane Doe"}}},
				"total_count": 1,
			})
		default:
			gotAssignee = r.URL.Query().Get("assigned_to_id")
			json.NewEncoder(w).Encode(map[string]any{"issues": []any{}, "total_count": 0})
		}
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "issue", "list", "--project", "web", "--assignee", "Jane Doe")

	if gotAssignee != "3" {
		t.Errorf("assigned_to_id = %q, want 3", gotAssignee)
	}
}
