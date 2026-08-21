package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// captureUpdateBody records the payload sent by a PUT, under the given key.
func captureUpdateBody(t *testing.T, key string, body *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/issues/1234.json" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]any{
				"issue": map[string]any{"id": 1234, "project": map[string]any{"id": 7, "name": "Web"}},
			})
			return
		}
		var wrapper map[string]map[string]any
		if err := json.NewDecoder(r.Body).Decode(&wrapper); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		*body = wrapper[key]
		w.WriteHeader(http.StatusOK)
	}))
}

// An update used to serialize the whole request struct with omitempty, so a
// flag the user never typed was indistinguishable from one set to its zero
// value — and every untouched field was simply dropped.
func TestIssueUpdateSendsOnlyTheFlagsGiven(t *testing.T) {
	var got map[string]any
	srv := captureUpdateBody(t, "issue", &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "issue", "update", "1234", "--subject", "New subject")

	if len(got) != 1 {
		t.Fatalf("sent %d fields (%v), want only subject", len(got), got)
	}
	if got["subject"] != "New subject" {
		t.Errorf("subject = %#v, want \"New subject\"", got["subject"])
	}
}

// Redmine clears these by being sent an empty string; 0 would be rejected as
// an invalid id, and omitting the field leaves the old value in place.
func TestIssueUpdateClearsFields(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		field string
	}{
		{"unassign", []string{"--assignee", "0"}, "assigned_to_id"},
		{"detach parent", []string{"--parent", "0"}, "parent_issue_id"},
		{"clear category", []string{"--category", ""}, "category_id"},
		{"clear estimate", []string{"--estimated-hours", "0"}, "estimated_hours"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got map[string]any
			srv := captureUpdateBody(t, "issue", &got)
			defer srv.Close()
			setupTestProfile(t, srv)

			runCLI(t, append([]string{"issue", "update", "1234"}, c.args...)...)

			v, ok := got[c.field]
			if !ok {
				t.Fatalf("%s was not sent at all: %v", c.field, got)
			}
			if v != "" {
				t.Errorf("%s = %#v, want an empty string to clear it", c.field, v)
			}
		})
	}
}

// Clearing a description means sending an empty string, which the previous
// omitempty encoding dropped — so `--description ""` was a no-op.
func TestIssueUpdateClearsDescription(t *testing.T) {
	var got map[string]any
	srv := captureUpdateBody(t, "issue", &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "issue", "update", "1234", "--description", "")

	v, ok := got["description"]
	if !ok {
		t.Fatalf("description was not sent: %v", got)
	}
	if v != "" {
		t.Errorf("description = %#v, want an empty string", v)
	}
}

// 0% complete is a real value, not an instruction to clear the field, so it
// must go out as the number 0 rather than the empty string.
func TestIssueUpdateSendsZeroDoneRatioAsANumber(t *testing.T) {
	var got map[string]any
	srv := captureUpdateBody(t, "issue", &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "issue", "update", "1234", "--done-ratio", "0")

	if got["done_ratio"] != float64(0) {
		t.Errorf("done_ratio = %#v, want the number 0", got["done_ratio"])
	}
}

func TestIssueUpdateRejectsOutOfRangeDoneRatio(t *testing.T) {
	var got map[string]any
	srv := captureUpdateBody(t, "issue", &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	if _, _, err := runCLIErr(t, "issue", "update", "1234", "--done-ratio", "140"); err == nil {
		t.Error("expected an error for a done ratio above 100")
	}
}

func TestTimeEditSendsOnlyTheFlagsGiven(t *testing.T) {
	var got map[string]any
	srv := captureUpdateBody(t, "time_entry", &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "time", "edit", "99", "--hours", "2.5")

	if len(got) != 1 || got["hours"] != 2.5 {
		t.Fatalf("sent %v, want only hours=2.5", got)
	}
}

func TestTimeEditClearsComment(t *testing.T) {
	var got map[string]any
	srv := captureUpdateBody(t, "time_entry", &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "time", "edit", "99", "--comment", "")

	v, ok := got["comments"]
	if !ok {
		t.Fatalf("comments was not sent: %v", got)
	}
	if v != "" {
		t.Errorf("comments = %#v, want an empty string", v)
	}
}
