package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// issueDetailServer serves one fully-populated issue.
func issueDetailServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issues/1234.json" {
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"issue": map[string]any{
				"id":              1234,
				"subject":         "Fix login redirect",
				"project":         map[string]any{"id": 7, "name": "Web"},
				"tracker":         map[string]any{"id": 1, "name": "Bug"},
				"status":          map[string]any{"id": 2, "name": "In Progress"},
				"priority":        map[string]any{"id": 3, "name": "High"},
				"parent":          map[string]any{"id": 1200},
				"fixed_version":   map[string]any{"id": 4, "name": "2026.09"},
				"start_date":      "2026-08-01",
				"due_date":        "2026-08-29",
				"done_ratio":      40,
				"estimated_hours": 3.5,
				"spent_hours":     1.25,
			},
		})
	}))
}

// rmine has always been able to filter on due dates and never able to show
// one: the fields were absent from the Issue struct, so they were dropped
// from -o json as well as from the human view.
func TestIssueViewShowsSchedulingFields(t *testing.T) {
	srv := issueDetailServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "issue", "view", "1234")

	for _, want := range []string{
		"Due:      2026-08-29",
		"Start:    2026-08-01",
		"Parent:   #1200",
		"Version:  2026.09",
		"Progress: 40%",
		"Estimate: 3.50h",
		"Spent:    1.25h",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("issue view output missing %q:\n%s", want, out)
		}
	}
}

func TestIssueViewJSONCarriesSchedulingFields(t *testing.T) {
	srv := issueDetailServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "issue", "view", "1234", "-o", "json")

	var got struct {
		DueDate        string   `json:"due_date"`
		StartDate      string   `json:"start_date"`
		DoneRatio      int      `json:"done_ratio"`
		EstimatedHours *float64 `json:"estimated_hours"`
		Parent         *struct {
			ID int `json:"id"`
		} `json:"parent"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output isn't valid JSON: %v\noutput: %s", err, out)
	}
	if got.DueDate != "2026-08-29" || got.StartDate != "2026-08-01" || got.DoneRatio != 40 {
		t.Errorf("scheduling fields = %+v, want them populated", got)
	}
	if got.EstimatedHours == nil || *got.EstimatedHours != 3.5 {
		t.Errorf("estimated_hours = %v, want 3.5", got.EstimatedHours)
	}
	// --parent could be written since the previous release but never read back.
	if got.Parent == nil || got.Parent.ID != 1200 {
		t.Errorf("parent = %+v, want id 1200", got.Parent)
	}
}

func TestIssueListShowsProjectAndDueColumns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"issues": []map[string]any{
				{"id": 1, "subject": "has a due date", "due_date": "2026-08-29",
					"project": map[string]any{"id": 7, "name": "Web"}},
				{"id": 2, "subject": "has none",
					"project": map[string]any{"id": 8, "name": "Ops"}},
			},
			"total_count": 2,
		})
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "issue", "list")

	for _, want := range []string{"PROJECT", "DUE", "Web", "Ops", "2026-08-29"} {
		if !strings.Contains(out, want) {
			t.Errorf("issue list output missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "-") {
		t.Errorf("issue with no due date should render as -:\n%s", out)
	}
}
