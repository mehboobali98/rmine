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
	"time"
)

// requestTimeout bounds every JSON call to Redmine. Without it a server that
// accepts a connection and then stalls hangs the CLI — and any agent driving
// it — indefinitely, with no output and no way to tell progress from a hang.
const requestTimeout = 60 * time.Second

// downloadHeaderTimeout bounds how long an attachment download may wait for
// response headers. Downloads deliberately get no overall deadline instead: a
// large attachment on a slow link is slow but healthy, and capping total
// transfer time would abort it, while a stalled server never sends headers.
const downloadHeaderTimeout = 30 * time.Second

// Client talks to one Redmine server using API-key authentication.
type Client struct {
	baseURL  string
	apiKey   string
	http     *http.Client
	download *http.Client
}

// New builds a Client for the given Redmine base URL and API key.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: requestTimeout},
		download: &http.Client{
			Transport: &http.Transport{ResponseHeaderTimeout: downloadHeaderTimeout},
		},
	}
}

// BaseURL returns the server's root URL, trailing slash trimmed. Callers use
// it to build the web address of a resource they have fetched over the API.
func (c *Client) BaseURL() string {
	return c.baseURL
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

// Download streams an attachment's content to w. contentURL is the absolute
// URL Redmine puts in an attachment's content_url field. This can't reuse
// do(), which asks for JSON and unmarshals the whole body — attachments are
// arbitrary bytes and large enough that buffering them all is wasteful.
func (c *Client) Download(contentURL string, w io.Writer) error {
	req, err := http.NewRequest(http.MethodGet, contentURL, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-Redmine-API-Key", c.apiKey)

	resp, err := c.download.Do(req)
	if err != nil {
		return fmt.Errorf("calling redmine: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode}
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("downloading attachment: %w", err)
	}
	return nil
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
