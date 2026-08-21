package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/mehboobali98/rmine/internal/config"
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

// resetFlags restores every flag in the tree to its default and clears its
// Changed marker. Flag values live in package-level vars that cobra only
// writes when a flag is present, so without this a `-o json` in one test
// silently stays in effect for every test that runs after it.
func resetFlags(cmd *cobra.Command) {
	clear := func(fs *pflag.FlagSet) {
		fs.VisitAll(func(f *pflag.Flag) {
			// Set() appends on slice flags rather than replacing, so
			// repeated resets would accumulate the default instead of
			// restoring it.
			if sv, ok := f.Value.(pflag.SliceValue); ok {
				_ = sv.Replace(nil)
			} else {
				_ = f.Value.Set(f.DefValue)
			}
			f.Changed = false
		})
	}
	clear(cmd.Flags())
	clear(cmd.PersistentFlags())
	for _, sub := range cmd.Commands() {
		resetFlags(sub)
	}
}

// runCLIErr executes rootCmd with args and returns its stdout, its stderr,
// and whatever error Execute reported.
func runCLIErr(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	resetFlags(rootCmd)
	t.Cleanup(func() { resetFlags(rootCmd) })

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	// Drain both pipes concurrently. A pipe holds ~64KB before it blocks, so
	// reading only after the command returns deadlocks the test on any
	// command that produces more output than that.
	outC, errC := make(chan []byte, 1), make(chan []byte, 1)
	go func() { b, _ := io.ReadAll(outR); outC <- b }()
	go func() { b, _ := io.ReadAll(errR); errC <- b }()

	origStdout, origStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW

	rootCmd.SetArgs(args)
	execErr := rootCmd.Execute()

	os.Stdout, os.Stderr = origStdout, origStderr
	outW.Close()
	errW.Close()

	return string(<-outC), string(<-errC), execErr
}

// runCLI executes rootCmd with args and returns whatever it wrote to stdout,
// failing the test if the command errored.
func runCLI(t *testing.T, args ...string) string {
	t.Helper()

	out, errOut, err := runCLIErr(t, args...)
	if err != nil {
		t.Fatalf("command %v failed: %v\nstdout: %s\nstderr: %s", args, err, out, errOut)
	}
	return out
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

// TestRunCLIDoesNotLeakFlagsBetweenRuns guards the harness itself: before
// resetFlags, a -o json in one run stayed set for every later run, so a test
// asserting on table output would have been handed JSON and passed anyway.
func TestRunCLIDoesNotLeakFlagsBetweenRuns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"issues":      []map[string]any{{"id": 7, "subject": "leak probe"}},
			"total_count": 1,
		})
	}))
	defer srv.Close()

	setupTestProfile(t, srv)

	if out := runCLI(t, "issue", "list", "-o", "json"); !strings.HasPrefix(out, "[") {
		t.Fatalf("first run: want JSON, got %q", out)
	}
	out := runCLI(t, "issue", "list")
	if strings.HasPrefix(out, "[") {
		t.Errorf("second run inherited -o json from the first: %q", out)
	}
	if !strings.Contains(out, "SUBJECT") {
		t.Errorf("second run: want table output with headers, got %q", out)
	}
}
