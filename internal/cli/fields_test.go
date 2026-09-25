package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// fieldsServer is a project using Bug and SubTask, plus a SOC tracker with no
// issues yet.
func fieldsServer(t *testing.T) *httptest.Server {
	t.Helper()
	cf := func(id int, name string) map[string]any {
		return map[string]any{"id": id, "name": name, "value": ""}
	}
	byTracker := map[string][]map[string]any{
		"1": {cf(5, "Client Reference"), cf(6, "Dev Reviewer")},
		"6": {cf(6, "Dev Reviewer"), cf(19, "QA Verified")},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/2.json":
			json.NewEncoder(w).Encode(map[string]any{"project": map[string]any{
				"id": 2, "name": "Assets", "identifier": "assets",
				"trackers": []map[string]any{
					{"id": 1, "name": "Bug"}, {"id": 6, "name": "SubTask"}, {"id": 8, "name": "SOC"},
				},
			}})
		case "/trackers.json":
			json.NewEncoder(w).Encode(map[string]any{"trackers": []map[string]any{
				{"id": 1, "name": "Bug"}, {"id": 6, "name": "SubTask"}, {"id": 8, "name": "SOC"}, {"id": 9, "name": "VAPT"},
			}})
		case "/issues.json":
			q := r.URL.Query()
			if q.Get("subproject_id") != "!*" || q.Get("status_id") != "*" {
				t.Errorf("sample query should exclude subprojects and include closed issues: %v", q)
			}
			issues := []map[string]any{}
			if fields, ok := byTracker[q.Get("tracker_id")]; ok {
				issues = append(issues, map[string]any{"id": 1, "custom_fields": fields})
			}
			json.NewEncoder(w).Encode(map[string]any{"issues": issues, "total_count": len(issues)})
		default:
			t.Errorf("unexpected request: %s", r.URL)
		}
	}))
}

func TestProjectFieldsGroupsFieldsByTracker(t *testing.T) {
	srv := fieldsServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	stdout, stderr, err := runCLIErr(t, "project", "fields", "2", "-o", "json")
	if err != nil {
		t.Fatalf("project fields failed: %v\nstderr: %s", err, stderr)
	}

	var got projectFields
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout isn't valid JSON: %v\nstdout: %s", err, stdout)
	}
	want := []projectField{
		{ID: 5, Name: "Client Reference", Trackers: []string{"Bug"}},
		{ID: 6, Name: "Dev Reviewer", Trackers: []string{"Bug", "SubTask"}},
		{ID: 19, Name: "QA Verified", Trackers: []string{"SubTask"}},
	}
	if !reflect.DeepEqual(got.Fields, want) {
		t.Errorf("fields = %+v, want %+v", got.Fields, want)
	}
	if !reflect.DeepEqual(got.UnsampledTrackers, []string{"SOC"}) {
		t.Errorf("unsampled_trackers = %v, want [SOC]", got.UnsampledTrackers)
	}
	if !strings.Contains(stderr, "SOC") {
		t.Errorf("stderr should name the tracker with nothing to sample, got %q", stderr)
	}
}

func TestProjectFieldsNarrowsToOneTracker(t *testing.T) {
	srv := fieldsServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "project", "fields", "2", "--tracker", "sub-task")

	if !strings.Contains(out, "QA Verified") || strings.Contains(out, "Client Reference") {
		t.Errorf("expected only SubTask's fields, got:\n%s", out)
	}
}

func TestProjectFieldsRejectsATrackerTheProjectDoesNotUse(t *testing.T) {
	srv := fieldsServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	_, _, err := runCLIErr(t, "project", "fields", "2", "--tracker", "VAPT")
	if err == nil || !strings.Contains(err.Error(), "VAPT") {
		t.Fatalf("expected an error naming VAPT, got %v", err)
	}
}
