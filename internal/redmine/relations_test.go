package redmine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateRelationType(t *testing.T) {
	for _, valid := range RelationTypes() {
		if err := ValidateRelationType(valid); err != nil {
			t.Errorf("ValidateRelationType(%q) = %v, want nil", valid, err)
		}
	}
	// A caller reaching for the English word rather than Redmine's spelling
	// is the likely mistake, and the message has to name the alternatives.
	err := ValidateRelationType("blocking")
	if err == nil {
		t.Fatal("ValidateRelationType(\"blocking\") = nil, want an error")
	}
	if got := err.Error(); !strings.Contains(got, "blocks") || !strings.Contains(got, "precedes") {
		t.Errorf("error does not list the valid types: %q", got)
	}
}

func TestInvertRelationTypeRoundTrips(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"precedes", "follows"},
		{"follows", "precedes"},
		{"blocks", "blocked"},
		{"blocked", "blocks"},
		{"relates", "relates"},
		{"copied_to", "copied_from"},
	} {
		if got := InvertRelationType(tc.in); got != tc.want {
			t.Errorf("InvertRelationType(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if got := InvertRelationType(InvertRelationType(tc.in)); got != tc.in {
			t.Errorf("InvertRelationType is not an involution for %q: got %q", tc.in, got)
		}
	}
}

func TestCreateIssueRelationSendsDelayOnlyWhereItApplies(t *testing.T) {
	cases := []struct {
		name         string
		relationType string
		delay        *int
		wantDelay    bool
	}{
		{"precedes carries a delay", "precedes", intPtr(3), true},
		{"relates drops it", "relates", intPtr(3), false},
		{"no delay given", "precedes", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/issues/100/relations.json" {
					t.Errorf("path = %s", r.URL.Path)
				}
				json.NewDecoder(r.Body).Decode(&body)
				json.NewEncoder(w).Encode(map[string]any{
					"relation": map[string]any{"id": 9, "issue_id": 100, "issue_to_id": 200, "relation_type": tc.relationType},
				})
			}))
			defer srv.Close()

			rel, err := New(srv.URL, "k").CreateIssueRelation(100, NewRelation{
				IssueToID: 200, RelationType: tc.relationType, Delay: tc.delay,
			})
			if err != nil {
				t.Fatalf("CreateIssueRelation: %v", err)
			}
			if rel.ID != 9 {
				t.Errorf("relation id = %d, want 9", rel.ID)
			}
			_, gotDelay := body["relation"]["delay"]
			if gotDelay != tc.wantDelay {
				t.Errorf("delay present = %t, want %t (body: %+v)", gotDelay, tc.wantDelay, body["relation"])
			}
		})
	}
}

func TestCreateIssueRelationRejectsUnknownTypeBeforeCalling(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("client sent a request for a type it could have rejected itself")
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "k").CreateIssueRelation(1, NewRelation{IssueToID: 2, RelationType: "nope"}); err == nil {
		t.Fatal("want an error for an unknown relation type")
	}
}

func intPtr(v int) *int { return &v }
