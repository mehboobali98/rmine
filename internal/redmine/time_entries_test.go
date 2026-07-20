package redmine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSpentOnRange(t *testing.T) {
	cases := []struct {
		from, to, want string
	}{
		{"2026-01-01", "2026-01-31", "><2026-01-01|2026-01-31"},
		{"2026-01-01", "", ">=2026-01-01"},
		{"", "2026-01-31", "<=2026-01-31"},
	}
	for _, c := range cases {
		if got := spentOnRange(c.from, c.to); got != c.want {
			t.Errorf("spentOnRange(%q, %q) = %q, want %q", c.from, c.to, got, c.want)
		}
	}
}

func TestListTimeEntriesAppliesFilters(t *testing.T) {
	var gotQuery map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = map[string]string{
			"issue_id": r.URL.Query().Get("issue_id"),
			"spent_on": r.URL.Query().Get("spent_on"),
		}
		resp := timeEntryListResponse{
			TimeEntries: []TimeEntry{{ID: 1, Hours: 2.5}},
			TotalCount:  1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := New(srv.URL, "test-key")
	entries, err := client.ListTimeEntries(TimeEntryListFilter{
		IssueID: "42",
		From:    "2026-01-01",
		To:      "2026-01-31",
	})
	if err != nil {
		t.Fatalf("ListTimeEntries: %v", err)
	}
	if len(entries) != 1 || entries[0].Hours != 2.5 {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	if gotQuery["issue_id"] != "42" {
		t.Errorf("issue_id = %q, want 42", gotQuery["issue_id"])
	}
	if gotQuery["spent_on"] != "><2026-01-01|2026-01-31" {
		t.Errorf("spent_on = %q", gotQuery["spent_on"])
	}
}

func TestDeleteTimeEntry(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(srv.URL, "test-key")
	if err := client.DeleteTimeEntry(7); err != nil {
		t.Fatalf("DeleteTimeEntry: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/time_entries/7.json" {
		t.Errorf("path = %q", gotPath)
	}
}
