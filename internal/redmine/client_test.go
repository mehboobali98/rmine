package redmine

import (
	"net/http"
	"testing"
)

// A stalled server is the failure this guards against, but reproducing one
// takes as long as the timeout itself — so assert the client is wired with a
// deadline instead of waiting out a real hang.
func TestNewSetsTimeouts(t *testing.T) {
	c := New("https://redmine.example.com", "key")

	if c.http.Timeout != requestTimeout {
		t.Errorf("JSON client timeout = %v, want %v", c.http.Timeout, requestTimeout)
	}

	if c.download.Timeout != 0 {
		t.Errorf("download client timeout = %v, want no overall deadline", c.download.Timeout)
	}
	tr, ok := c.download.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("download transport = %T, want *http.Transport", c.download.Transport)
	}
	if tr.ResponseHeaderTimeout != downloadHeaderTimeout {
		t.Errorf("download header timeout = %v, want %v", tr.ResponseHeaderTimeout, downloadHeaderTimeout)
	}
}
