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
	Children       []IssueChild  `json:"children,omitempty"`
	Relations      []Relation    `json:"relations,omitempty"`
	Journals       []Journal     `json:"journals,omitempty"`
	CreatedOn      time.Time     `json:"created_on"`
	UpdatedOn      time.Time     `json:"updated_on"`
}

// IssueChild is a subtask as Redmine embeds it under an issue's children.
// It carries only enough to identify the child; fetch it by ID for the rest.
// Children nest, so a whole subtask tree arrives in one response.
type IssueChild struct {
	ID       int          `json:"id"`
	Tracker  IDName       `json:"tracker"`
	Subject  string       `json:"subject"`
	Children []IssueChild `json:"children,omitempty"`
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
	VersionID     string // fixed_version_id; "*" for any, "!*" for none
	ParentID      string // direct children of this issue
	CustomFields  []CustomFieldFilter
	Sort          string // Redmine sort spec, e.g. "due_date:asc,priority:desc"
	Limit         int    // 0 means "use Redmine's default page size"
	All           bool   // ignore Limit and fetch every matching issue
}

// CustomFieldFilter matches issues whose custom field ID equals any of
// Values. A single value of "*" matches any set value and "!*" an unset one.
type CustomFieldFilter struct {
	ID     int
	Values []string
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
	if f.VersionID != "" {
		switch f.VersionID {
		case "*", "!*":
			addField("fixed_version_id", f.VersionID)
		default:
			addField("fixed_version_id", "=", f.VersionID)
		}
	}
	if f.ParentID != "" {
		addField("parent_id", "=", f.ParentID)
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
	for _, cf := range f.CustomFields {
		field := "cf_" + strconv.Itoa(cf.ID)
		if len(cf.Values) == 1 && (cf.Values[0] == "*" || cf.Values[0] == "!*") {
			addField(field, cf.Values[0])
		} else {
			addField(field, "=", cf.Values...)
		}
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
	// "contains" isn't expressible as a simple param), and so do custom
	// fields, whose simple cf_N=value form reads a leading "!", "~" or "*"
	// in the value as an operator. Once either is set every other active
	// filter has to move to the advanced form too, or it would be silently
	// ignored.
	var base url.Values
	if f.Subject != "" || len(f.CustomFields) > 0 {
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
		if f.VersionID != "" {
			base.Set("fixed_version_id", f.VersionID)
		}
		if f.ParentID != "" {
			base.Set("parent_id", f.ParentID)
		}
		if f.UpdatedAfter != "" || f.UpdatedBefore != "" {
			base.Set("updated_on", dateRangeFilter(f.UpdatedAfter, f.UpdatedBefore))
		}
		if f.DueAfter != "" || f.DueBefore != "" {
			base.Set("due_date", dateRangeFilter(f.DueAfter, f.DueBefore))
		}
	}

	if f.Sort != "" {
		base.Set("sort", f.Sort)
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

// GetIssueOptions selects what Redmine embeds alongside the issue.
//
// Every one of these is an `include` the API will not send unless asked, and
// a field that is never asked for is indistinguishable from one that is
// empty — an issue's subtasks read as "no subtasks" rather than as "not
// fetched". Attachments are not listed here because they always come along:
// they are a handful of small keys and the one thing every caller wanted.
type GetIssueOptions struct {
	// Comments fetches the issue's journals. Off by default because a
	// long-running issue's history is far bigger than the issue itself.
	Comments bool
	// Children fetches the subtask tree.
	Children bool
	// Relations fetches typed links to other issues.
	Relations bool
}

// FullIssue asks for everything except comments — the detail view's default.
// Children and relations are bounded and small, unlike a journal list, so
// there is nothing to gain by making a caller opt into them one at a time.
func FullIssue() GetIssueOptions {
	return GetIssueOptions{Children: true, Relations: true}
}

// GetIssue fetches a single issue by ID.
func (c *Client) GetIssue(id int, opts GetIssueOptions) (*Issue, error) {
	includes := []string{"attachments"}
	if opts.Comments {
		includes = append(includes, "journals")
	}
	if opts.Children {
		includes = append(includes, "children")
	}
	if opts.Relations {
		includes = append(includes, "relations")
	}
	query := url.Values{"include": {strings.Join(includes, ",")}}

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
	FixedVersionID int
	StartDate      string
	DueDate        string
	EstimatedHours float64
	DoneRatio      int
	CustomFields   []CustomField
	Uploads        []Upload
}

// issueFields is the payload for the two writes that always send a fixed set
// of fields: creating an issue, and appending a note. Updates build a sparse
// map instead — see UpdateIssueRequest.fields.
type issueFields struct {
	ProjectID      string        `json:"project_id,omitempty"`
	Subject        string        `json:"subject,omitempty"`
	Description    string        `json:"description,omitempty"`
	TrackerID      int           `json:"tracker_id,omitempty"`
	PriorityID     int           `json:"priority_id,omitempty"`
	AssignedTo     int           `json:"assigned_to_id,omitempty"`
	CategoryID     int           `json:"category_id,omitempty"`
	ParentID       int           `json:"parent_issue_id,omitempty"`
	FixedVersionID int           `json:"fixed_version_id,omitempty"`
	StartDate      string        `json:"start_date,omitempty"`
	DueDate        string        `json:"due_date,omitempty"`
	EstimatedHours float64       `json:"estimated_hours,omitempty"`
	DoneRatio      int           `json:"done_ratio,omitempty"`
	Notes          string        `json:"notes,omitempty"`
	CustomFields   []CustomField `json:"custom_fields,omitempty"`
	Uploads        []Upload      `json:"uploads,omitempty"`
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
			FixedVersionID: req.FixedVersionID,
			StartDate:      req.StartDate,
			DueDate:        req.DueDate,
			EstimatedHours: req.EstimatedHours,
			DoneRatio:      req.DoneRatio,
			CustomFields:   req.CustomFields,
			Uploads:        req.Uploads,
		},
	}

	var resp issueResponse
	if err := c.post("/issues.json", body, &resp); err != nil {
		return nil, err
	}
	return &resp.Issue, nil
}

// UpdateIssueRequest describes an edit to an existing issue.
//
// Every field is a pointer, and only the non-nil ones are sent: a nil field
// is left exactly as it is on the server, while a pointer to a zero value is
// transmitted explicitly. That distinction is what makes a field clearable —
// the previous value-typed struct could not tell "leave the description
// alone" apart from "set the description to empty", and silently chose the
// former for both.
type UpdateIssueRequest struct {
	Subject        *string
	Description    *string
	TrackerID      *int
	PriorityID     *int
	StatusID       *int
	AssignedTo     *int // 0 clears the assignee
	CategoryID     *int // 0 clears the category
	ParentID       *int // 0 detaches from the parent issue
	FixedVersionID *int // 0 clears the target version
	StartDate      *string
	DueDate        *string
	EstimatedHours *float64 // 0 clears the estimate
	DoneRatio      *int     // 0 is a real value, not a clear
	CustomFields   []CustomField

	// Notes records a journal comment against this edit. Redmine files it on
	// the same journal entry as the field changes, so the note explains the
	// change instead of trailing it as a separate remark.
	Notes *string

	// Uploads attaches files staged by Client.UploadFile.
	Uploads []Upload
}

// fields renders the request as the sparse object Redmine expects.
func (r UpdateIssueRequest) fields() map[string]any {
	m := map[string]any{}
	setIf(m, "subject", r.Subject)
	setIf(m, "description", r.Description)
	setIf(m, "tracker_id", r.TrackerID)
	setIf(m, "priority_id", r.PriorityID)
	setIf(m, "status_id", r.StatusID)
	setIf(m, "start_date", r.StartDate)
	setIf(m, "due_date", r.DueDate)
	setIf(m, "done_ratio", r.DoneRatio)
	if r.AssignedTo != nil {
		m["assigned_to_id"] = clearable(*r.AssignedTo)
	}
	if r.CategoryID != nil {
		m["category_id"] = clearable(*r.CategoryID)
	}
	if r.ParentID != nil {
		m["parent_issue_id"] = clearable(*r.ParentID)
	}
	if r.EstimatedHours != nil {
		m["estimated_hours"] = clearable(*r.EstimatedHours)
	}
	if r.FixedVersionID != nil {
		m["fixed_version_id"] = clearable(*r.FixedVersionID)
	}
	setIf(m, "notes", r.Notes)
	if len(r.CustomFields) > 0 {
		m["custom_fields"] = r.CustomFields
	}
	if len(r.Uploads) > 0 {
		m["uploads"] = r.Uploads
	}
	return m
}

// setIf adds a field to the payload when the caller set it.
func setIf[T any](m map[string]any, key string, v *T) {
	if v != nil {
		m[key] = *v
	}
}

// clearable renders a numeric field that Redmine clears when given an empty
// string. Sending 0 for these would be rejected as an invalid id rather than
// understood as "unset it".
func clearable[T int | float64](v T) any {
	if v <= 0 {
		return ""
	}
	return v
}

// UpdateIssue applies a partial update to an issue.
func (c *Client) UpdateIssue(id int, req UpdateIssueRequest) error {
	body := map[string]any{"issue": req.fields()}
	return c.put(fmt.Sprintf("/issues/%d.json", id), body)
}

// AddNote appends a comment to an issue, optionally carrying files staged by
// Client.UploadFile. Redmine files the attachments on the same journal entry
// as the note.
func (c *Client) AddNote(id int, note string, uploads []Upload) error {
	body := struct {
		Issue issueFields `json:"issue"`
	}{Issue: issueFields{Notes: note, Uploads: uploads}}
	return c.put(fmt.Sprintf("/issues/%d.json", id), body)
}

// MissingCustomFields reports which of the custom fields a write asked for
// are absent from the issue Redmine stored.
//
// Redmine exposes only a subset of an instance's custom fields on any given
// tracker, and a write naming a field outside that subset is not rejected —
// it is accepted, answered with 200, and dropped. Nothing in the response
// distinguishes that from a field that was stored, except that the field is
// missing from custom_fields entirely: Redmine lists every field the tracker
// *does* expose, set or not, so absence is a reliable signal rather than a
// guess about empty values.
//
// The IDs come back in the order they were requested, so a message built from
// them names the fields in the order the caller typed them.
func MissingCustomFields(requested, stored []CustomField) []int {
	if len(requested) == 0 {
		return nil
	}

	present := make(map[int]bool, len(stored))
	for _, f := range stored {
		present[f.ID] = true
	}

	var missing []int
	seen := make(map[int]bool, len(requested))
	for _, f := range requested {
		if present[f.ID] || seen[f.ID] {
			continue
		}
		seen[f.ID] = true
		missing = append(missing, f.ID)
	}
	return missing
}
