package redmine

import (
	"errors"
	"fmt"
	"strings"
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

func findIDByName(items []IDName, name string) (int, error) {
	for _, item := range items {
		if strings.EqualFold(item.Name, name) {
			return item.ID, nil
		}
	}
	return 0, fmt.Errorf("%w for %q", ErrNoMatch, name)
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
	for _, s := range statuses {
		if strings.EqualFold(s.Name, name) {
			return s.ID, nil
		}
	}
	return 0, fmt.Errorf("status: %w for %q", ErrNoMatch, name)
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
