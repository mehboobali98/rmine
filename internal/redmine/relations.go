package redmine

import (
	"fmt"
	"sort"
	"strings"
)

// Relation is a typed link between two issues.
//
// The type is read from IssueID's side: a relation with RelationType
// "precedes" means IssueID precedes IssueToID. Redmine stores each link once
// and shows it from both ends, inverting the name for the other issue, so the
// same row reads as "follows" when fetched via IssueToID.
type Relation struct {
	ID           int    `json:"id"`
	IssueID      int    `json:"issue_id"`
	IssueToID    int    `json:"issue_to_id"`
	RelationType string `json:"relation_type"`
	Delay        *int   `json:"delay,omitempty"`
}

// relationInverses maps every relation type Redmine accepts to how the same
// link reads from the other issue. The keys double as the set of valid types.
var relationInverses = map[string]string{
	"relates":     "relates",
	"duplicates":  "duplicated",
	"duplicated":  "duplicates",
	"blocks":      "blocked",
	"blocked":     "blocks",
	"precedes":    "follows",
	"follows":     "precedes",
	"copied_to":   "copied_from",
	"copied_from": "copied_to",
}

// RelationTypes lists the relation types Redmine accepts, sorted so that a
// help string or an error message is stable between runs.
func RelationTypes() []string {
	types := make([]string, 0, len(relationInverses))
	for t := range relationInverses {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

// ValidateRelationType reports whether Redmine knows this relation type.
// Checking here rather than at the server is worth the duplication: Redmine
// answers an unknown type with a bare 422 and no field name, which reads like
// a problem with the issues rather than with the word that was typed.
func ValidateRelationType(t string) error {
	if _, ok := relationInverses[t]; !ok {
		return fmt.Errorf("unknown relation type %q (want one of: %s)", t, strings.Join(RelationTypes(), ", "))
	}
	return nil
}

// InvertRelationType returns how a relation reads from the other issue's
// side, or the type unchanged if it isn't one Redmine defines.
func InvertRelationType(t string) string {
	if inverse, ok := relationInverses[t]; ok {
		return inverse
	}
	return t
}

// SupportsDelay reports whether a relation type carries a delay in days.
// Only the scheduling relations do; Redmine ignores the field on the rest.
func SupportsDelay(t string) bool {
	return t == "precedes" || t == "follows"
}

// ListIssueRelations returns every relation involving one issue, in both
// directions.
func (c *Client) ListIssueRelations(issueID int) ([]Relation, error) {
	var resp struct {
		Relations []Relation `json:"relations"`
	}
	if err := c.get(fmt.Sprintf("/issues/%d/relations.json", issueID), nil, &resp); err != nil {
		return nil, err
	}
	return resp.Relations, nil
}

// NewRelation describes a link to create from one issue to another.
type NewRelation struct {
	IssueToID    int
	RelationType string
	Delay        *int // only meaningful for precedes/follows
}

// CreateIssueRelation links issueID to another issue and returns the stored
// relation.
func (c *Client) CreateIssueRelation(issueID int, rel NewRelation) (*Relation, error) {
	if err := ValidateRelationType(rel.RelationType); err != nil {
		return nil, err
	}

	body := map[string]any{"relation": map[string]any{
		"issue_to_id":   rel.IssueToID,
		"relation_type": rel.RelationType,
	}}
	if rel.Delay != nil && SupportsDelay(rel.RelationType) {
		body["relation"].(map[string]any)["delay"] = *rel.Delay
	}

	var resp struct {
		Relation Relation `json:"relation"`
	}
	if err := c.post(fmt.Sprintf("/issues/%d/relations.json", issueID), body, &resp); err != nil {
		return nil, err
	}
	return &resp.Relation, nil
}

// DeleteIssueRelation removes one relation by its own ID — not by either
// issue's. Relation IDs come from ListIssueRelations.
func (c *Client) DeleteIssueRelation(relationID int) error {
	return c.delete(fmt.Sprintf("/relations/%d.json", relationID))
}
