package cli

import (
	"encoding/json"
	"errors"
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

// projectServer answers both the project listing and single-project fetches,
// so the identifier fast path and the listing fallback are both exercised.
func projectServer(t *testing.T, listCalls *int) *httptest.Server {
	t.Helper()
	const identifier = "assetsonar-scrum"
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects.json":
			if listCalls != nil {
				*listCalls++
			}
			json.NewEncoder(w).Encode(map[string]any{
				"projects": []map[string]any{
					{"id": 7, "name": "AssetSonar Scrum Team", "identifier": identifier},
				},
				"total_count": 1,
			})
		case "/projects/" + identifier + ".json":
			json.NewEncoder(w).Encode(map[string]any{
				"project": map[string]any{"id": 7, "name": "AssetSonar Scrum Team", "identifier": identifier},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestResolveProjectFilter(t *testing.T) {
	srv := projectServer(t, nil)
	defer srv.Close()
	client := redmine.New(srv.URL, "test-key")

	cases := []struct{ in, want string }{
		{"", ""},
		{"42", "42"},
		{"assetsonar scrum team", "7"},
		{"assetsonar-scrum", "7"},
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

// A name that does not match used to be passed through for Redmine to reject
// with a vaguer error, which read as a server problem rather than a typo.
func TestResolveProjectFilterReportsUnknownProject(t *testing.T) {
	srv := projectServer(t, nil)
	defer srv.Close()
	client := redmine.New(srv.URL, "test-key")

	_, err := resolveProjectFilter(client, "No Such Project")
	if err == nil {
		t.Fatal("expected an error for an unknown project name")
	}
	if !errors.Is(err, redmine.ErrNoMatch) {
		t.Errorf("error should wrap ErrNoMatch, got %v", err)
	}
	if !strings.Contains(err.Error(), "No Such Project") {
		t.Errorf("error should name the value, got %v", err)
	}
}

// When the lookup itself fails we have not established that the value is
// wrong, so it is still passed through for the server to judge.
func TestResolveProjectFilterPassesThroughWhenLookupFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	client := redmine.New(srv.URL, "test-key")

	got, err := resolveProjectFilter(client, "some-identifier")
	if err != nil {
		t.Fatalf("resolveProjectFilter: %v", err)
	}
	if got != "some-identifier" {
		t.Errorf("got %q, want the value passed through unchanged", got)
	}
}

// Resolving an identifier should not page through every project first.
func TestResolveProjectFilterSkipsListingForIdentifiers(t *testing.T) {
	listCalls := 0
	srv := projectServer(t, &listCalls)
	defer srv.Close()
	client := redmine.New(srv.URL, "test-key")

	if _, err := resolveProjectFilter(client, "assetsonar-scrum"); err != nil {
		t.Fatalf("resolveProjectFilter: %v", err)
	}
	if listCalls != 0 {
		t.Errorf("listed all projects %d time(s) to resolve an identifier", listCalls)
	}

	if _, err := resolveProjectFilter(client, "AssetSonar Scrum Team"); err != nil {
		t.Fatalf("resolveProjectFilter: %v", err)
	}
	if listCalls != 1 {
		t.Errorf("display-name lookup made %d listing calls, want 1", listCalls)
	}
}

func TestResolveDueDateRange(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC) // a Friday

	after, before, err := resolveDueDateRange(now, "", "", 2, false, false)
	if err != nil {
		t.Fatalf("--due-within: %v", err)
	}
	if after != "2026-07-24" || before != "2026-07-26" {
		t.Errorf("--due-within 2 = %q..%q, want 2026-07-24..2026-07-26", after, before)
	}

	after, before, err = resolveDueDateRange(now, "", "", 0, true, false)
	if err != nil {
		t.Fatalf("--due-next-week: %v", err)
	}
	if after != "2026-07-27" || before != "2026-08-02" {
		t.Errorf("--due-next-week = %q..%q, want 2026-07-27..2026-08-02", after, before)
	}

	after, before, err = resolveDueDateRange(now, "2026-01-01", "2026-01-31", 0, false, false)
	if err != nil {
		t.Fatalf("raw dates: %v", err)
	}
	if after != "2026-01-01" || before != "2026-01-31" {
		t.Errorf("raw dates = %q..%q, want unchanged", after, before)
	}

	if _, _, err := resolveDueDateRange(now, "", "", 2, true, false); err == nil {
		t.Error("expected an error when --due-within and --due-next-week are combined")
	}
	if _, _, err := resolveDueDateRange(now, "2026-01-01", "", 2, false, false); err == nil {
		t.Error("expected an error when a shortcut is combined with --due-after")
	}
}

func TestResolveDueDateRangeOverdue(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	after, before, err := resolveDueDateRange(now, "", "", 0, false, true)
	if err != nil {
		t.Fatalf("--overdue: %v", err)
	}
	if after != "" {
		t.Errorf("--overdue set a lower bound of %q, want none", after)
	}
	if before != "2026-07-23" {
		t.Errorf("--overdue = ..%q, want ..2026-07-23 (strictly before today)", before)
	}

	if _, _, err := resolveDueDateRange(now, "", "", 0, true, true); err == nil {
		t.Error("expected an error when --overdue and --due-next-week are combined")
	}
	if _, _, err := resolveDueDateRange(now, "", "2026-01-01", 0, false, true); err == nil {
		t.Error("expected an error when --overdue is combined with --due-before")
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

// A user name can only be resolved inside a project, because /users.json is
// admin-only while project memberships are readable by members.
func TestResolveUserFilterPassesThroughIDsAndMe(t *testing.T) {
	for _, in := range []string{"", "me", "42"} {
		got, err := resolveUserFilter(nil, "--assignee", in, "")
		if err != nil {
			t.Errorf("resolveUserFilter(%q): %v", in, err)
			continue
		}
		if got != in {
			t.Errorf("resolveUserFilter(%q) = %q, want it unchanged", in, got)
		}
	}
}

func TestResolveUserFilterNeedsAProjectForNames(t *testing.T) {
	_, err := resolveUserFilter(nil, "--assignee", "Jane Doe", "")
	if err == nil {
		t.Fatal("expected an error for a name with no project in scope")
	}
	for _, want := range []string{"--assignee", "Jane Doe", "--project"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

func TestResolveUserFilterResolvesNameWithinProject(t *testing.T) {
	srv := membershipServer(t)
	defer srv.Close()
	client := redmine.New(srv.URL, "test-key")

	cases := []struct{ in, want string }{
		{"Jane Doe", "3"},   // exact, case-insensitive
		{"jane doe", "3"},   //
		{"jane", "3"},       // unique substring
		{"Ahmed Khan", "4"}, //
	}
	for _, c := range cases {
		got, err := resolveUserFilter(client, "--assignee", c.in, "7")
		if err != nil {
			t.Errorf("resolveUserFilter(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolveUserFilter(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Assigning work to the wrong person is not recoverable by the caller, so an
// ambiguous name is an error listing the candidates rather than a guess.
func TestResolveUserFilterRejectsAmbiguousName(t *testing.T) {
	srv := membershipServer(t)
	defer srv.Close()
	client := redmine.New(srv.URL, "test-key")

	_, err := resolveUserFilter(client, "--assignee", "an", "7")
	if err == nil {
		t.Fatal("expected an error for a name matching several members")
	}
	for _, want := range []string{"Jane Doe", "Ahmed Khan", "numeric ID"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

func TestResolveUserFilterReportsUnknownName(t *testing.T) {
	srv := membershipServer(t)
	defer srv.Close()
	client := redmine.New(srv.URL, "test-key")

	_, err := resolveUserFilter(client, "--assignee", "Nobody", "7")
	if err == nil {
		t.Fatal("expected an error for a name that matches no member")
	}
	if !errors.Is(err, redmine.ErrNoMatch) {
		t.Errorf("error should wrap ErrNoMatch, got %v", err)
	}
}

// Groups hold project roles too, but cannot be an assignee.
func TestResolveUserFilterIgnoresGroups(t *testing.T) {
	srv := membershipServer(t)
	defer srv.Close()
	client := redmine.New(srv.URL, "test-key")

	if _, err := resolveUserFilter(client, "--assignee", "Developers", "7"); err == nil {
		t.Error("a group name should not resolve to an assignee")
	}
}

// membershipServer serves one project's members: two people whose names both
// contain "an", plus a group.
func membershipServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/7/memberships.json" {
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"memberships": []map[string]any{
				{"id": 1, "user": map[string]any{"id": 3, "name": "Jane Doe"}},
				{"id": 2, "user": map[string]any{"id": 4, "name": "Ahmed Khan"}},
				{"id": 3, "group": map[string]any{"id": 9, "name": "Developers"}},
			},
			"total_count": 3,
		})
	}))
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

func TestNormalizeSort(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"   ", ""},
		{"due_date", "due_date"},
		{"due_date:asc", "due_date:asc"},
		{"priority:desc,due_date:asc", "priority:desc,due_date:asc"},
		{"cf_12:desc", "cf_12:desc"}, // custom fields sort too, so no whitelist
		// Whitespace is stripped rather than forwarded into the query.
		{"priority:desc, due_date:asc", "priority:desc,due_date:asc"},
		{" due_date : asc ", "due_date:asc"},
	}
	for _, c := range cases {
		got, err := normalizeSort(c.in)
		if err != nil {
			t.Errorf("normalizeSort(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("normalizeSort(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	for _, bad := range []string{
		"due_date:up",
		"due_date:ASC",
		",due_date",
		"due_date,,priority",
	} {
		if _, err := normalizeSort(bad); err == nil {
			t.Errorf("normalizeSort(%q): expected an error", bad)
		}
	}
}
