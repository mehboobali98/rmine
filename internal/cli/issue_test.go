package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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
	if !reflect.DeepEqual(fields, want) {
		t.Errorf("fields = %+v, want %+v", fields, want)
	}

	if _, err := parseCustomFields([]string{"no-equals-sign"}); err == nil {
		t.Error("expected an error for a value missing '='")
	}
	if _, err := parseCustomFields([]string{"notanumber=value"}); err == nil {
		t.Error("expected an error for a non-numeric id")
	}
}

func TestParseCustomFieldsCombinesRepeatedIDIntoMultiValue(t *testing.T) {
	fields, err := parseCustomFields([]string{"11=16", "11=27"})
	if err != nil {
		t.Fatalf("parseCustomFields: %v", err)
	}
	want := []redmine.CustomField{{ID: 11, Values: []string{"16", "27"}}}
	if !reflect.DeepEqual(fields, want) {
		t.Errorf("fields = %+v, want %+v", fields, want)
	}
}

func TestResolveTrackerFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"trackers": []map[string]any{
				{"id": 1, "name": "Bug"},
				{"id": 2, "name": "Feature"},
			},
		})
	}))
	defer srv.Close()
	client := redmine.New(srv.URL, "test-key")

	cases := []struct{ in, want string }{
		{"", ""},
		{"2", "2"},
		{"Bug", "1"},
		{"feature", "2"},
	}
	for _, c := range cases {
		got, err := resolveTrackerFilter(client, c.in)
		if err != nil {
			t.Errorf("resolveTrackerFilter(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolveTrackerFilter(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	// Previously this went to Redmine as tracker_id=NoSuchTracker, which
	// matched nothing and returned an empty list with a 200.
	if _, err := resolveTrackerFilter(client, "NoSuchTracker"); err == nil {
		t.Error("expected an error for an unknown tracker name")
	}
}

func TestResolveUserFilterRejectsNames(t *testing.T) {
	for _, in := range []string{"", "me", "42"} {
		got, err := resolveUserFilter("--assignee", in)
		if err != nil {
			t.Errorf("resolveUserFilter(%q): %v", in, err)
			continue
		}
		if got != in {
			t.Errorf("resolveUserFilter(%q) = %q, want it unchanged", in, got)
		}
	}

	_, err := resolveUserFilter("--assignee", "Jane Doe")
	if err == nil {
		t.Fatal("expected an error for a user name")
	}
	if !strings.Contains(err.Error(), "--assignee") || !strings.Contains(err.Error(), "Jane Doe") {
		t.Errorf("error should name the flag and the value, got: %v", err)
	}
}

func TestResolveIDFilterRejectsNonNumeric(t *testing.T) {
	got, err := resolveIDFilter("--issue", "1234")
	if err != nil || got != "1234" {
		t.Errorf("resolveIDFilter(1234) = %q, %v; want 1234, nil", got, err)
	}
	if _, err := resolveIDFilter("--issue", "CMDB-7"); err == nil {
		t.Error("expected an error for a non-numeric issue filter")
	}
}
