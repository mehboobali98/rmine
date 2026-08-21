package redmine

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Issue is a Redmine issue.
//
// The scheduling fields — due date, start date, progress, estimate — are
// carried even though rmine long filtered on due dates without ever reading
// one back. Anything omitted here is dropped from `issue view -o json` too,
// which is the source of truth the skill file points agents at, so a field
// missing from this struct is a question rmine simply cannot answer.
type Issue struct {
	ID             int           `json:"id"`
	Project        IDName        `json:"project"`
	Tracker        IDName        `json:"tracker"`
	Status         IDName        `json:"status"`
	Priority       IDName        `json:"priority"`
	Author         IDName        `json:"author"`
	AssignedTo     *IDName       `json:"assigned_to"`
	Subject        string        `json:"subject"`
	Description    string        `json:"description"`
	Category       *IDName       `json:"category,omitempty"`
	Parent         *IssueRef     `json:"parent,omitempty"`
	FixedVersion   *IDName       `json:"fixed_version,omitempty"`
	StartDate      string        `json:"start_date,omitempty"`
	DueDate        string        `json:"due_date,omitempty"`
	DoneRatio      int           `json:"done_ratio"`
	EstimatedHours *float64      `json:"estimated_hours,omitempty"`
	SpentHours     *float64      `json:"spent_hours,omitempty"`
	CustomFields   []CustomField `json:"custom_fields,omitempty"`
	Attachments    []Attachment  `json:"attachments,omitempty"`
	Journals       []Journal     `json:"journals,omitempty"`
	CreatedOn      time.Time     `json:"created_on"`
	UpdatedOn      time.Time     `json:"updated_on"`
}

// IssueRef is a bare reference to another issue — what Redmine embeds for an
// issue's parent, and for the issue a time entry was logged against.
type IssueRef struct {
	ID int `json:"id"`
}

// Attachment is a file attached to an issue. ContentURL is an absolute URL on
// the same server; fetch it with Client.Download.
type Attachment struct {
	ID          int       `json:"id"`
	Filename    string    `json:"filename"`
	Filesize    int       `json:"filesize"`
	ContentType string    `json:"content_type"`
	Description string    `json:"description"`
	ContentURL  string    `json:"content_url"`
	Author      IDName    `json:"author"`
	CreatedOn   time.Time `json:"created_on"`
}

// Journal is one entry in an issue's history. Redmine records field changes
// and comments in the same list, so an entry with empty Notes is a bare field
// change with nothing a reader would call a comment.
type Journal struct {
	ID        int       `json:"id"`
	User      IDName    `json:"user"`
	Notes     string    `json:"notes"`
	CreatedOn time.Time `json:"created_on"`
}

// CustomField is one value of an issue's custom fields, which are defined
// per Redmine instance (and sometimes per project/tracker) by each server's
// admin and can't be known ahead of time. Name is populated by Redmine on
// read; set fields by ID on write — find the ID for a field by inspecting
// an existing issue via `rmine issue view <id> -o json`.
//
// Values, when it has 2+ elements, marshals as a JSON array instead of
// Value's plain string — Redmine requires an array to set more than one
// option on a checkbox/multi-select field. Leave Values empty and set Value
// for every single-value field (the common case).
type CustomField struct {
	ID     int
	Name   string
	Value  FieldValue
	Values []string
}

func (f CustomField) MarshalJSON() ([]byte, error) {
	wire := struct {
		ID    int    `json:"id"`
		Name  string `json:"name,omitempty"`
		Value any    `json:"value"`
	}{ID: f.ID, Name: f.Name}
	if len(f.Values) > 0 {
		wire.Value = f.Values
	} else {
		wire.Value = string(f.Value)
	}
	return json.Marshal(wire)
}

func (f *CustomField) UnmarshalJSON(data []byte) error {
	var wire struct {
		ID    int        `json:"id"`
		Name  string     `json:"name,omitempty"`
		Value FieldValue `json:"value"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	f.ID, f.Name, f.Value = wire.ID, wire.Name, wire.Value
	return nil
}

// FieldValue is a custom field's value. Redmine encodes single-value fields
// as a JSON string but multi-value fields (checkboxes, multi-selects) as a
// JSON array of strings; both unmarshal here as one string, joined with
// ", " in the multi-value case. To write more than one value back, set
// CustomField.Values instead.
type FieldValue string

func (v *FieldValue) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*v = FieldValue(s)
		return nil
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("custom field value is neither a string nor a string array: %w", err)
	}
	*v = FieldValue(strings.Join(list, ", "))
	return nil
}

// IssueListFilter narrows a `rmine issue list` query. Empty fields are omitted.
type IssueListFilter struct {
	ProjectID     string
	StatusID      string
	AssignedTo    string
	TrackerID     string
	Subject       string // substring match against the issue subject
	UpdatedAfter  string // YYYY-MM-DD
	UpdatedBefore string // YYYY-MM-DD
	DueAfter      string // YYYY-MM-DD
	DueBefore     string // YYYY-MM-DD
	Limit         int    // 0 means "use Redmine's default page size"
	All           bool   // ignore Limit and fetch every matching issue
}

type issueListResponse struct {
	Issues     []Issue `json:"issues"`
	TotalCount int     `json:"total_count"`
}

type issueResponse struct {
	Issue Issue `json:"issue"`
}

// buildAdvancedIssueFilter translates IssueListFilter into Redmine's
// f[]/op[field]/v[field][] advanced filter syntax, the only form that
// supports the subject "contains" operator.
func buildAdvancedIssueFilter(f IssueListFilter) url.Values {
	q := url.Values{}
	addField := func(field, op string, values ...string) {
		q.Add("f[]", field)
		q.Set("op["+field+"]", op)
		for _, v := range values {
			q.Add("v["+field+"][]", v)
		}
	}

	if f.ProjectID != "" {
		addField("project_id", "=", f.ProjectID)
	}
	if f.TrackerID != "" {
		addField("tracker_id", "=", f.TrackerID)
	}
	if f.AssignedTo != "" {
		addField("assigned_to_id", "=", f.AssignedTo)
	}
	if f.StatusID != "" {
		switch f.StatusID {
		case "open":
			addField("status_id", "o")
		case "closed":
			addField("status_id", "c")
		case "*":
			addField("status_id", "*")
		default:
			addField("status_id", "=", f.StatusID)
		}
	}
	switch {
	case f.UpdatedAfter != "" && f.UpdatedBefore != "":
		addField("updated_on", "><", f.UpdatedAfter, f.UpdatedBefore)
	case f.UpdatedAfter != "":
		addField("updated_on", ">=", f.UpdatedAfter)
	case f.UpdatedBefore != "":
		addField("updated_on", "<=", f.UpdatedBefore)
	}
	switch {
	case f.DueAfter != "" && f.DueBefore != "":
		addField("due_date", "><", f.DueAfter, f.DueBefore)
	case f.DueAfter != "":
		addField("due_date", ">=", f.DueAfter)
	case f.DueBefore != "":
		addField("due_date", "<=", f.DueBefore)
	}
	if f.Subject != "" {
		addField("subject", "~", f.Subject)
	}
	return q
}

// ListIssues returns issues matching the filter, paging through results when
// All is set or the requested Limit exceeds Redmine's per-page cap.
func (c *Client) ListIssues(f IssueListFilter) ([]Issue, error) {
	const pageSize = 100

	// Redmine's issues.json only reads params[:f] (the advanced filter
	// array) OR the simple field params (project_id=, status_id=, ...) —
	// never both. A subject search needs the advanced form (op "~" for
	// "contains" isn't expressible as a simple param), so once Subject is
	// set every other active filter has to move to the advanced form too,
	// or it would be silently ignored.
	var base url.Values
	if f.Subject != "" {
		base = buildAdvancedIssueFilter(f)
	} else {
		base = url.Values{}
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
		if f.UpdatedAfter != "" || f.UpdatedBefore != "" {
			base.Set("updated_on", dateRangeFilter(f.UpdatedAfter, f.UpdatedBefore))
		}
		if f.DueAfter != "" || f.DueBefore != "" {
			base.Set("due_date", dateRangeFilter(f.DueAfter, f.DueBefore))
		}
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

// GetIssue fetches a single issue by ID. Attachments always come along — they
// are a handful of small keys. Comments (Redmine calls them journals) only
// when asked: a long-running issue's history is far bigger than the issue
// itself, and most callers don't want it.
func (c *Client) GetIssue(id int, withComments bool) (*Issue, error) {
	includes := "attachments"
	if withComments {
		includes += ",journals"
	}
	query := url.Values{"include": {includes}}

	var resp issueResponse
	if err := c.get(fmt.Sprintf("/issues/%d.json", id), query, &resp); err != nil {
		return nil, err
	}
	return &resp.Issue, nil
}

// CreateIssueRequest describes a new issue. Project and Subject are required
// by Redmine; the rest are optional and omitted from the request when zero.
type CreateIssueRequest struct {
	ProjectID      string
	Subject        string
	Description    string
	TrackerID      int
	PriorityID     int
	AssignedTo     int
	CategoryID     int
	ParentID       int
	StartDate      string
	DueDate        string
	EstimatedHours float64
	DoneRatio      int
	CustomFields   []CustomField
}

type issueFields struct {
	ProjectID      string        `json:"project_id,omitempty"`
	Subject        string        `json:"subject,omitempty"`
	Description    string        `json:"description,omitempty"`
	TrackerID      int           `json:"tracker_id,omitempty"`
	PriorityID     int           `json:"priority_id,omitempty"`
	AssignedTo     int           `json:"assigned_to_id,omitempty"`
	StatusID       int           `json:"status_id,omitempty"`
	CategoryID     int           `json:"category_id,omitempty"`
	ParentID       int           `json:"parent_issue_id,omitempty"`
	StartDate      string        `json:"start_date,omitempty"`
	DueDate        string        `json:"due_date,omitempty"`
	EstimatedHours float64       `json:"estimated_hours,omitempty"`
	DoneRatio      int           `json:"done_ratio,omitempty"`
	Notes          string        `json:"notes,omitempty"`
	CustomFields   []CustomField `json:"custom_fields,omitempty"`
}

// CreateIssue creates a new issue and returns it as stored by Redmine.
func (c *Client) CreateIssue(req CreateIssueRequest) (*Issue, error) {
	body := struct {
		Issue issueFields `json:"issue"`
	}{
		Issue: issueFields{
			ProjectID:      req.ProjectID,
			Subject:        req.Subject,
			Description:    req.Description,
			TrackerID:      req.TrackerID,
			PriorityID:     req.PriorityID,
			AssignedTo:     req.AssignedTo,
			CategoryID:     req.CategoryID,
			ParentID:       req.ParentID,
			StartDate:      req.StartDate,
			DueDate:        req.DueDate,
			EstimatedHours: req.EstimatedHours,
			DoneRatio:      req.DoneRatio,
			CustomFields:   req.CustomFields,
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
	Subject        string
	Description    string
	TrackerID      int
	PriorityID     int
	AssignedTo     int
	StatusID       int
	CategoryID     int
	ParentID       int
	StartDate      string
	DueDate        string
	EstimatedHours float64
	DoneRatio      int
	CustomFields   []CustomField
}

// UpdateIssue applies a partial update to an issue.
func (c *Client) UpdateIssue(id int, req UpdateIssueRequest) error {
	body := struct {
		Issue issueFields `json:"issue"`
	}{
		Issue: issueFields{
			Subject:        req.Subject,
			Description:    req.Description,
			TrackerID:      req.TrackerID,
			PriorityID:     req.PriorityID,
			AssignedTo:     req.AssignedTo,
			StatusID:       req.StatusID,
			CategoryID:     req.CategoryID,
			ParentID:       req.ParentID,
			StartDate:      req.StartDate,
			DueDate:        req.DueDate,
			EstimatedHours: req.EstimatedHours,
			DoneRatio:      req.DoneRatio,
			CustomFields:   req.CustomFields,
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
