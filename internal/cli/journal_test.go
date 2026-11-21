package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mehboobali98/rmine/internal/redmine"
)

func TestDescribeDetail(t *testing.T) {
	str := func(s string) *string { return &s }
	attr := func(name string, oldValue, newValue *string) redmine.JournalDetail {
		return redmine.JournalDetail{Property: "attr", Name: name, OldValue: oldValue, NewValue: newValue}
	}
	names := detailNames{customFields: map[string]string{"7": "Docs Needed"}}

	for _, tc := range []struct {
		detail redmine.JournalDetail
		want   string
	}{
		{attr("assigned_to_id", nil, str("12")), "Assignee: (none) -> 12"},
		{attr("fixed_version_id", str("4"), str("6")), "Version: 4 -> 6"},
		{attr("parent_id", str("1200"), nil), "Parent: 1200 -> (none)"},
		{attr("priority_id", str("1"), str("2")), "Priority: 1 -> 2"},
		{attr("subject", str("Old"), str("New")), "subject: Old -> New"},
		{attr("due_date", str(""), str("2026-02-01")), "due_date: (none) -> 2026-02-01"},
		{redmine.JournalDetail{Property: "cf", Name: "8", NewValue: str("x")}, "Custom field 8: (none) -> x"},
		{redmine.JournalDetail{Property: "attachment", Name: "21", NewValue: str("shot.png")}, "attachment 21: (none) -> shot.png"},
	} {
		if got := describeDetail(tc.detail, names); got != tc.want {
			t.Errorf("describeDetail(%s %s) = %q, want %q", tc.detail.Property, tc.detail.Name, got, tc.want)
		}
	}
}

// journalServer serves issue 1234 with the given journals and a "Docs Needed"
// custom field, plus any extra paths in routes. Any other request fails the
// test, so a test can prove a lookup was never made.
func journalServer(t *testing.T, journals []map[string]any, routes map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/issues/1234.json" {
			json.NewEncoder(w).Encode(map[string]any{
				"issue": map[string]any{
					"id":            1234,
					"subject":       "Write the guide",
					"status":        map[string]any{"id": 5, "name": "Ready"},
					"custom_fields": []map[string]any{{"id": 7, "name": "Docs Needed", "value": "Yes"}},
					"journals":      journals,
				},
			})
			return
		}
		if handler, ok := routes[r.URL.Path]; ok {
			handler(w, r)
			return
		}
		t.Errorf("unexpected request: %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
}

func statusesHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{
		"issue_statuses": []map[string]any{{"id": 3, "name": "Review"}, {"id": 5, "name": "Ready"}},
	})
}

func failWith(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }
}

func journal(id int, user, notes, createdOn string, details ...map[string]any) map[string]any {
	return map[string]any{
		"id":         id,
		"user":       map[string]any{"id": id, "name": user},
		"notes":      notes,
		"created_on": createdOn,
		"details":    details,
	}
}

// detail builds one field change. A nil old or new value is sent as JSON
// null, the way Redmine reports a field that had no value.
func detail(property, name string, oldValue, newValue any) map[string]any {
	return map[string]any{"property": property, "name": name, "old_value": oldValue, "new_value": newValue}
}

func TestIssueViewJSONCarriesJournalDetails(t *testing.T) {
	srv := journalServer(t, []map[string]any{
		journal(10, "Ann", "", "2026-01-05T10:00:00Z", detail("attr", "status_id", "3", "5")),
	}, nil)
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "issue", "view", "1234", "--comments", "-o", "json")

	for _, want := range []string{`"details"`, `"name": "status_id"`, `"old_value": "3"`, `"new_value": "5"`} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %s:\n%s", want, out)
		}
	}
}

func TestIssueViewPrintsFieldChanges(t *testing.T) {
	srv := journalServer(t, []map[string]any{
		journal(10, "Ann", "", "2026-01-05T10:00:00Z",
			detail("attr", "status_id", "3", "5"),
			detail("cf", "7", nil, "Yes")),
		journal(11, "Bob", "Looks good", "2026-01-06T09:30:00Z",
			detail("attr", "description", "old text", "new text")),
	}, map[string]http.HandlerFunc{"/issue_statuses.json": statusesHandler})
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "issue", "view", "1234", "--comments")

	for _, want := range []string{
		"--- Ann, 2026-01-05 10:00 ---", // a field-only journal is no longer skipped
		"Status: Review -> Ready",
		"Docs Needed: (none) -> Yes",
		"--- Bob, 2026-01-06 09:30 ---",
		"Description: changed",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("issue view output missing %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"old text", "new text"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("issue view output prints description text %q:\n%s", unwanted, out)
		}
	}
	changed, notes := strings.Index(out, "Description: changed"), strings.Index(out, "Looks good")
	if notes == -1 || notes < changed {
		t.Errorf("notes should follow the journal's field changes:\n%s", out)
	}
}

func TestIssueViewSkipsStatusLookupWithoutStatusChanges(t *testing.T) {
	// No status, tracker or priority routes: journalServer fails the test if
	// any of them is requested.
	srv := journalServer(t, []map[string]any{
		journal(10, "Ann", "", "2026-01-05T10:00:00Z", detail("cf", "7", nil, "Yes")),
	}, nil)
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "issue", "view", "1234", "--comments")

	if !strings.Contains(out, "Docs Needed: (none) -> Yes") {
		t.Errorf("issue view output missing the custom field change:\n%s", out)
	}
}

func TestIssueViewFallsBackToRawIDs(t *testing.T) {
	t.Run("failed lookups", func(t *testing.T) {
		srv := journalServer(t, []map[string]any{
			journal(10, "Ann", "Moved", "2026-01-05T10:00:00Z",
				detail("attr", "status_id", "3", "5"),
				detail("attr", "tracker_id", "1", "2"),
				detail("cf", "7", nil, "Yes")),
		}, map[string]http.HandlerFunc{
			"/issue_statuses.json": failWith(http.StatusInternalServerError),
			"/trackers.json":       failWith(http.StatusForbidden),
		})
		defer srv.Close()
		setupTestProfile(t, srv)

		out, errOut, err := runCLIErr(t, "issue", "view", "1234", "--comments")

		if err != nil {
			t.Fatalf("a failed name lookup should not fail the view: %v\nstderr: %s", err, errOut)
		}
		for _, want := range []string{"Status: 3 -> 5", "Tracker: 1 -> 2", "Docs Needed: (none) -> Yes", "Moved"} {
			if !strings.Contains(out, want) {
				t.Errorf("issue view output missing %q:\n%s", want, out)
			}
		}
		lines := strings.Split(strings.TrimSpace(errOut), "\n")
		if len(lines) != 2 {
			t.Fatalf("want one warning line per failed lookup, got %d:\n%s", len(lines), errOut)
		}
		lower := strings.ToLower(errOut)
		if !strings.Contains(lower, "status") || !strings.Contains(lower, "tracker") {
			t.Errorf("warnings should name the failed lookups:\n%s", errOut)
		}
	})

	t.Run("id the lookup does not list", func(t *testing.T) {
		srv := journalServer(t, []map[string]any{
			journal(10, "Ann", "", "2026-01-05T10:00:00Z", detail("attr", "status_id", "3", "9")),
		}, map[string]http.HandlerFunc{"/issue_statuses.json": statusesHandler})
		defer srv.Close()
		setupTestProfile(t, srv)

		out, errOut, err := runCLIErr(t, "issue", "view", "1234", "--comments")

		if err != nil {
			t.Fatalf("issue view: %v", err)
		}
		if !strings.Contains(out, "Status: Review -> 9") {
			t.Errorf("issue view output missing the raw id fallback:\n%s", out)
		}
		if errOut != "" {
			t.Errorf("an unlisted id is not a failed lookup, want no warning, got:\n%s", errOut)
		}
	})
}
