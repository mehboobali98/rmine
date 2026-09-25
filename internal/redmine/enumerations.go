package redmine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ErrNoMatch reports that a lookup completed and nothing matched the given
// name — as opposed to the lookup itself failing. Callers need to tell the
// two apart: "you misspelled the project" and "we could not reach Redmine to
// check" call for different handling.
var ErrNoMatch = errors.New("no match")

// IssueStatus is a Redmine issue status, including whether it's a "closed" state.
type IssueStatus struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	IsClosed bool   `json:"is_closed"`
}

// ListTrackers returns every tracker (Bug, Feature, ...) defined on the server.
func (c *Client) ListTrackers() ([]IDName, error) {
	var resp struct {
		Trackers []IDName `json:"trackers"`
	}
	if err := c.get("/trackers.json", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Trackers, nil
}

// ListIssueStatuses returns every issue status defined on the server.
func (c *Client) ListIssueStatuses() ([]IssueStatus, error) {
	var resp struct {
		IssueStatuses []IssueStatus `json:"issue_statuses"`
	}
	if err := c.get("/issue_statuses.json", nil, &resp); err != nil {
		return nil, err
	}
	return resp.IssueStatuses, nil
}

// ListIssuePriorities returns every issue priority defined on the server.
func (c *Client) ListIssuePriorities() ([]IDName, error) {
	var resp struct {
		IssuePriorities []IDName `json:"issue_priorities"`
	}
	if err := c.get("/enumerations/issue_priorities.json", nil, &resp); err != nil {
		return nil, err
	}
	return resp.IssuePriorities, nil
}

// ListTimeEntryActivities returns every time-entry activity (Development,
// Design, ...) defined on the server.
func (c *Client) ListTimeEntryActivities() ([]IDName, error) {
	var resp struct {
		TimeEntryActivities []IDName `json:"time_entry_activities"`
	}
	if err := c.get("/enumerations/time_entry_activities.json", nil, &resp); err != nil {
		return nil, err
	}
	return resp.TimeEntryActivities, nil
}

// findIDByName matches name case-insensitively, then falls back to ignoring
// whitespace, hyphens and underscores. A loose spelling that fits more than
// one item is an error.
func findIDByName(items []IDName, name string) (int, error) {
	for _, item := range items {
		if strings.EqualFold(item.Name, name) {
			return item.ID, nil
		}
	}

	want := looseName(name)
	var loose []IDName
	for _, item := range items {
		if want != "" && looseName(item.Name) == want {
			loose = append(loose, item)
		}
	}
	switch len(loose) {
	case 0:
		return 0, noMatch(name, items)
	case 1:
		return loose[0].ID, nil
	default:
		names := make([]string, 0, len(loose))
		for _, item := range loose {
			names = append(names, strconv.Quote(item.Name))
		}
		return 0, fmt.Errorf("%q is ambiguous, it could be any of: %s", name, strings.Join(names, ", "))
	}
}

// looseName lowercases s and drops whitespace, hyphens and underscores.
func looseName(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '-' || r == '_' {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}

// maxListedCandidates caps how many valid names a rejection spells out.
// Trackers and statuses are short lists, but categories and versions run to
// hundreds on a long-lived project, and a screen of names buries the sentence
// that matters.
const maxListedCandidates = 15

// noMatch builds the error for a name that resolved to nothing, listing what
// would have worked.
//
// The bare "no match" this replaces left the caller to go find the valid
// spellings somewhere else — and for trackers, statuses, priorities and
// activities there was nowhere else in rmine to look. Naming the candidates
// turns every failed lookup into the enumeration the caller needed, which is
// most of what a discovery command would have been for.
func noMatch(name string, items []IDName) error {
	if len(items) == 0 {
		return fmt.Errorf("%w for %q", ErrNoMatch, name)
	}

	shown := items
	suffix := ""
	if len(shown) > maxListedCandidates {
		shown = shown[:maxListedCandidates]
		suffix = fmt.Sprintf(", ... (%d more)", len(items)-maxListedCandidates)
	}

	names := make([]string, 0, len(shown))
	for _, item := range shown {
		names = append(names, item.Name)
	}
	return fmt.Errorf("%w for %q (available: %s%s)", ErrNoMatch, name, strings.Join(names, ", "), suffix)
}

// ResolveTrackerID resolves a tracker name (e.g. "Bug") to its ID.
func (c *Client) ResolveTrackerID(name string) (int, error) {
	items, err := c.ListTrackers()
	if err != nil {
		return 0, err
	}
	id, err := findIDByName(items, name)
	if err != nil {
		return 0, fmt.Errorf("tracker: %w", err)
	}
	return id, nil
}

// ResolveIssuePriorityID resolves a priority name (e.g. "High") to its ID.
func (c *Client) ResolveIssuePriorityID(name string) (int, error) {
	items, err := c.ListIssuePriorities()
	if err != nil {
		return 0, err
	}
	id, err := findIDByName(items, name)
	if err != nil {
		return 0, fmt.Errorf("priority: %w", err)
	}
	return id, nil
}

// ResolveTimeEntryActivityID resolves an activity name (e.g. "Development") to its ID.
func (c *Client) ResolveTimeEntryActivityID(name string) (int, error) {
	items, err := c.ListTimeEntryActivities()
	if err != nil {
		return 0, err
	}
	id, err := findIDByName(items, name)
	if err != nil {
		return 0, fmt.Errorf("activity: %w", err)
	}
	return id, nil
}

// ResolveIssueStatusID resolves a status name (e.g. "Resolved") to its ID.
func (c *Client) ResolveIssueStatusID(name string) (int, error) {
	statuses, err := c.ListIssueStatuses()
	if err != nil {
		return 0, err
	}
	names := make([]IDName, 0, len(statuses))
	for _, s := range statuses {
		names = append(names, IDName{ID: s.ID, Name: s.Name})
	}
	id, err := findIDByName(names, name)
	if err != nil {
		return 0, fmt.Errorf("status: %w", err)
	}
	return id, nil
}

// DefaultClosedStatusID returns the ID of the first status flagged as
// "closed", used by `rmine issue close` when no explicit --status is given.
func (c *Client) DefaultClosedStatusID() (int, error) {
	statuses, err := c.ListIssueStatuses()
	if err != nil {
		return 0, err
	}
	for _, s := range statuses {
		if s.IsClosed {
			return s.ID, nil
		}
	}
	return 0, fmt.Errorf("no closed status defined on this server")
}
