package redmine

import (
	"fmt"
	"net/url"
	"strconv"
)

// TimeEntry is a logged unit of time against an issue or project.
type TimeEntry struct {
	ID       int       `json:"id"`
	Project  IDName    `json:"project"`
	Issue    *IssueRef `json:"issue"`
	User     IDName    `json:"user"`
	Activity IDName    `json:"activity"`
	Hours    float64   `json:"hours"`
	Comments string    `json:"comments"`
	SpentOn  string    `json:"spent_on"`
}

// TimeEntryListFilter narrows a `rmine time list` query. Empty fields are omitted.
type TimeEntryListFilter struct {
	IssueID   string
	ProjectID string
	UserID    string
	From      string // spent_on >= From (YYYY-MM-DD)
	To        string // spent_on <= To (YYYY-MM-DD)
	Limit     int
	All       bool
}

type timeEntryListResponse struct {
	TimeEntries []TimeEntry `json:"time_entries"`
	TotalCount  int         `json:"total_count"`
}

type timeEntryResponse struct {
	TimeEntry TimeEntry `json:"time_entry"`
}

// ListTimeEntries returns time entries matching the filter, paging through
// results when All is set or the requested Limit exceeds Redmine's per-page cap.
func (c *Client) ListTimeEntries(f TimeEntryListFilter) ([]TimeEntry, error) {
	const pageSize = 100

	base := url.Values{}
	if f.IssueID != "" {
		base.Set("issue_id", f.IssueID)
	}
	if f.ProjectID != "" {
		base.Set("project_id", f.ProjectID)
	}
	if f.UserID != "" {
		base.Set("user_id", f.UserID)
	}
	if f.From != "" || f.To != "" {
		base.Set("spent_on", spentOnRange(f.From, f.To))
	}

	want := f.Limit
	if f.All || want <= 0 {
		want = 0
	}

	var all []TimeEntry
	offset := 0
	for {
		q := url.Values{}
		for k, v := range base {
			q[k] = v
		}
		limit := pageSize
		if want > 0 && want-len(all) < pageSize {
			limit = want - len(all)
		}
		q.Set("limit", strconv.Itoa(limit))
		q.Set("offset", strconv.Itoa(offset))

		var resp timeEntryListResponse
		if err := c.get("/time_entries.json", q, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.TimeEntries...)
		offset += len(resp.TimeEntries)

		if len(resp.TimeEntries) == 0 || offset >= resp.TotalCount {
			break
		}
		if want > 0 && len(all) >= want {
			break
		}
	}

	if want > 0 && len(all) > want {
		all = all[:want]
	}
	return all, nil
}

// spentOnRange builds Redmine's range-filter syntax for the spent_on field.
func spentOnRange(from, to string) string {
	return dateRangeFilter(from, to)
}

// CreateTimeEntryRequest describes a new time entry. Exactly one of IssueID
// or ProjectID should be set, matching Redmine's requirement.
type CreateTimeEntryRequest struct {
	IssueID    int
	ProjectID  string
	Hours      float64
	ActivityID int
	Comments   string
	SpentOn    string // YYYY-MM-DD; empty means "today" per Redmine's default
}

// timeEntryFields is CreateTimeEntryRequest's wire form. It carries the same
// fields in the same order on purpose, so the two convert directly rather
// than needing a copy that has to be kept in step field by field.
type timeEntryFields struct {
	IssueID    int     `json:"issue_id,omitempty"`
	ProjectID  string  `json:"project_id,omitempty"`
	Hours      float64 `json:"hours,omitempty"`
	ActivityID int     `json:"activity_id,omitempty"`
	Comments   string  `json:"comments,omitempty"`
	SpentOn    string  `json:"spent_on,omitempty"`
}

// CreateTimeEntry logs a new time entry and returns it as stored by Redmine.
func (c *Client) CreateTimeEntry(req CreateTimeEntryRequest) (*TimeEntry, error) {
	body := struct {
		TimeEntry timeEntryFields `json:"time_entry"`
	}{
		TimeEntry: timeEntryFields(req),
	}

	var resp timeEntryResponse
	if err := c.post("/time_entries.json", body, &resp); err != nil {
		return nil, err
	}
	return &resp.TimeEntry, nil
}

// UpdateTimeEntryRequest describes an edit to an existing time entry. As with
// UpdateIssueRequest, only non-nil fields are sent, so clearing a comment is
// expressible where a value-typed struct could not tell an empty comment
// apart from an untouched one.
type UpdateTimeEntryRequest struct {
	Hours      *float64
	ActivityID *int
	Comments   *string
	SpentOn    *string
}

// fields renders the request as the sparse object Redmine expects.
func (r UpdateTimeEntryRequest) fields() map[string]any {
	m := map[string]any{}
	setIf(m, "hours", r.Hours)
	setIf(m, "activity_id", r.ActivityID)
	setIf(m, "comments", r.Comments)
	setIf(m, "spent_on", r.SpentOn)
	return m
}

// UpdateTimeEntry applies a partial update to a time entry.
func (c *Client) UpdateTimeEntry(id int, req UpdateTimeEntryRequest) error {
	body := map[string]any{"time_entry": req.fields()}
	return c.put(fmt.Sprintf("/time_entries/%d.json", id), body)
}

// DeleteTimeEntry removes a time entry.
func (c *Client) DeleteTimeEntry(id int) error {
	return c.delete(fmt.Sprintf("/time_entries/%d.json", id))
}
