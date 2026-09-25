package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// descriptionServer records the issue payload of every create or update.
func descriptionServer(t *testing.T, body *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/issues.json":
			var wrapper map[string]map[string]any
			json.NewDecoder(r.Body).Decode(&wrapper)
			*body = wrapper["issue"]
			json.NewEncoder(w).Encode(map[string]any{"issue": map[string]any{"id": 1, "subject": "ok"}})
		case r.Method == http.MethodPut && r.URL.Path == "/issues/1.json":
			var wrapper map[string]map[string]any
			json.NewDecoder(r.Body).Decode(&wrapper)
			*body = wrapper["issue"]
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
}

func TestIssueCreateReadsDescriptionFromFile(t *testing.T) {
	var body map[string]any
	srv := descriptionServer(t, &body)
	defer srv.Close()
	setupTestProfile(t, srv)

	const text = "## Context\n\nA body with `backticks`, $(dollars) and \"quotes\".\n"
	path := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}

	runCLI(t, "issue", "create", "--project", "2", "--subject", "s", "--description-file", path)

	if got := body["description"]; got != text {
		t.Errorf("description = %q, want %q", got, text)
	}
}

func TestIssueUpdateReadsDescriptionFromStdin(t *testing.T) {
	var body map[string]any
	srv := descriptionServer(t, &body)
	defer srv.Close()
	setupTestProfile(t, srv)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.WriteString("from stdin\n")
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin })

	runCLI(t, "issue", "update", "1", "--description-file", "-")

	if got := body["description"]; got != "from stdin\n" {
		t.Errorf("description = %q, want %q", got, "from stdin\n")
	}
}

func TestDescriptionFlagsAreMutuallyExclusive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request should not have been made")
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	_, _, err := runCLIErr(t, "issue", "update", "1", "--description", "x", "--description-file", "-")
	if err == nil {
		t.Fatal("expected --description with --description-file to be rejected")
	}
}

func TestMissingDescriptionFileFailsBeforeAnyRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request should not have been made")
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	_, _, err := runCLIErr(t, "issue", "create", "--project", "2", "--subject", "s",
		"--description-file", filepath.Join(t.TempDir(), "nope.md"))
	if err == nil {
		t.Fatal("expected a missing --description-file to fail")
	}
}
