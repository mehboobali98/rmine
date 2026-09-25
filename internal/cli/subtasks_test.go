package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestIssueListFiltersByParent(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/web.json":
			json.NewEncoder(w).Encode(map[string]any{
				"project": map[string]any{"id": 7, "name": "Web", "identifier": "web"},
			})
		default:
			gotQuery = r.URL.Query()
			json.NewEncoder(w).Encode(map[string]any{"issues": []any{}, "total_count": 0})
		}
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "config", "set-default-project", "web")
	runCLI(t, "issue", "list", "--parent", "54744")

	if got := gotQuery.Get("parent_id"); got != "54744" {
		t.Errorf("parent_id = %q, want 54744", got)
	}
	if got := gotQuery.Get("project_id"); got != "" {
		t.Errorf("default project narrowed an explicit --parent: project_id=%q", got)
	}
}

func TestIssueListParentSurvivesSubjectSearch(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		json.NewEncoder(w).Encode(map[string]any{"issues": []any{}, "total_count": 0})
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "issue", "list", "--parent", "54744", "--subject", "sonar")

	if got := gotQuery.Get("v[parent_id][]"); got != "54744" {
		t.Errorf("v[parent_id][] = %q, want 54744 (query: %v)", got, gotQuery)
	}
	if got := gotQuery.Get("op[parent_id]"); got != "=" {
		t.Errorf("op[parent_id] = %q, want =", got)
	}
}

func TestIssueListRejectsNonNumericParent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request should not have been made")
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	if _, _, err := runCLIErr(t, "issue", "list", "--parent", "CMDB-7"); err == nil {
		t.Fatal("expected --parent CMDB-7 to be rejected")
	}
}
