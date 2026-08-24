package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestJSONErrorsAreParseableOnStdout covers the reported gap: with -o json a
// failure left stdout empty, so a caller parsing stdout got a parse error
// that was indistinguishable from a crash — and on create, "re-run it to see
// the message" is the one recovery that risks a duplicate ticket.
func TestJSONErrorsAreParseableOnStdout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]any{
			"errors": []string{"Field A cannot be blank", "Field B cannot be blank"},
		})
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	stdout, stderr, err := runCLIErr(t, "issue", "create", "--project", "2", "--subject", "boom", "-o", "json")
	if err == nil {
		t.Fatal("want a non-nil error so the exit status is still non-zero")
	}

	var payload struct {
		Error struct {
			Message string   `json:"message"`
			Status  int      `json:"status"`
			Errors  []string `json:"errors"`
		} `json:"error"`
	}
	if uErr := json.Unmarshal([]byte(stdout), &payload); uErr != nil {
		t.Fatalf("stdout isn't valid JSON: %v\nstdout: %q", uErr, stdout)
	}
	if payload.Error.Status != 422 {
		t.Errorf("status = %d, want 422", payload.Error.Status)
	}
	if len(payload.Error.Errors) != 2 || payload.Error.Errors[0] != "Field A cannot be blank" {
		t.Errorf("errors = %v, want the server's two messages", payload.Error.Errors)
	}
	if !strings.Contains(payload.Error.Message, "422") {
		t.Errorf("message = %q, want it to mention the status", payload.Error.Message)
	}
	// The human sentence still reaches whoever is watching the terminal.
	if !strings.Contains(stderr, "Error:") {
		t.Errorf("stderr = %q, want the human error line", stderr)
	}
}

// A failure that never reached Redmine has no status to report, and a caller
// must be able to tell it from a 422: only one of the two says "nothing was
// written, resending is safe".
func TestNonAPIErrorsOmitStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	setupTestProfile(t, srv)

	stdout, _, err := runCLIErr(t, "issue", "view", "not-a-number", "-o", "json")
	if err == nil {
		t.Fatal("want an error")
	}
	var payload map[string]map[string]any
	if uErr := json.Unmarshal([]byte(stdout), &payload); uErr != nil {
		t.Fatalf("stdout isn't valid JSON: %v\nstdout: %q", uErr, stdout)
	}
	if _, present := payload["error"]["status"]; present {
		t.Errorf("a local failure reported an HTTP status: %+v", payload["error"])
	}
	if msg, _ := payload["error"]["message"].(string); !strings.Contains(msg, "not-a-number") {
		t.Errorf("message = %q, want it to name the bad argument", msg)
	}
}

// Flag parsing fails before -o is applied, and those failures — an agent
// guessing at a flag rmine does not have — are exactly the ones that need a
// parseable answer.
func TestJSONErrorSurvivesAFlagParseFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	setupTestProfile(t, srv)

	stdout, _, err := runCLIErr(t, "issue", "list", "--no-such-flag", "-o", "json")
	if err == nil {
		t.Fatal("want an error for an unknown flag")
	}
	var payload map[string]map[string]any
	if uErr := json.Unmarshal([]byte(stdout), &payload); uErr != nil {
		t.Fatalf("stdout isn't valid JSON: %v\nstdout: %q", uErr, stdout)
	}
	if msg, _ := payload["error"]["message"].(string); !strings.Contains(msg, "no-such-flag") {
		t.Errorf("message = %q, want it to name the unknown flag", msg)
	}
}

// Table output must stay unpolluted: a JSON error object on stdout is only
// correct for a caller that asked for JSON.
func TestTableModeLeavesStdoutEmptyOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	stdout, stderr, err := runCLIErr(t, "issue", "create", "--project", "2", "--subject", "boom")
	if err == nil {
		t.Fatal("want an error")
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want nothing in table mode", stdout)
	}
	if !strings.Contains(stderr, "Error:") {
		t.Errorf("stderr = %q, want the human error line", stderr)
	}
}

func TestUnknownOutputFormatIsRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("a command ran despite an unusable output format")
	}))
	defer srv.Close()
	setupTestProfile(t, srv)

	// `-o jsno` used to fall through to the table branch and exit 0, handing
	// back the one shape the caller had said it would not parse.
	_, _, err := runCLIErr(t, "issue", "list", "-o", "jsno")
	if err == nil {
		t.Fatal("want an error for an unknown --output value")
	}
	if !strings.Contains(err.Error(), "table or json") {
		t.Errorf("error = %v, want it to name the valid formats", err)
	}
}

func TestJSONInArgs(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"issue", "list", "-o", "json"}, true},
		{[]string{"issue", "list", "--output", "json"}, true},
		{[]string{"issue", "list", "--output=json"}, true},
		{[]string{"issue", "list", "-ojson"}, true},
		{[]string{"issue", "list"}, false},
		{[]string{"issue", "list", "-o", "table"}, false},
		{[]string{"issue", "list", "-o"}, false},
		// A subject that happens to look like the flag's value must not be
		// read as one.
		{[]string{"issue", "list", "--subject", "json"}, false},
	}
	for _, tc := range cases {
		if got := jsonInArgs(tc.args); got != tc.want {
			t.Errorf("jsonInArgs(%v) = %t, want %t", tc.args, got, tc.want)
		}
	}
}
