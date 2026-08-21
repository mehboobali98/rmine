package redmine

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Membership is one principal's membership of a project. Redmine puts a
// person in User and a group in Group; exactly one of the two is set.
type Membership struct {
	ID    int     `json:"id"`
	User  *IDName `json:"user"`
	Group *IDName `json:"group"`
}

type membershipListResponse struct {
	Memberships []Membership `json:"memberships"`
	TotalCount  int          `json:"total_count"`
}

// ListProjectMemberships returns the principals with a role on one project.
//
// This is the only user listing an ordinary account can generally read:
// /users.json is admin-only, which is why rmine resolves user names within a
// project rather than server-wide.
func (c *Client) ListProjectMemberships(projectIDOrIdentifier string) ([]Membership, error) {
	const pageSize = 100
	path := "/projects/" + url.PathEscape(projectIDOrIdentifier) + "/memberships.json"

	var all []Membership
	offset := 0
	for {
		q := url.Values{}
		q.Set("limit", strconv.Itoa(pageSize))
		q.Set("offset", strconv.Itoa(offset))

		var resp membershipListResponse
		if err := c.get(path, q, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Memberships...)

		offset += len(resp.Memberships)
		if len(resp.Memberships) == 0 || offset >= resp.TotalCount {
			break
		}
	}
	return all, nil
}

// ResolveUserID resolves a person's display name to their numeric ID within
// one project, matched case-insensitively. An exact match wins; failing that,
// a single substring match is accepted so "jane" finds "Jane Doe".
//
// Several matches are an error rather than a guess: picking one would assign
// work to the wrong person, and the caller cannot tell that happened.
func (c *Client) ResolveUserID(projectIDOrIdentifier, name string) (int, error) {
	memberships, err := c.ListProjectMemberships(projectIDOrIdentifier)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusForbidden || apiErr.StatusCode == http.StatusNotFound) {
			return 0, fmt.Errorf("cannot read the member list of project %q (%s) — pass a numeric user ID instead", projectIDOrIdentifier, err)
		}
		return 0, err
	}

	var partial []IDName
	for _, m := range memberships {
		if m.User == nil {
			continue // a group, which cannot be an assignee
		}
		if strings.EqualFold(m.User.Name, name) {
			return m.User.ID, nil
		}
		if strings.Contains(strings.ToLower(m.User.Name), strings.ToLower(name)) {
			partial = append(partial, *m.User)
		}
	}

	switch len(partial) {
	case 1:
		return partial[0].ID, nil
	case 0:
		return 0, fmt.Errorf("user: %w for %q in project %q", ErrNoMatch, name, projectIDOrIdentifier)
	default:
		names := make([]string, 0, len(partial))
		for _, u := range partial {
			names = append(names, fmt.Sprintf("%s (id %d)", u.Name, u.ID))
		}
		return 0, fmt.Errorf("user: %q matches %d members of project %q — %s; use a numeric ID to pick one",
			name, len(partial), projectIDOrIdentifier, strings.Join(names, ", "))
	}
}
