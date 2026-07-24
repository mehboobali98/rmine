package redmine

import (
	"fmt"
	"net/url"
	"strconv"
)

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

func (c *Client) GetProject(idOrIdentifier string) (*Project, error) {
	var resp projectResponse
	if err := c.get(fmt.Sprintf("/projects/%s.json", idOrIdentifier), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Project, nil
}
