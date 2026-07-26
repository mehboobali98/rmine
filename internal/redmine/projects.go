package redmine

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Project is a Redmine project.
type Project struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
	Status      int    `json:"status"`
}

type projectListResponse struct {
	Projects   []Project `json:"projects"`
	TotalCount int       `json:"total_count"`
}

type projectResponse struct {
	Project Project `json:"project"`
}

// ListProjects returns every project visible to the authenticated user,
// paging through Redmine's default 25-per-page limit automatically.
func (c *Client) ListProjects() ([]Project, error) {
	const pageSize = 100
	var all []Project
	offset := 0
	for {
		q := url.Values{}
		q.Set("limit", strconv.Itoa(pageSize))
		q.Set("offset", strconv.Itoa(offset))

		var resp projectListResponse
		if err := c.get("/projects.json", q, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Projects...)

		offset += len(resp.Projects)
		if len(resp.Projects) == 0 || offset >= resp.TotalCount {
			break
		}
	}
	return all, nil
}

// GetProject fetches a single project by numeric ID or string identifier.
func (c *Client) GetProject(idOrIdentifier string) (*Project, error) {
	var resp projectResponse
	if err := c.get(fmt.Sprintf("/projects/%s.json", idOrIdentifier), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Project, nil
}

// ResolveProjectID resolves a project's display name or identifier, matched
// case-insensitively, to its numeric ID.
func (c *Client) ResolveProjectID(name string) (int, error) {
	projects, err := c.ListProjects()
	if err != nil {
		return 0, err
	}
	for _, p := range projects {
		if strings.EqualFold(p.Name, name) || strings.EqualFold(p.Identifier, name) {
			return p.ID, nil
		}
	}
	return 0, fmt.Errorf("project: no match for %q", name)
}

// ListIssueCategories returns the issue categories defined on one project.
// Categories are project-specific, unlike trackers/statuses/priorities which
// are shared server-wide.
func (c *Client) ListIssueCategories(projectIDOrIdentifier string) ([]IDName, error) {
	var resp struct {
		IssueCategories []IDName `json:"issue_categories"`
	}
	if err := c.get(fmt.Sprintf("/projects/%s/issue_categories.json", projectIDOrIdentifier), nil, &resp); err != nil {
		return nil, err
	}
	return resp.IssueCategories, nil
}

// ResolveIssueCategoryID resolves a category name, matched case-insensitively,
// to its numeric ID within the given project.
func (c *Client) ResolveIssueCategoryID(projectIDOrIdentifier, name string) (int, error) {
	categories, err := c.ListIssueCategories(projectIDOrIdentifier)
	if err != nil {
		return 0, err
	}
	id, err := findIDByName(categories, name)
	if err != nil {
		return 0, fmt.Errorf("category: %w", err)
	}
	return id, nil
}
