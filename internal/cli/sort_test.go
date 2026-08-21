package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The normalized spec, not the raw flag value, must be what reaches Redmine.
func TestIssueListSendsNormalizedSort(t *testing.T) {
	var gotSort string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSort = r.URL.Query().Get("sort")
		json.NewEncoder(w).Encode(map[string]any{"issues": []any{}, "total_count": 0})
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "issue", "list", "--sort", "priority:desc, due_date:asc")

	if want := "priority:desc,due_date:asc"; gotSort != want {
		t.Errorf("sort sent as %q, want %q", gotSort, want)
	}
}

func TestTimeListSendsSort(t *testing.T) {
	var gotSort string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSort = r.URL.Query().Get("sort")
		json.NewEncoder(w).Encode(map[string]any{"time_entries": []any{}, "total_count": 0})
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "time", "list", "--sort", "spent_on:desc")

	if gotSort != "spent_on:desc" {
		t.Errorf("sort sent as %q, want spent_on:desc", gotSort)
	}
}

func TestListRejectsBadSort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request should not have been made")
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	if _, _, err := runCLIErr(t, "issue", "list", "--sort", "due_date:sideways"); err == nil {
		t.Error("expected an error for an invalid sort direction")
	}
}
