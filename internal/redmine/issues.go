package redmine

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Issue is a Redmine issue.
type Issue struct {
	ID          int       `json:"id"`
	Project     IDName    `json:"project"`
	Tracker     IDName    `json:"tracker"`
	Status      IDName    `json:"status"`
	Priority    IDName    `json:"priority"`
	Author      IDName    `json:"author"`
	AssignedTo  *IDName   `json:"assigned_to"`
	Subject     string    `json:"subject"`
	Description string    `json:"description"`
	CreatedOn   time.Time `json:"created_on"`
	UpdatedOn   time.Time `json:"updated_on"`
}

// IssueListFilter narrows a `rdm issue list` query. Empty fields are omitted.
type IssueListFilter struct {
	ProjectID  string
	StatusID   string
	AssignedTo string
	TrackerID  string
	Limit      int  // 0 means "use Redmine's default page size"
	All        bool // ignore Limit and fetch every matching issue
}

type issueListResponse struct {
	Issues     []Issue `json:"issues"`
	TotalCount int     `json:"total_count"`
}

type issueResponse struct {
	Issue Issue `json:"issue"`
}

// ListIssues returns issues matching the filter, paging through results when
// All is set or the requested Limit exceeds Redmine's per-page cap.
func (c *Client) ListIssues(f IssueListFilter) ([]Issue, error) {
	const pageSize = 100

	base := url.Values{}
	if f.ProjectID != "" {
		base.Set("project_id", f.ProjectID)
	}
	if f.StatusID != "" {
		base.Set("status_id", f.StatusID)
	}
	if f.AssignedTo != "" {
		base.Set("assigned_to_id", f.AssignedTo)
	}
	if f.TrackerID != "" {
		base.Set("tracker_id", f.TrackerID)
	}

	want := f.Limit
	if f.All || want <= 0 {
		want = 0 // unbounded
	}

	var all []Issue
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

		var resp issueListResponse
		if err := c.get("/issues.json", q, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Issues...)
		offset += len(resp.Issues)

		if len(resp.Issues) == 0 || offset >= resp.TotalCount {
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

// GetIssue fetches a single issue by ID.
func (c *Client) GetIssue(id int) (*Issue, error) {
	var resp issueResponse
	if err := c.get(fmt.Sprintf("/issues/%d.json", id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Issue, nil
}

// CreateIssueRequest describes a new issue. Project and Subject are required
// by Redmine; the rest are optional and omitted from the request when zero.
type CreateIssueRequest struct {
	ProjectID   string
	Subject     string
	Description string
	TrackerID   int
	PriorityID  int
	AssignedTo  int
}

type issueFields struct {
	ProjectID   string `json:"project_id,omitempty"`
	Subject     string `json:"subject,omitempty"`
	Description string `json:"description,omitempty"`
	TrackerID   int    `json:"tracker_id,omitempty"`
	PriorityID  int    `json:"priority_id,omitempty"`
	AssignedTo  int    `json:"assigned_to_id,omitempty"`
	StatusID    int    `json:"status_id,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

// CreateIssue creates a new issue and returns it as stored by Redmine.
func (c *Client) CreateIssue(req CreateIssueRequest) (*Issue, error) {
	body := struct {
		Issue issueFields `json:"issue"`
	}{
		Issue: issueFields{
			ProjectID:   req.ProjectID,
			Subject:     req.Subject,
			Description: req.Description,
			TrackerID:   req.TrackerID,
			PriorityID:  req.PriorityID,
			AssignedTo:  req.AssignedTo,
		},
	}

	var resp issueResponse
	if err := c.post("/issues.json", body, &resp); err != nil {
		return nil, err
	}
	return &resp.Issue, nil
}

// UpdateIssueRequest describes an edit to an existing issue. Zero-value
// fields are left unchanged on the server.
type UpdateIssueRequest struct {
	Subject     string
	Description string
	TrackerID   int
	PriorityID  int
	AssignedTo  int
	StatusID    int
}

// UpdateIssue applies a partial update to an issue.
func (c *Client) UpdateIssue(id int, req UpdateIssueRequest) error {
	body := struct {
		Issue issueFields `json:"issue"`
	}{
		Issue: issueFields{
			Subject:     req.Subject,
			Description: req.Description,
			TrackerID:   req.TrackerID,
			PriorityID:  req.PriorityID,
			AssignedTo:  req.AssignedTo,
			StatusID:    req.StatusID,
		},
	}
	return c.put(fmt.Sprintf("/issues/%d.json", id), body)
}

// AddNote appends a comment to an issue via Redmine's notes field.
func (c *Client) AddNote(id int, note string) error {
	body := struct {
		Issue issueFields `json:"issue"`
	}{Issue: issueFields{Notes: note}}
	return c.put(fmt.Sprintf("/issues/%d.json", id), body)
}
