package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureIssueBody serves issue writes and records the body it was sent.
func captureIssueBody(t *testing.T, body *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var wrapper struct {
			Issue map[string]any `json:"issue"`
		}
		if err := json.NewDecoder(r.Body).Decode(&wrapper); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		*body = wrapper.Issue
		json.NewEncoder(w).Encode(map[string]any{"issue": map[string]any{"id": 1, "subject": "created"}})
	}))
}

func TestIssueCreateSendsSchedulingFields(t *testing.T) {
	var got map[string]any
	srv := captureIssueBody(t, &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "issue", "create",
		"--project", "7", "--subject", "Ship it",
		"--start-date", "2026-08-01",
		"--due-date", "2026-08-29",
		"--estimated-hours", "3.5",
		"--done-ratio", "40",
	)

	want := map[string]any{
		"start_date":      "2026-08-01",
		"due_date":        "2026-08-29",
		"estimated_hours": 3.5,
		"done_ratio":      float64(40),
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s sent as %#v, want %#v", k, got[k], v)
		}
	}
}

func TestIssueUpdateSendsSchedulingFields(t *testing.T) {
	var got map[string]any
	srv := captureIssueBody(t, &got)
	defer srv.Close()
	setupTestProfile(t, srv)

	runCLI(t, "issue", "update", "1234", "--due-date", "2026-09-30")

	if got["due_date"] != "2026-09-30" {
		t.Errorf("due_date sent as %#v, want 2026-09-30", got["due_date"])
	}
}

// Redmine answers a malformed filter date with an empty result rather than an
// error, so an unchecked typo is indistinguishable from "nothing matched".
func TestDateFlagsAreValidated(t *testing.T) {
	srv := captureIssueBody(t, new(map[string]any))
	defer srv.Close()
	setupTestProfile(t, srv)

	cases := [][]string{
		{"issue", "list", "--due-after", "29-08-2026"},
		{"issue", "list", "--updated-after", "yesterday"},
		{"issue", "create", "--project", "7", "--subject", "x", "--due-date", "2026-13-45"},
		{"time", "list", "--from", "last monday"},
		{"time", "log", "42", "--hours", "1", "--date", "08/20/2026"},
	}
	for _, args := range cases {
		_, _, err := runCLIErr(t, args...)
		if err == nil {
			t.Errorf("%v: expected an error for a malformed date", args)
			continue
		}
		if !strings.Contains(err.Error(), "YYYY-MM-DD") {
			t.Errorf("%v: error should explain the format, got %v", args, err)
		}
	}
}

func TestValidateDatesAcceptsEmptyAndWellFormed(t *testing.T) {
	err := validateDates(
		dateFlag{"--due-date", ""},
		dateFlag{"--start-date", "2026-08-01"},
	)
	if err != nil {
		t.Errorf("validateDates: %v", err)
	}
}
