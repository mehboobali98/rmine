package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mutationServer answers the writes the mutating commands make, plus the
// status lookup `issue close` needs to find a closed status.
func mutationServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/issue_statuses.json":
			json.NewEncoder(w).Encode(map[string]any{
				"issue_statuses": []map[string]any{
					{"id": 1, "name": "New", "is_closed": false},
					{"id": 5, "name": "Closed", "is_closed": true},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/issues/"), strings.HasPrefix(r.URL.Path, "/time_entries/"):
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
}

// Both the README and SKILL.md tell callers that every command accepts
// -o json. The mutating commands used to print an English sentence either
// way, so an agent that passed the flag uniformly got a parse error back.
func TestMutatingCommandsEmitJSON(t *testing.T) {
	srv := mutationServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	cases := []struct {
		name string
		args []string
		want actionResult
	}{
		{"issue update", []string{"issue", "update", "1234", "--subject", "new"}, actionResult{Status: "updated", Issue: 1234}},
		{"issue close", []string{"issue", "close", "1234"}, actionResult{Status: "closed", Issue: 1234}},
		{"issue comment", []string{"issue", "comment", "1234", "a note"}, actionResult{Status: "commented", Issue: 1234}},
		{"time edit", []string{"time", "edit", "99", "--hours", "2"}, actionResult{Status: "updated", TimeEntry: 99}},
		{"time delete", []string{"time", "delete", "99", "-y"}, actionResult{Status: "deleted", TimeEntry: 99}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := runCLI(t, append(c.args, "-o", "json")...)

			var got actionResult
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("output isn't valid JSON: %v\noutput: %s", err, out)
			}
			if got != c.want {
				t.Errorf("result = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestMutatingCommandsStillPrintProseByDefault(t *testing.T) {
	srv := mutationServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "issue", "close", "1234")
	if want := "Closed issue #1234\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

func TestListProfilesEmitsJSON(t *testing.T) {
	srv := mutationServer(t)
	defer srv.Close()
	setupTestProfile(t, srv)

	out := runCLI(t, "config", "list-profiles", "-o", "json")

	var got []profileInfo
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output isn't valid JSON: %v\noutput: %s", err, out)
	}
	if len(got) != 1 || got[0].Name != "test" || !got[0].Current {
		t.Fatalf("profiles = %+v, want one current profile named test", got)
	}
	if strings.Contains(out, "test-key") {
		t.Error("list-profiles leaked the API key into its output")
	}
}
