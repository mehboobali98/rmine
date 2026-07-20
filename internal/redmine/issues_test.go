package redmine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestListIssuesPagesThroughResults(t *testing.T) {
	const total = 5
	var offsetsSeen []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Redmine-API-Key") != "test-key" {
			t.Errorf("missing/incorrect API key header")
		}
		offsetsSeen = append(offsetsSeen, r.URL.Query().Get("offset"))

		offset := atoiOrZero(r.URL.Query().Get("offset"))
		limit := atoiOrZero(r.URL.Query().Get("limit"))
		if limit == 0 {
			limit = 2
		}

		// Serve 2 issues per page.
		var issues []Issue
		for i := offset; i < offset+2 && i < total; i++ {
			issues = append(issues, Issue{ID: i + 1, Subject: "issue"})
		}
		resp := issueListResponse{Issues: issues, TotalCount: total}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := New(srv.URL, "test-key")
	issues, err := client.ListIssues(IssueListFilter{All: true})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != total {
		t.Fatalf("got %d issues, want %d", len(issues), total)
	}
	if len(offsetsSeen) < 2 {
		t.Fatalf("expected multiple pages, saw offsets %v", offsetsSeen)
	}
}

func TestListIssuesSubjectSearchUsesAdvancedFilters(t *testing.T) {
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		resp := issueListResponse{Issues: []Issue{{ID: 1}}, TotalCount: 1}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := New(srv.URL, "test-key")
	_, err := client.ListIssues(IssueListFilter{
		Subject:    "CMDB",
		AssignedTo: "me",
		ProjectID:  "42",
	})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}

	fields := gotQuery["f[]"]
	wantFields := map[string]bool{"project_id": true, "assigned_to_id": true, "subject": true}
	if len(fields) != len(wantFields) {
		t.Fatalf("f[] = %v, want fields %v", fields, wantFields)
	}
	for _, f := range fields {
		if !wantFields[f] {
			t.Errorf("unexpected filter field %q", f)
		}
	}
	if gotQuery.Get("op[subject]") != "~" {
		t.Errorf("op[subject] = %q, want ~", gotQuery.Get("op[subject]"))
	}
	if gotQuery.Get("v[subject][]") != "CMDB" {
		t.Errorf("v[subject][] = %q, want CMDB", gotQuery.Get("v[subject][]"))
	}
	if gotQuery.Get("op[project_id]") != "=" || gotQuery.Get("v[project_id][]") != "42" {
		t.Errorf("project_id filter = op %q value %q", gotQuery.Get("op[project_id]"), gotQuery.Get("v[project_id][]"))
	}
	if gotQuery.Get("op[assigned_to_id]") != "=" || gotQuery.Get("v[assigned_to_id][]") != "me" {
		t.Errorf("assigned_to_id filter = op %q value %q", gotQuery.Get("op[assigned_to_id]"), gotQuery.Get("v[assigned_to_id][]"))
	}
	// The simple-param path must not also be sent — Redmine ignores it once f[] is present.
	if gotQuery.Get("project_id") != "" || gotQuery.Get("assigned_to_id") != "" {
		t.Errorf("simple params leaked alongside advanced filters: %v", gotQuery)
	}
}

func TestListIssuesAppliesAssigneeAndDateFilters(t *testing.T) {
	var gotQuery map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = map[string]string{
			"assigned_to_id": r.URL.Query().Get("assigned_to_id"),
			"updated_on":     r.URL.Query().Get("updated_on"),
		}
		resp := issueListResponse{Issues: []Issue{{ID: 1}}, TotalCount: 1}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := New(srv.URL, "test-key")
	_, err := client.ListIssues(IssueListFilter{
		AssignedTo:   "me",
		UpdatedAfter: "2026-07-01",
	})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if gotQuery["assigned_to_id"] != "me" {
		t.Errorf("assigned_to_id = %q, want me", gotQuery["assigned_to_id"])
	}
	if gotQuery["updated_on"] != ">=2026-07-01" {
		t.Errorf("updated_on = %q, want >=2026-07-01", gotQuery["updated_on"])
	}
}

func TestListIssuesRespectsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := issueListResponse{
			Issues:     []Issue{{ID: 1}, {ID: 2}, {ID: 3}},
			TotalCount: 10,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := New(srv.URL, "test-key")
	issues, err := client.ListIssues(IssueListFilter{Limit: 3})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 3 {
		t.Fatalf("got %d issues, want 3", len(issues))
	}
}

func TestGetIssueSurfacesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := New(srv.URL, "test-key")
	_, err := client.GetIssue(999)
	if err == nil {
		t.Fatal("expected an error for 404 response")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("got status %d, want 404", apiErr.StatusCode)
	}
}

func TestCreateIssueSurfacesValidationErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(errorBody{Errors: []string{"Subject can't be blank"}})
	}))
	defer srv.Close()

	client := New(srv.URL, "test-key")
	_, err := client.CreateIssue(CreateIssueRequest{ProjectID: "1"})
	if err == nil {
		t.Fatal("expected a validation error")
	}
	if got := err.Error(); got == "" {
		t.Fatal("expected non-empty error message")
	}
}

func atoiOrZero(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
