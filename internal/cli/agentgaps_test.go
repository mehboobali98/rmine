package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// customFieldServer stands in for a Redmine whose tracker exposes field 11
// and not field 33 — the shape that made a --field 33 write look like it
// worked.
func customFieldServer(t *testing.T) *httptest.Server {
	t.Helper()
	stored := func() map[string]any {
		return map[string]any{
			"id": 555, "subject": "made",
			"project": map[string]any{"id": 2, "name": "Demo"},
			"tracker": map[string]any{"id": 4, "name": "SubTask"},
			"custom_fields": []map[string]any{
				{"id": 11, "name": "Team", "value": "Platform"},
			},
		}
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/issues.json" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{"issue": stored()})
		case r.URL.Path == "/issues/555.json" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/issues/555.json":
			json.NewEncoder(w).Encode(map[string]any{"issue": stored()})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
}

func TestCreateReportsCustomFieldsTheTrackerDropped(t *testing.T) {
	srv := customFieldServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	stdout, stderr, err := runCLIErr(t,
		"issue", "create", "--project", "2", "--subject", "made",
		"--field", "11=Platform", "--field", "33=Nope", "-o", "json")
	// The issue was created, so this stays a success — exiting non-zero here
	// would invite the retry that files a second ticket.
	if err != nil {
		t.Fatalf("create failed: %v\nstderr: %s", err, stderr)
	}

	var got struct {
		ID            int   `json:"id"`
		DroppedFields []int `json:"dropped_fields"`
	}
	if uErr := json.Unmarshal([]byte(stdout), &got); uErr != nil {
		t.Fatalf("stdout isn't valid JSON: %v\nstdout: %s", uErr, stdout)
	}
	if got.ID != 555 {
		t.Errorf("id = %d, want 555", got.ID)
	}
	if len(got.DroppedFields) != 1 || got.DroppedFields[0] != 33 {
		t.Fatalf("dropped_fields = %v, want [33]", got.DroppedFields)
	}
	if !strings.Contains(stderr, "33") || !strings.Contains(stderr, "SubTask") {
		t.Errorf("stderr should warn naming the field and tracker, got %q", stderr)
	}
}

func TestUpdateReportsCustomFieldsTheTrackerDropped(t *testing.T) {
	srv := customFieldServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	stdout, stderr, err := runCLIErr(t, "issue", "update", "555", "--field", "33=Nope", "-o", "json")
	if err != nil {
		t.Fatalf("update failed: %v\nstderr: %s", err, stderr)
	}
	var got actionResult
	if uErr := json.Unmarshal([]byte(stdout), &got); uErr != nil {
		t.Fatalf("stdout isn't valid JSON: %v\nstdout: %s", uErr, stdout)
	}
	if got.Status != "updated" {
		t.Errorf("status = %q, want updated", got.Status)
	}
	if len(got.DroppedFields) != 1 || got.DroppedFields[0] != 33 {
		t.Errorf("dropped_fields = %v, want [33]", got.DroppedFields)
	}
}

// A write whose fields all landed must stay quiet, or the warning becomes
// noise that gets ignored.
func TestWritesStaySilentWhenEveryFieldLands(t *testing.T) {
	srv := customFieldServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	stdout, stderr, err := runCLIErr(t,
		"issue", "create", "--project", "2", "--subject", "made", "--field", "11=Platform", "-o", "json")
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if strings.Contains(stderr, "Warning") {
		t.Errorf("stderr warned about a field that was stored: %q", stderr)
	}
	if strings.Contains(stdout, "dropped_fields") {
		t.Errorf("dropped_fields present on a clean write: %s", stdout)
	}
}

// Reading back costs a request, so it must only happen when custom fields
// were part of the edit.
func TestUpdateWithoutFieldsDoesNotReadBack(t *testing.T) {
	var gets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets++
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	if _, _, err := runCLIErr(t, "issue", "update", "555", "--subject", "new"); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if gets != 0 {
		t.Errorf("made %d read-back request(s) for an edit with no --field", gets)
	}
}

func TestCreateUploadsAttachmentsAndReferencesTheToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.pdf")
	if err := os.WriteFile(path, []byte("pdf bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var uploaded string
	var issueBody map[string]map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uploads.json":
			uploaded = r.URL.Query().Get("filename")
			json.NewEncoder(w).Encode(map[string]any{"upload": map[string]any{"id": 1, "token": "1.tok"}})
		case "/issues.json":
			json.NewDecoder(r.Body).Decode(&issueBody)
			json.NewEncoder(w).Encode(map[string]any{"issue": map[string]any{"id": 5, "subject": "x"}})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	if _, _, err := runCLIErr(t, "issue", "create", "--project", "2", "--subject", "x", "--attach", path); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if uploaded != "spec.pdf" {
		t.Errorf("uploaded filename = %q, want spec.pdf", uploaded)
	}
	uploads, ok := issueBody["issue"]["uploads"].([]any)
	if !ok || len(uploads) != 1 {
		t.Fatalf("issue payload carried no upload: %+v", issueBody["issue"])
	}
	if got := uploads[0].(map[string]any)["token"]; got != "1.tok" {
		t.Errorf("token = %v, want 1.tok", got)
	}
}

// A file that cannot be read must stop the write, not create an issue whose
// description promises an attachment that isn't there.
func TestCreateFailsBeforeWritingWhenAnAttachmentIsUnreadable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/issues.json" {
			t.Error("an issue was created despite an unreadable attachment")
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	_, _, err := runCLIErr(t, "issue", "create", "--project", "2", "--subject", "x",
		"--attach", filepath.Join(t.TempDir(), "gone.pdf"))
	if err == nil {
		t.Fatal("want an error for a missing attachment")
	}
}

func TestIssueViewShowsSubtasksAndRelations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"issue": map[string]any{
			"id": 100, "subject": "parent",
			"children": []map[string]any{
				{"id": 101, "subject": "first sub", "tracker": map[string]any{"id": 4, "name": "SubTask"}},
			},
			"relations": []map[string]any{
				// Recorded from #99's side: #99 precedes #100, so from #100
				// this has to read as "follows #99".
				{"id": 9, "issue_id": 99, "issue_to_id": 100, "relation_type": "precedes"},
			},
		}})
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "issue", "view", "100")
	if !strings.Contains(out, "#101") || !strings.Contains(out, "first sub") {
		t.Errorf("subtasks missing from view:\n%s", out)
	}
	if !strings.Contains(out, "follows #99") {
		t.Errorf("relation should read from #100's side as \"follows #99\":\n%s", out)
	}
}

func TestProjectViewAcceptsADisplayName(t *testing.T) {
	var fetched string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects.json":
			json.NewEncoder(w).Encode(map[string]any{"projects": []map[string]any{
				{"id": 2, "name": "Display Name", "identifier": "display-name"},
			}, "total_count": 1})
		case "/projects/2.json":
			fetched = r.URL.Query().Get("include")
			json.NewEncoder(w).Encode(map[string]any{"project": map[string]any{
				"id": 2, "name": "Display Name", "identifier": "display-name", "status": 1,
				"trackers":         []map[string]any{{"id": 1, "name": "Bug"}, {"id": 4, "name": "SubTask"}},
				"issue_categories": []map[string]any{{"id": 3, "name": "Backend"}},
			}})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	// The spelling that used to 404 while every other project-taking command
	// accepted it.
	out := runCLI(t, "project", "view", "Display Name")
	if !strings.Contains(out, "Display Name") {
		t.Errorf("view did not resolve the display name:\n%s", out)
	}
	if !strings.Contains(fetched, "trackers") {
		t.Errorf("include = %q, want the detail includes", fetched)
	}
	// The discovery payload: which trackers and categories a create may use.
	if !strings.Contains(out, "SubTask") || !strings.Contains(out, "Backend") {
		t.Errorf("view did not report trackers and categories:\n%s", out)
	}
}

func TestVersionFlagResolvesNamesAndClears(t *testing.T) {
	var lastBody map[string]map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/issues/555.json":
			if r.Method == http.MethodPut {
				json.NewDecoder(r.Body).Decode(&lastBody)
				w.WriteHeader(http.StatusNoContent)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"issue": map[string]any{
				"id": 555, "project": map[string]any{"id": 2, "name": "Demo"},
			}})
		case "/projects/2/versions.json":
			json.NewEncoder(w).Encode(map[string]any{"versions": []map[string]any{
				{"id": 4, "name": "Sprint 42", "status": "open"},
			}})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	if _, _, err := runCLIErr(t, "issue", "update", "555", "--version", "sprint 42"); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if got := lastBody["issue"]["fixed_version_id"]; got != float64(4) {
		t.Errorf("fixed_version_id = %#v, want 4", got)
	}

	if _, _, err := runCLIErr(t, "issue", "update", "555", "--version", ""); err != nil {
		t.Fatalf("clearing failed: %v", err)
	}
	if got := lastBody["issue"]["fixed_version_id"]; got != "" {
		t.Errorf("clearing sent %#v, want an empty string", got)
	}
}

func TestRelateRejectsBadInputBeforeCallingTheServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s", r.URL.Path)
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown type", []string{"issue", "relate", "1", "blocking", "2"}, "blocks"},
		{"self relation", []string{"issue", "relate", "1", "relates", "1"}, "itself"},
		{"delay where it does not apply", []string{"issue", "relate", "1", "relates", "2", "--delay", "3"}, "precedes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := runCLIErr(t, tc.args...)
			if err == nil {
				t.Fatalf("want an error for %v", tc.args)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestRelateReportsTheCreatedRelation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issues/100/relations.json" {
			t.Errorf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{"relation": map[string]any{
			"id": 9, "issue_id": 100, "issue_to_id": 200, "relation_type": "precedes",
		}})
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "issue", "relate", "100", "precedes", "200", "-o", "json")
	var got actionResult
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output isn't valid JSON: %v\noutput: %s", err, out)
	}
	if got.Status != "related" || got.Relation != 9 || got.Issue != 100 {
		t.Errorf("result = %+v, want status=related relation=9 issue=100", got)
	}
}

func TestNameLookupsListTheValidSpellings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/trackers.json":
			json.NewEncoder(w).Encode(map[string]any{"trackers": []map[string]any{
				{"id": 1, "name": "Bug"}, {"id": 2, "name": "Feature"},
			}})
		case "/issue_statuses.json":
			json.NewEncoder(w).Encode(map[string]any{"issue_statuses": []map[string]any{
				{"id": 1, "name": "New"}, {"id": 2, "name": "In Progress"},
			}})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	// Discovery by failure: a wrong guess now answers with the right ones.
	_, _, err := runCLIErr(t, "issue", "list", "--tracker", "Bugz")
	if err == nil || !strings.Contains(err.Error(), "Feature") {
		t.Errorf("tracker rejection = %v, want the valid names listed", err)
	}
	_, _, err = runCLIErr(t, "issue", "update", "1", "--status", "Doing")
	if err == nil || !strings.Contains(err.Error(), "In Progress") {
		t.Errorf("status rejection = %v, want the valid names listed", err)
	}
}

func TestEnumerationListingCommands(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/trackers.json":
			json.NewEncoder(w).Encode(map[string]any{"trackers": []map[string]any{{"id": 4, "name": "SubTask"}}})
		case "/issue_statuses.json":
			json.NewEncoder(w).Encode(map[string]any{"issue_statuses": []map[string]any{
				{"id": 5, "name": "Closed", "is_closed": true},
			}})
		case "/enumerations/issue_priorities.json":
			json.NewEncoder(w).Encode(map[string]any{"issue_priorities": []map[string]any{{"id": 6, "name": "Urgent"}}})
		case "/enumerations/time_entry_activities.json":
			json.NewEncoder(w).Encode(map[string]any{"time_entry_activities": []map[string]any{{"id": 7, "name": "Design"}}})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	for _, tc := range []struct{ group, want string }{
		{"tracker", "SubTask"},
		{"status", "Closed"},
		{"priority", "Urgent"},
		{"activity", "Design"},
	} {
		out := runCLI(t, tc.group, "list")
		if !strings.Contains(out, tc.want) {
			t.Errorf("%s list did not report %q:\n%s", tc.group, tc.want, out)
		}
	}

	// The closed column is what makes `issue close`'s default checkable.
	if out := runCLI(t, "status", "list"); !strings.Contains(out, "CLOSED") {
		t.Errorf("status list is missing the CLOSED column:\n%s", out)
	}
}

func TestProjectVersionsListing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/2/versions.json":
			json.NewEncoder(w).Encode(map[string]any{"versions": []map[string]any{
				{"id": 4, "name": "Sprint 42", "status": "open", "due_date": "2026-09-01",
					"project": map[string]any{"id": 1, "name": "Parent"}},
			}})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "project", "versions", "2")
	for _, want := range []string{"Sprint 42", "open", "2026-09-01", "Parent"} {
		if !strings.Contains(out, want) {
			t.Errorf("versions listing missing %q:\n%s", want, out)
		}
	}
}
