package redmine

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMissingCustomFieldsSpotsTheSilentDrop covers the case rmine could not
// previously detect at all: Redmine accepts a custom field the tracker does
// not expose, answers 200, and omits it from the stored issue.
func TestMissingCustomFieldsSpotsTheSilentDrop(t *testing.T) {
	cases := []struct {
		name      string
		requested []CustomField
		stored    []CustomField
		want      []int
	}{
		{
			name:      "field the tracker does not expose",
			requested: []CustomField{{ID: 33, Value: "SomeValue"}},
			stored:    []CustomField{{ID: 11, Name: "Team"}},
			want:      []int{33},
		},
		{
			// The stored value being empty is not evidence of a drop: an
			// exposed field that was set to "" is a legitimate outcome, and
			// flagging it would make the warning cry wolf on every clear.
			name:      "exposed field stored empty",
			requested: []CustomField{{ID: 11, Value: ""}},
			stored:    []CustomField{{ID: 11, Name: "Team", Value: ""}},
			want:      nil,
		},
		{
			name:      "some kept, some dropped, reported in request order",
			requested: []CustomField{{ID: 33}, {ID: 11}, {ID: 44}},
			stored:    []CustomField{{ID: 11}},
			want:      []int{33, 44},
		},
		{
			// A multi-value field arrives as repeated --field flags with one
			// ID; it must not be reported twice.
			name:      "repeated id reported once",
			requested: []CustomField{{ID: 33}, {ID: 33}},
			stored:    nil,
			want:      []int{33},
		},
		{
			name:      "nothing requested",
			requested: nil,
			stored:    []CustomField{{ID: 11}},
			want:      nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MissingCustomFields(tc.requested, tc.stored)
			if len(got) != len(tc.want) {
				t.Fatalf("MissingCustomFields = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("MissingCustomFields = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestCreateIssueSendsVersionAndUploads(t *testing.T) {
	var body map[string]map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		json.NewEncoder(w).Encode(map[string]any{"issue": map[string]any{"id": 5}})
	}))
	defer srv.Close()

	_, err := New(srv.URL, "k").CreateIssue(CreateIssueRequest{
		ProjectID:      "2",
		Subject:        "with a milestone",
		FixedVersionID: 7,
		Uploads:        []Upload{{Token: "1.abc", Filename: "spec.pdf", ContentType: "application/pdf"}},
	})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}

	issue := body["issue"]
	if issue["fixed_version_id"] != float64(7) {
		t.Errorf("fixed_version_id = %v, want 7", issue["fixed_version_id"])
	}
	uploads, ok := issue["uploads"].([]any)
	if !ok || len(uploads) != 1 {
		t.Fatalf("uploads = %v, want one entry", issue["uploads"])
	}
	if got := uploads[0].(map[string]any)["token"]; got != "1.abc" {
		t.Errorf("upload token = %v, want 1.abc", got)
	}
}

func TestUpdateIssueClearsVersionAndCarriesNotes(t *testing.T) {
	cases := []struct {
		name    string
		version int
		want    any
	}{
		// Redmine clears these numeric fields on an empty string; a 0 would
		// come back as an invalid id instead of unsetting anything.
		{"clearing sends empty string", 0, ""},
		{"setting sends the id", 7, float64(7)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&body)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			notes := "why this changed"
			err := New(srv.URL, "k").UpdateIssue(1, UpdateIssueRequest{
				FixedVersionID: &tc.version,
				Notes:          &notes,
			})
			if err != nil {
				t.Fatalf("UpdateIssue: %v", err)
			}
			if got := body["issue"]["fixed_version_id"]; got != tc.want {
				t.Errorf("fixed_version_id = %#v, want %#v", got, tc.want)
			}
			if got := body["issue"]["notes"]; got != notes {
				t.Errorf("notes = %v, want %q", got, notes)
			}
		})
	}
}

// TestUpdateIssueOmitsUntouchedFields guards the pointer contract against the
// two fields added here: an edit that says nothing about the version or the
// notes must not send either.
func TestUpdateIssueOmitsUntouchedFields(t *testing.T) {
	var body map[string]map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	subject := "just the subject"
	if err := New(srv.URL, "k").UpdateIssue(1, UpdateIssueRequest{Subject: &subject}); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	for _, key := range []string{"fixed_version_id", "notes", "uploads"} {
		if _, present := body["issue"][key]; present {
			t.Errorf("%s was sent for an edit that never mentioned it", key)
		}
	}
}

func TestGetIssueRequestsChildrenAndRelations(t *testing.T) {
	cases := []struct {
		name string
		opts GetIssueOptions
		want string
	}{
		{"bare", GetIssueOptions{}, "attachments"},
		{"full", FullIssue(), "attachments,children,relations"},
		{"full with comments", GetIssueOptions{Comments: true, Children: true, Relations: true}, "attachments,journals,children,relations"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotInclude string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotInclude = r.URL.Query().Get("include")
				json.NewEncoder(w).Encode(map[string]any{"issue": map[string]any{
					"id": 1,
					"children": []map[string]any{
						{"id": 2, "subject": "sub", "tracker": map[string]any{"id": 4, "name": "SubTask"},
							"children": []map[string]any{{"id": 3, "subject": "deeper"}}},
					},
					"relations": []map[string]any{
						{"id": 9, "issue_id": 1, "issue_to_id": 5, "relation_type": "precedes"},
					},
				}})
			}))
			defer srv.Close()

			issue, err := New(srv.URL, "k").GetIssue(1, tc.opts)
			if err != nil {
				t.Fatalf("GetIssue: %v", err)
			}
			if gotInclude != tc.want {
				t.Errorf("include = %q, want %q", gotInclude, tc.want)
			}
			// The server echoes both regardless; what's under test above is
			// the include param, and here that the nesting parses.
			if len(issue.Children) != 1 || issue.Children[0].Subject != "sub" {
				t.Fatalf("children not parsed: %+v", issue.Children)
			}
			if len(issue.Children[0].Children) != 1 || issue.Children[0].Children[0].ID != 3 {
				t.Errorf("nested children not parsed: %+v", issue.Children[0])
			}
			if len(issue.Relations) != 1 || issue.Relations[0].RelationType != "precedes" {
				t.Errorf("relations not parsed: %+v", issue.Relations)
			}
		})
	}
}

func TestListIssuesFiltersByVersion(t *testing.T) {
	cases := []struct {
		name    string
		filter  IssueListFilter
		wantKey string
		want    string
	}{
		{"plain filter", IssueListFilter{VersionID: "7"}, "fixed_version_id", "7"},
		{"none", IssueListFilter{VersionID: "!*"}, "fixed_version_id", "!*"},
		// A subject search forces every filter into the advanced form, or
		// Redmine ignores the ones left behind.
		{"advanced form", IssueListFilter{VersionID: "7", Subject: "x"}, "v[fixed_version_id][]", "7"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var query string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				query = r.URL.RawQuery
				json.NewEncoder(w).Encode(map[string]any{"issues": []any{}, "total_count": 0})
			}))
			defer srv.Close()

			if _, err := New(srv.URL, "k").ListIssues(tc.filter); err != nil {
				t.Fatalf("ListIssues: %v", err)
			}
			parsed, _ := url.ParseQuery(query)
			if got := parsed.Get(tc.wantKey); got != tc.want {
				t.Errorf("%s = %q, want %q (query: %s)", tc.wantKey, got, tc.want, query)
			}
		})
	}
}

func TestUploadFileStreamsBytesAndNamesTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.pdf")
	const content = "%PDF-1.4 not really"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var gotFilename, gotContentType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/uploads.json" {
			t.Errorf("path = %s, want /uploads.json", r.URL.Path)
		}
		gotFilename = r.URL.Query().Get("filename")
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		json.NewEncoder(w).Encode(map[string]any{"upload": map[string]any{"id": 7, "token": "7.xyz"}})
	}))
	defer srv.Close()

	upload, err := New(srv.URL, "k").UploadFile(path)
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if upload.Token != "7.xyz" {
		t.Errorf("token = %q, want 7.xyz", upload.Token)
	}
	if upload.Filename != "report.pdf" {
		t.Errorf("filename = %q, want report.pdf", upload.Filename)
	}
	if upload.ContentType != "application/pdf" {
		t.Errorf("content type = %q, want application/pdf", upload.ContentType)
	}
	if gotFilename != "report.pdf" {
		t.Errorf("query filename = %q", gotFilename)
	}
	// Redmine reads the raw body; sending it as JSON would store the quoted
	// base64 of the file rather than the file.
	if gotContentType != "application/octet-stream" {
		t.Errorf("request content type = %q, want application/octet-stream", gotContentType)
	}
	if gotBody != content {
		t.Errorf("body = %q, want %q", gotBody, content)
	}
}

func TestUploadFileReportsAMissingFileWithoutCallingTheServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("client called the server for a file it could not open")
	}))
	defer srv.Close()

	_, err := New(srv.URL, "k").UploadFile(filepath.Join(t.TempDir(), "absent.pdf"))
	if err == nil {
		t.Fatal("want an error for a missing file")
	}
	if !strings.Contains(err.Error(), "absent.pdf") {
		t.Errorf("error does not name the file: %v", err)
	}
}

func TestListVersionsAndResolveByName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/2/versions.json" {
			t.Errorf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{"versions": []map[string]any{
			{"id": 3, "name": "Sprint 41", "status": "closed"},
			{"id": 4, "name": "Sprint 42", "status": "open", "due_date": "2026-09-01"},
		}})
	}))
	defer srv.Close()
	client := New(srv.URL, "k")

	versions, err := client.ListVersions("2")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 2 || versions[1].DueDate != "2026-09-01" {
		t.Fatalf("versions not parsed: %+v", versions)
	}

	// Case-insensitive, like every other name match in rmine.
	id, err := client.ResolveVersionID("2", "sprint 42")
	if err != nil {
		t.Fatalf("ResolveVersionID: %v", err)
	}
	if id != 4 {
		t.Errorf("id = %d, want 4", id)
	}

	_, err = client.ResolveVersionID("2", "Sprint 99")
	if err == nil {
		t.Fatal("want an error for an unknown version")
	}
	if !strings.Contains(err.Error(), "Sprint 42") {
		t.Errorf("rejection does not list the valid versions: %v", err)
	}
}
