package redmine

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetIssueIncludesAttachmentsAndOptionallyJournals(t *testing.T) {
	for _, tc := range []struct {
		name        string
		withComment bool
		wantInclude string
	}{
		{"comments off", false, "attachments"},
		{"comments on", true, "attachments,journals"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotInclude string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotInclude = r.URL.Query().Get("include")
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{
					"issue": map[string]any{
						"id": 1,
						"attachments": []map[string]any{
							{"id": 7, "filename": "spec.docx", "filesize": 1234, "content_url": srvURL(r) + "/attachments/download/7/spec.docx"},
						},
						"journals": []map[string]any{
							{"id": 3, "notes": "clarified the scope"},
							{"id": 4, "notes": ""},
						},
					},
				})
			}))
			defer srv.Close()

			issue, err := New(srv.URL, "test-key").GetIssue(1, tc.withComment)
			if err != nil {
				t.Fatalf("GetIssue: %v", err)
			}
			if gotInclude != tc.wantInclude {
				t.Errorf("include = %q, want %q", gotInclude, tc.wantInclude)
			}
			if len(issue.Attachments) != 1 || issue.Attachments[0].Filename != "spec.docx" {
				t.Errorf("attachments not parsed: %+v", issue.Attachments)
			}
			// The server echoes journals regardless; parsing them is what's under
			// test here, the include param above is what actually gates the fetch.
			if len(issue.Journals) != 2 || issue.Journals[0].Notes != "clarified the scope" {
				t.Errorf("journals not parsed: %+v", issue.Journals)
			}
		})
	}
}

func TestDownloadAuthenticatesAndStreamsBytes(t *testing.T) {
	const body = "not really a docx"
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Redmine-API-Key")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	if err := New(srv.URL, "test-key").Download(srv.URL+"/attachments/download/7/spec.docx", &buf); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if gotKey != "test-key" {
		t.Errorf("API key header = %q, want test-key", gotKey)
	}
	if buf.String() != body {
		t.Errorf("got %q, want %q", buf.String(), body)
	}
}

func TestDownloadSurfacesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	err := New(srv.URL, "test-key").Download(srv.URL+"/attachments/download/7/spec.docx", &buf)
	if err == nil {
		t.Fatal("expected an error for a 403 response")
	}
	if buf.Len() != 0 {
		t.Errorf("wrote %d bytes on a failed download, want 0", buf.Len())
	}
}

func srvURL(r *http.Request) string { return "http://" + r.Host }
