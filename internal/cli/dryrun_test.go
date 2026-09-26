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

// readOnlyServer answers reads and fails the test on any write.
func readOnlyServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("dry run sent a write: %s %s", r.Method, r.URL)
			return
		}
		switch r.URL.Path {
		case "/trackers.json":
			json.NewEncoder(w).Encode(map[string]any{"trackers": []map[string]any{{"id": 6, "name": "SubTask"}}})
		default:
			json.NewEncoder(w).Encode(map[string]any{"issue": map[string]any{"id": 1, "project": map[string]any{"id": 2}}})
		}
	}))
}

type dryRunReport struct {
	DryRun   bool `json:"dry_run"`
	Requests []struct {
		Method string         `json:"method"`
		Path   string         `json:"path"`
		Body   map[string]any `json:"body"`
		File   string         `json:"file"`
	} `json:"requests"`
}

func TestDryRunCreateShowsTheRequestWithoutSendingIt(t *testing.T) {
	srv := readOnlyServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	stdout, stderr, err := runCLIErr(t, "issue", "create", "--project", "2", "--subject", "s",
		"--tracker", "sub-task", "--dry-run", "-o", "json")
	if err != nil {
		t.Fatalf("dry run failed: %v\nstderr: %s", err, stderr)
	}

	var got dryRunReport
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout isn't valid JSON: %v\n%s", err, stdout)
	}
	if !got.DryRun || len(got.Requests) != 1 {
		t.Fatalf("want one planned request, got %+v", got)
	}
	req := got.Requests[0]
	issue, _ := req.Body["issue"].(map[string]any)
	if req.Method != "POST" || req.Path != "/issues.json" || issue["subject"] != "s" || issue["tracker_id"] != float64(6) {
		t.Errorf("planned request = %+v", req)
	}
}

func TestDryRunListsUploadsBeforeTheWriteThatUsesThem(t *testing.T) {
	srv := readOnlyServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	path := filepath.Join(t.TempDir(), "spec.pdf")
	if err := os.WriteFile(path, []byte("%PDF"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout := runCLI(t, "issue", "update", "1", "--notes", "see spec", "--attach", path, "--dry-run", "-o", "json")

	var got dryRunReport
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout isn't valid JSON: %v\n%s", err, stdout)
	}
	if len(got.Requests) != 2 {
		t.Fatalf("want upload then PUT, got %+v", got.Requests)
	}
	if up := got.Requests[0]; up.Method != "POST" || !strings.HasPrefix(up.Path, "/uploads.json") || up.File != path {
		t.Errorf("upload = %+v", up)
	}
	put := got.Requests[1]
	if put.Method != "PUT" || put.Path != "/issues/1.json" || !strings.Contains(stdout, "token from the upload above") {
		t.Errorf("update = %+v", put)
	}
}

func TestDryRunDeleteNeitherPromptsNorDeletes(t *testing.T) {
	srv := readOnlyServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	stdout, stderr, err := runCLIErr(t, "time", "delete", "42", "--dry-run")
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if strings.Contains(stderr, "[y/N]") {
		t.Errorf("dry run prompted for confirmation: %q", stderr)
	}
	if !strings.Contains(stdout, "DELETE /time_entries/42.json") || !strings.Contains(stdout, "nothing was sent") {
		t.Errorf("stdout = %q", stdout)
	}
}
