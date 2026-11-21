package cli

import (
	"strconv"

	"github.com/mehboobali98/rmine/internal/redmine"
)

// detailNames maps the IDs in journal field changes to names, keyed by the ID
// as Redmine sends it (a string). A nil map, because its lookup was not
// needed or failed, makes describeDetail print the raw ID.
type detailNames struct {
	statuses     map[string]string
	trackers     map[string]string
	priorities   map[string]string
	customFields map[string]string
}

// attributeLabels are the readable names of the built-in fields whose journal
// values are IDs. Any other attribute is shown by its Redmine key.
var attributeLabels = map[string]string{
	"status_id":        "Status",
	"assigned_to_id":   "Assignee",
	"tracker_id":       "Tracker",
	"priority_id":      "Priority",
	"fixed_version_id": "Version",
	"parent_id":        "Parent",
}

// loadDetailNames fetches the names the issue's journal changes refer to. It
// asks Redmine only for the lists some change needs; custom field names come
// from the issue itself. A failed lookup is not an error: the view still
// prints with raw IDs, and one warning per failed lookup goes to stderr.
func loadDetailNames(c *redmine.Client, issue *redmine.Issue) detailNames {
	names := detailNames{customFields: map[string]string{}}
	for _, f := range issue.CustomFields {
		names.customFields[strconv.Itoa(f.ID)] = f.Name
	}

	needed := map[string]bool{}
	for _, j := range issue.Journals {
		for _, d := range j.Details {
			if d.Property == "attr" {
				needed[d.Name] = true
			}
		}
	}

	if needed["status_id"] {
		statuses, err := c.ListIssueStatuses()
		if err != nil {
			warnLookupFailed("issue status", err)
		} else {
			names.statuses = map[string]string{}
			for _, s := range statuses {
				names.statuses[strconv.Itoa(s.ID)] = s.Name
			}
		}
	}
	if needed["tracker_id"] {
		names.trackers = lookupIDNames("tracker", c.ListTrackers)
	}
	if needed["priority_id"] {
		names.priorities = lookupIDNames("priority", c.ListIssuePriorities)
	}
	return names
}

func lookupIDNames(what string, list func() ([]redmine.IDName, error)) map[string]string {
	items, err := list()
	if err != nil {
		warnLookupFailed(what, err)
		return nil
	}
	byID := make(map[string]string, len(items))
	for _, item := range items {
		byID[strconv.Itoa(item.ID)] = item.Name
	}
	return byID
}

func warnLookupFailed(what string, err error) {
	promptf("Warning: could not look up %s names (%v); showing raw IDs.\n", what, err)
}

// describeDetail renders one field change as "<Label>: <old> -> <new>". A
// description change prints only that it changed, since either version can
// run to many lines.
func describeDetail(d redmine.JournalDetail, n detailNames) string {
	var label string
	var byID map[string]string
	switch d.Property {
	case "attr":
		if d.Name == "description" {
			return "Description: changed"
		}
		label = attributeLabels[d.Name]
		if label == "" {
			label = d.Name
		}
		switch d.Name {
		case "status_id":
			byID = n.statuses
		case "tracker_id":
			byID = n.trackers
		case "priority_id":
			byID = n.priorities
		}
	case "cf":
		label = n.customFields[d.Name]
		if label == "" {
			label = "Custom field " + d.Name
		}
	default:
		label = d.Property + " " + d.Name
	}
	return label + ": " + detailValue(d.OldValue, byID) + " -> " + detailValue(d.NewValue, byID)
}

// detailValue shows a change's value, named when byID knows it. Redmine
// reports a field with no value as null or as "", depending on the field.
func detailValue(v *string, byID map[string]string) string {
	if v == nil || *v == "" {
		return "(none)"
	}
	if name, ok := byID[*v]; ok {
		return name
	}
	return *v
}
