package redmine

import (
	"net/url"
	"strconv"
)

// TrackerCustomFields returns the custom fields Redmine attaches to issues of
// one tracker in one project, read off the most recent such issue. Redmine
// lists every field the tracker enables on an issue, set or not, so one issue
// is enough. found is false when the project has no issue of that tracker.
//
// This stands in for /custom_fields.json, which is admin-only, and for the
// project's issue_custom_fields include, which needs Redmine 4.2 and does not
// say which tracker a field belongs to.
func (c *Client) TrackerCustomFields(projectIDOrIdentifier string, trackerID int) (fields []CustomField, found bool, err error) {
	q := url.Values{}
	q.Set("project_id", projectIDOrIdentifier)
	q.Set("subproject_id", "!*")
	q.Set("tracker_id", strconv.Itoa(trackerID))
	q.Set("status_id", "*")
	q.Set("sort", "id:desc")
	q.Set("limit", "1")

	var resp issueListResponse
	if err := c.get("/issues.json", q, &resp); err != nil {
		return nil, false, err
	}
	if len(resp.Issues) == 0 {
		return nil, false, nil
	}
	return resp.Issues[0].CustomFields, true, nil
}
