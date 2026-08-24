package redmine

import (
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Upload is a file staged on the server, ready to be attached to an issue.
//
// Redmine attaches files in two steps: the bytes go to /uploads.json on their
// own and come back as a token, which a later issue create/update references.
// Nothing is visible on the issue until that second call lands, so a token
// that is never referenced simply expires.
type Upload struct {
	Token       string `json:"token"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Description string `json:"description,omitempty"`
}

type uploadResponse struct {
	Upload struct {
		ID    int    `json:"id"`
		Token string `json:"token"`
	} `json:"upload"`
}

// UploadFile stages a local file and returns the Upload that references it.
//
// The body is streamed from disk rather than buffered: attachments are the
// one thing rmine sends that is routinely larger than memory is cheap for.
func (c *Client) UploadFile(path string) (*Upload, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory, not a file", path)
	}

	name := filepath.Base(path)
	query := url.Values{"filename": {name}}

	var resp uploadResponse
	if err := c.doRaw("POST", "/uploads.json", query, "application/octet-stream", f, &resp); err != nil {
		return nil, fmt.Errorf("uploading %s: %w", name, err)
	}
	if resp.Upload.Token == "" {
		return nil, fmt.Errorf("uploading %s: redmine returned no upload token", name)
	}

	return &Upload{
		Token:       resp.Upload.Token,
		Filename:    name,
		ContentType: contentTypeFor(name),
	}, nil
}

// contentTypeFor guesses an attachment's MIME type from its extension.
// Redmine stores whatever it is told and serves it back on download, so a
// wrong guess makes a browser mishandle the file; an unknown extension is
// better left as the generic type than guessed at.
func contentTypeFor(name string) string {
	ct := mime.TypeByExtension(filepath.Ext(name))
	if ct == "" {
		return "application/octet-stream"
	}
	// TypeByExtension appends charset for text types; Redmine's content_type
	// column is short, and the bare type is what its own UI stores.
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return ct
}
