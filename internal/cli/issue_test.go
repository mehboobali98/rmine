package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mehboobali98/rmine/internal/redmine"
)

func TestResolveStatusFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"issue_statuses": []map[string]any{
				{"id": 1, "name": "New", "is_closed": false},
				{"id": 5, "name": "Closed", "is_closed": true},
			},
		})
	}))
	defer srv.Close()
	client := redmine.New(srv.URL, "test-key")

	cases := []struct{ in, want string }{
		{"", ""},
		{"open", "open"},
		{"closed", "closed"},
		{"*", "*"},
		{"7", "7"},
		{"New", "1"},
	}
	for _, c := range cases {
		got, err := resolveStatusFilter(client, c.in)
		if err != nil {
			t.Errorf("resolveStatusFilter(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolveStatusFilter(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	if _, err := resolveStatusFilter(client, "NoSuchStatus"); err == nil {
		t.Error("expected an error for an unknown status name")
	}
}
