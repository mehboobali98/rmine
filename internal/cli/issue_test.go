package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mehboobali98/rmine/internal/redmine"
)

func TestResolveStatusFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"issue_statuses": []map[string]any{
				{"id": 1, "name": "New", "is_closed": false},
				{"id": 2, "name": "In Progress", "is_closed": false},
				{"id": 5, "name": "Closed", "is_closed": true},
			},
		})
	}))
	defer srv.Close()
	client := redmine.New(srv.URL, "test-key")

	cases := []struct{ in, want string }{
		{"", ""},
		{"open", "open"},
		{"closed", "closed"},
		{"*", "*"},
		{"7", "7"},
		{"New", "1"},
		{"in progress", "2"},
	}
	for _, c := range cases {
		got, err := resolveStatusFilter(client, c.in)
		if err != nil {
			t.Errorf("resolveStatusFilter(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolveStatusFilter(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	if _, err := resolveStatusFilter(client, "NoSuchStatus"); err == nil {
		t.Error("expected an error for an unknown status name")
	}
}

func TestResolveProjectFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{
				{"id": 7, "name": "AssetSonar Scrum Team", "identifier": "assetsonar-scrum"},
			},
			"total_count": 1,
		})
	}))
	defer srv.Close()
	client := redmine.New(srv.URL, "test-key")

	cases := []struct{ in, want string }{
		{"", ""},
		{"42", "42"},
		{"assetsonar scrum team", "7"},
		{"assetsonar-scrum", "7"},
		{"some-other-identifier", "some-other-identifier"}, // no match: passed through as-is
	}
	for _, c := range cases {
		got, err := resolveProjectFilter(client, c.in)
		if err != nil {
			t.Errorf("resolveProjectFilter(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolveProjectFilter(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestResolveDueDateRange(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC) // a Friday

	after, before, err := resolveDueDateRange(now, "", "", 2, false)
	if err != nil {
		t.Fatalf("--due-within: %v", err)
	}
	if after != "2026-07-24" || before != "2026-07-26" {
		t.Errorf("--due-within 2 = %q..%q, want 2026-07-24..2026-07-26", after, before)
	}

	after, before, err = resolveDueDateRange(now, "", "", 0, true)
	if err != nil {
		t.Fatalf("--due-next-week: %v", err)
	}
	if after != "2026-07-27" || before != "2026-08-02" {
		t.Errorf("--due-next-week = %q..%q, want 2026-07-27..2026-08-02", after, before)
	}

	after, before, err = resolveDueDateRange(now, "2026-01-01", "2026-01-31", 0, false)
	if err != nil {
		t.Fatalf("raw dates: %v", err)
	}
	if after != "2026-01-01" || before != "2026-01-31" {
		t.Errorf("raw dates = %q..%q, want unchanged", after, before)
	}

	if _, _, err := resolveDueDateRange(now, "", "", 2, true); err == nil {
		t.Error("expected an error when --due-within and --due-next-week are combined")
	}
	if _, _, err := resolveDueDateRange(now, "2026-01-01", "", 2, false); err == nil {
		t.Error("expected an error when a shortcut is combined with --due-after")
	}
}

func TestParseCustomFields(t *testing.T) {
	fields, err := parseCustomFields([]string{"12=staging", "34=P1"})
	if err != nil {
		t.Fatalf("parseCustomFields: %v", err)
	}
	want := []redmine.CustomField{{ID: 12, Value: "staging"}, {ID: 34, Value: "P1"}}
	if len(fields) != len(want) {
		t.Fatalf("got %d fields, want %d", len(fields), len(want))
	}
	for i, f := range fields {
		if f != want[i] {
			t.Errorf("field %d = %+v, want %+v", i, f, want[i])
		}
	}

	if _, err := parseCustomFields([]string{"no-equals-sign"}); err == nil {
		t.Error("expected an error for a value missing '='")
	}
	if _, err := parseCustomFields([]string{"notanumber=value"}); err == nil {
		t.Error("expected an error for a non-numeric id")
	}
}
