package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// rmine reports issues by number, which is not something a person can click.
// An agent relaying a result had no way to produce a link at all.
func TestIssueViewShowsWebURL(t *testing.T) {
	srv := issueDetailServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "issue", "view", "1234")

	want := srv.URL + "/issues/1234"
	if !strings.Contains(out, want) {
		t.Errorf("issue view output missing %q:\n%s", want, out)
	}
}

func TestIssueViewJSONCarriesWebURL(t *testing.T) {
	srv := issueDetailServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "issue", "view", "1234", "-o", "json")

	var got struct {
		ID      int    `json:"id"`
		URL     string `json:"url"`
		Subject string `json:"subject"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output isn't valid JSON: %v\noutput: %s", err, out)
	}
	if want := srv.URL + "/issues/1234"; got.URL != want {
		t.Errorf("url = %q, want %q", got.URL, want)
	}
	// The decoration must not displace the issue's own fields.
	if got.ID != 1234 || got.Subject == "" {
		t.Errorf("issue fields lost in decoration: %+v", got)
	}
}

func TestIssueListJSONCarriesWebURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issues.json" {
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"issues": []map[string]any{
				{"id": 11, "subject": "first"},
				{"id": 22, "subject": "second"},
			},
			"total_count": 2,
		})
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "issue", "list", "-o", "json")

	var got []struct {
		ID      int    `json:"id"`
		URL     string `json:"url"`
		Subject string `json:"subject"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output isn't valid JSON: %v\noutput: %s", err, out)
	}
	if len(got) != 2 {
		t.Fatalf("got %d issues, want 2", len(got))
	}
	for _, iss := range got {
		want := fmt.Sprintf("%s/issues/%d", srv.URL, iss.ID)
		if iss.URL != want {
			t.Errorf("issue %d url = %q, want %q", iss.ID, iss.URL, want)
		}
		if iss.Subject == "" {
			t.Errorf("issue %d lost its subject in decoration", iss.ID)
		}
	}
}
