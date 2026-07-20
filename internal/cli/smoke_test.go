package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/mehboobali98/rdm/internal/config"
)

// setupTestProfile points a "test" profile at srv and selects it, without
// touching the user's real config file.
func setupTestProfile(t *testing.T, srv *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	prevXDG, hadXDG := os.LookupEnv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", dir)
	t.Cleanup(func() {
		if hadXDG {
			os.Setenv("XDG_CONFIG_HOME", prevXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	})

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.Profiles["test"] = config.Profile{URL: srv.URL, APIKey: "test-key"}
	cfg.CurrentProfile = "test"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

// runCLI executes rootCmd with args and returns whatever it wrote to stdout.
func runCLI(t *testing.T, args ...string) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	rootCmd.SetArgs(args)
	execErr := rootCmd.Execute()

	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = origStdout

	if execErr != nil {
		t.Fatalf("command %v failed: %v\noutput so far: %s", args, execErr, out)
	}
	return string(out)
}

func TestIssueListSmoke(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/issues.json":
			json.NewEncoder(w).Encode(map[string]any{
				"issues": []map[string]any{
					{"id": 1, "subject": "Fix login bug", "tracker": map[string]any{"id": 1, "name": "Bug"},
						"status": map[string]any{"id": 1, "name": "New"}, "priority": map[string]any{"id": 1, "name": "Normal"}},
				},
				"total_count": 1,
			})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	setupTestProfile(t, srv)
	out := runCLI(t, "issue", "list", "-o", "json")

	var issues []map[string]any
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatalf("output isn't valid JSON: %v\noutput: %s", err, out)
	}
	if len(issues) != 1 || issues[0]["subject"] != "Fix login bug" {
		t.Fatalf("unexpected issues: %+v", issues)
	}
}

func TestTimeLogSmoke(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/time_entries.json":
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"time_entry": map[string]any{
					"id": 99, "hours": 1.5,
					"project":  map[string]any{"id": 1, "name": "Demo"},
					"activity": map[string]any{"id": 1, "name": "Development"},
				},
			})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	setupTestProfile(t, srv)
	out := runCLI(t, "time", "log", "42", "--hours", "1.5", "-o", "json")

	var entry map[string]any
	if err := json.Unmarshal([]byte(out), &entry); err != nil {
		t.Fatalf("output isn't valid JSON: %v\noutput: %s", err, out)
	}
	if entry["id"].(float64) != 99 {
		t.Fatalf("unexpected entry: %+v", entry)
	}
}
