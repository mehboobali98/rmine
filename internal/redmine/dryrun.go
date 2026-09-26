package redmine

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PlannedRequest is a write a dry-run client recorded instead of sending.
// File is set for an upload, whose body is the file's bytes rather than JSON.
type PlannedRequest struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body,omitempty"`
	File   string          `json:"file,omitempty"`
}

// DryRunError stops a command at its first write when the client is in
// dry-run mode, carrying every request that would have been sent.
type DryRunError struct {
	Planned []PlannedRequest
}

func (e *DryRunError) Error() string {
	steps := make([]string, 0, len(e.Planned))
	for _, p := range e.Planned {
		steps = append(steps, p.Method+" "+p.Path)
	}
	return fmt.Sprintf("dry run, nothing sent: %s", strings.Join(steps, ", "))
}

// DryRunUploadToken stands in for the token a real upload would return.
const DryRunUploadToken = "(dry run: token from the upload above)"

// SetDryRun makes every write record itself instead of reaching the server.
// Reads still go out, so names resolve exactly as they would for real.
func (c *Client) SetDryRun(on bool) {
	c.dryRun = on
}

func (c *Client) planWrite(method, path string, body any) error {
	req := PlannedRequest{Method: method, Path: path}
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		req.Body = data
	}
	c.planned = append(c.planned, req)
	return &DryRunError{Planned: c.planned}
}
