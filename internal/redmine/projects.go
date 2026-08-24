package redmine

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Project is a Redmine project.
//
// The fields below the fold are only populated by GetProject, and only when
// asked for: Redmine returns them under `include`, and a listing never
// carries them. They are what an agent needs to construct a valid `issue
// create` — which trackers this project accepts, which categories and
// versions exist — and their absence used to make that undiscoverable
// without sampling existing issues.
type Project struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Identifier  string  `json:"identifier"`
	Description string  `json:"description"`
	Homepage    string  `json:"homepage,omitempty"`
	Status      int     `json:"status"`
	IsPublic    *bool   `json:"is_public,omitempty"`
	Parent      *IDName `json:"parent,omitempty"`

	Trackers        []IDName `json:"trackers,omitempty"`
	IssueCategories []IDName `json:"issue_categories,omitempty"`
	EnabledModules  []IDName `json:"enabled_modules,omitempty"`
}

// projectDetailIncludes is what GetProjectDetail asks Redmine to embed. An
// unknown include is ignored rather than rejected, so a server with the
// issue-tracking module disabled simply returns fewer keys.
var projectDetailIncludes = []string{"trackers", "issue_categories", "enabled_modules"}

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
	return c.getProject(idOrIdentifier, nil)
}

// GetProjectDetail fetches a project along with the trackers, categories and
// modules it has enabled.
func (c *Client) GetProjectDetail(idOrIdentifier string) (*Project, error) {
	return c.getProject(idOrIdentifier, projectDetailIncludes)
}

func (c *Client) getProject(idOrIdentifier string, includes []string) (*Project, error) {
	var query url.Values
	if len(includes) > 0 {
		query = url.Values{"include": {strings.Join(includes, ",")}}
	}
	var resp projectResponse
	if err := c.get("/projects/"+url.PathEscape(idOrIdentifier)+".json", query, &resp); err != nil {
		return nil, err
	}
	return &resp.Project, nil
}

// ResolveProjectID resolves a project's display name or identifier, matched
// case-insensitively, to its numeric ID. It returns an error wrapping
// ErrNoMatch when the lookup succeeded but nothing matched.
func (c *Client) ResolveProjectID(name string) (int, error) {
	// An identifier can be fetched directly, so try that before paging
	// through every project the user can see just to match one string — on a
	// large instance that listing costs more than the command it precedes.
	// A display name that happens to be identifier-shaped resolves by
	// identifier first; both are legitimate matches, only the order differs.
	if looksLikeIdentifier(name) {
		if p, err := c.GetProject(name); err == nil && p.ID != 0 {
			return p.ID, nil
		}
	}

	projects, err := c.ListProjects()
	if err != nil {
		return 0, err
	}
	candidates := make([]IDName, 0, len(projects))
	for _, p := range projects {
		if strings.EqualFold(p.Name, name) || strings.EqualFold(p.Identifier, name) {
			return p.ID, nil
		}
		candidates = append(candidates, IDName{ID: p.ID, Name: p.Name})
	}
	return 0, fmt.Errorf("project: %w", noMatch(name, candidates))
}

// looksLikeIdentifier reports whether s has the shape of a Redmine project
// identifier: lowercase letters, digits, dashes and underscores. Anything
// else — spaces, capitals — can only be a display name.
func looksLikeIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// ListIssueCategories returns the issue categories defined on one project.
// Categories are project-specific, unlike trackers/statuses/priorities which
// are shared server-wide.
func (c *Client) ListIssueCategories(projectIDOrIdentifier string) ([]IDName, error) {
	var resp struct {
		IssueCategories []IDName `json:"issue_categories"`
	}
	if err := c.get("/projects/"+url.PathEscape(projectIDOrIdentifier)+"/issue_categories.json", nil, &resp); err != nil {
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
