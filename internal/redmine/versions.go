package redmine

import (
	"fmt"
	"net/url"
)

// Version is a project's target version (Redmine's fixed_version) — the
// milestone or sprint an issue is scheduled for.
//
// Status is "open", "locked" or "closed"; only open versions accept new
// issues. Sharing says how far down the project tree the version is visible
// ("none", "descendants", "hierarchy", "tree", "system"), which is why a
// version listed on a parent project can legitimately be set on a child's
// issue.
type Version struct {
	ID          int    `json:"id"`
	Project     IDName `json:"project"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	DueDate     string `json:"due_date,omitempty"`
	Sharing     string `json:"sharing,omitempty"`
}

// ListVersions returns the versions available on one project, including any
// shared down from its ancestors.
func (c *Client) ListVersions(projectIDOrIdentifier string) ([]Version, error) {
	var resp struct {
		Versions []Version `json:"versions"`
	}
	path := "/projects/" + url.PathEscape(projectIDOrIdentifier) + "/versions.json"
	if err := c.get(path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Versions, nil
}

// ResolveVersionID resolves a version name, matched case-insensitively, to
// its numeric ID within the given project.
//
// Versions are project-scoped and their names repeat across projects ("2.0"
// exists on most of them), so unlike a tracker or a status this lookup cannot
// be done server-wide — the caller has to supply the project.
func (c *Client) ResolveVersionID(projectIDOrIdentifier, name string) (int, error) {
	versions, err := c.ListVersions(projectIDOrIdentifier)
	if err != nil {
		return 0, err
	}
	names := make([]IDName, 0, len(versions))
	for _, v := range versions {
		names = append(names, IDName{ID: v.ID, Name: v.Name})
	}
	id, err := findIDByName(names, name)
	if err != nil {
		return 0, fmt.Errorf("version: %w", err)
	}
	return id, nil
}
