package redmine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListProjectsPagesThroughResults(t *testing.T) {
	const total = 3
	requests := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		offset := atoiOrZero(r.URL.Query().Get("offset"))
		var projects []Project
		for i := offset; i < offset+2 && i < total; i++ {
			projects = append(projects, Project{ID: i + 1, Name: "proj"})
		}
		resp := projectListResponse{Projects: projects, TotalCount: total}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := New(srv.URL, "test-key")
	projects, err := client.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != total {
		t.Fatalf("got %d projects, want %d", len(projects), total)
	}
	if requests < 2 {
		t.Fatalf("expected pagination across multiple requests, got %d", requests)
	}
}

func TestResolveProjectIDMatchesNameOrIdentifierCaseInsensitively(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := projectListResponse{
			Projects: []Project{
				{ID: 7, Name: "AssetSonar Scrum Team", Identifier: "assetsonar-scrum"},
			},
			TotalCount: 1,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	client := New(srv.URL, "test-key")

	for _, in := range []string{"assetsonar scrum team", "AssetSonar Scrum Team", "assetsonar-scrum", "ASSETSONAR-SCRUM"} {
		id, err := client.ResolveProjectID(in)
		if err != nil {
			t.Errorf("ResolveProjectID(%q): %v", in, err)
			continue
		}
		if id != 7 {
			t.Errorf("ResolveProjectID(%q) = %d, want 7", in, id)
		}
	}

	if _, err := client.ResolveProjectID("no such project"); err == nil {
		t.Error("expected an error for an unknown project")
	}
}

func TestResolveIssueCategoryIDMatchesNameCaseInsensitively(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]any{
			"issue_categories": []map[string]any{
				{"id": 11, "name": "Functional"},
				{"id": 28, "name": "Bug"},
			},
		})
	}))
	defer srv.Close()
	client := New(srv.URL, "test-key")

	id, err := client.ResolveIssueCategoryID("2", "functional")
	if err != nil {
		t.Fatalf("ResolveIssueCategoryID: %v", err)
	}
	if id != 11 {
		t.Errorf("ResolveIssueCategoryID(...) = %d, want 11", id)
	}
	if gotPath != "/projects/2/issue_categories.json" {
		t.Errorf("path = %q, want /projects/2/issue_categories.json", gotPath)
	}

	if _, err := client.ResolveIssueCategoryID("2", "no such category"); err == nil {
		t.Error("expected an error for an unknown category")
	}
}

func TestGetProjectByIdentifier(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/my-project.json" {
			t.Errorf("path = %q", r.URL.Path)
		}
		json.NewEncoder(w).Encode(projectResponse{Project: Project{ID: 1, Identifier: "my-project"}})
	}))
	defer srv.Close()

	client := New(srv.URL, "test-key")
	project, err := client.GetProject("my-project")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if project.Identifier != "my-project" {
		t.Fatalf("got identifier %q", project.Identifier)
	}
}
