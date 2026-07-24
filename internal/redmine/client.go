// Package redmine is a minimal client for the parts of the Redmine REST API
// that rmine needs: issues, projects, time entries, and the enumerations used
// to resolve human-readable names to the IDs the API expects.
package redmine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client talks to one Redmine server using API-key authentication.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{},
	}
}

// APIError represents a non-2xx response from Redmine.
type APIError struct {
	StatusCode int
	Errors     []string
}

func (e *APIError) Error() string {
	if len(e.Errors) > 0 {
		return fmt.Sprintf("redmine returned %d: %s", e.StatusCode, strings.Join(e.Errors, "; "))
	}
	return fmt.Sprintf("redmine returned %d", e.StatusCode)
}

type errorBody struct {
	Errors []string `json:"errors"`
}

func (c *Client) do(method, path string, query url.Values, body, out any) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, u, reqBody)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-Redmine-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling redmine: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		var eb errorBody
		if json.Unmarshal(data, &eb) == nil {
			apiErr.Errors = eb.Errors
		}
		return apiErr
	}

	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}
	return nil
}

func (c *Client) get(path string, query url.Values, out any) error {
	return c.do(http.MethodGet, path, query, nil, out)
}

func (c *Client) post(path string, body, out any) error {
	return c.do(http.MethodPost, path, nil, body, out)
}

func (c *Client) put(path string, body any) error {
	return c.do(http.MethodPut, path, nil, body, nil)
}

func (c *Client) delete(path string) error {
	return c.do(http.MethodDelete, path, nil, nil, nil)
}

// dateRangeFilter builds Redmine's range-filter syntax for a date field,
// e.g. "><2026-01-01|2026-01-31", ">=2026-01-01", or "<=2026-01-31".
func dateRangeFilter(from, to string) string {
	switch {
	case from != "" && to != "":
		return fmt.Sprintf("><%s|%s", from, to)
	case from != "":
		return fmt.Sprintf(">=%s", from)
	default:
		return fmt.Sprintf("<=%s", to)
	}
}

// IDName is Redmine's common {id, name} shape used for projects, trackers,
// statuses, priorities, users, and activities embedded in other resources.
type IDName struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
