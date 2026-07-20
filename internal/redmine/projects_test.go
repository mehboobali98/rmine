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
