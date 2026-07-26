package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mehboobali98/rmine/internal/redmine"
)

var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Work with Redmine issues",
}

var issueListCmd = &cobra.Command{
	Use:   "list",
	Short: "List issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		status, _ := cmd.Flags().GetString("status")
		assignee, _ := cmd.Flags().GetString("assignee")
		tracker, _ := cmd.Flags().GetString("tracker")
		subject, _ := cmd.Flags().GetString("subject")
		updatedAfter, _ := cmd.Flags().GetString("updated-after")
		updatedBefore, _ := cmd.Flags().GetString("updated-before")
		dueAfter, _ := cmd.Flags().GetString("due-after")
		dueBefore, _ := cmd.Flags().GetString("due-before")
		dueWithin, _ := cmd.Flags().GetInt("due-within")
		dueNextWeek, _ := cmd.Flags().GetBool("due-next-week")
		limit, _ := cmd.Flags().GetInt("limit")
		all, _ := cmd.Flags().GetBool("all")

		client, err := newClient()
		if err != nil {
			return err
		}

		status, err = resolveStatusFilter(client, status)
		if err != nil {
			return err
		}
		project, err = resolveProjectFilter(client, project)
		if err != nil {
			return err
		}
		dueAfter, dueBefore, err = resolveDueDateRange(time.Now(), dueAfter, dueBefore, dueWithin, dueNextWeek)
		if err != nil {
			return err
		}

		filter := redmine.IssueListFilter{
			ProjectID:     project,
			StatusID:      status,
			AssignedTo:    assignee,
			TrackerID:     tracker,
			Subject:       subject,
			UpdatedAfter:  updatedAfter,
			UpdatedBefore: updatedBefore,
			DueAfter:      dueAfter,
			DueBefore:     dueBefore,
			Limit:         limit,
			All:           all,
		}
		issues, err := client.ListIssues(filter)
		if err != nil {
			return err
		}

		if wantsJSON() {
			return printJSON(issues)
		}

		rows := make([][]string, 0, len(issues))
		for _, iss := range issues {
			rows = append(rows, []string{
				strconv.Itoa(iss.ID),
				iss.Tracker.Name,
				iss.Status.Name,
				iss.Priority.Name,
				assigneeName(iss.AssignedTo),
				iss.Subject,
			})
		}
		printTable([]string{"ID", "TRACKER", "STATUS", "PRIORITY", "ASSIGNEE", "SUBJECT"}, rows)
		return nil
	},
}

var issueViewCmd = &cobra.Command{
	Use:   "view <id>",
	Short: "Show an issue's details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseIssueID(args[0])
		if err != nil {
			return err
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		issue, err := client.GetIssue(id)
		if err != nil {
			return err
		}

		if wantsJSON() {
			return printJSON(issue)
		}

		fmt.Printf("#%d %s\n", issue.ID, issue.Subject)
		fmt.Printf("Project:  %s\n", issue.Project.Name)
		fmt.Printf("Tracker:  %s\n", issue.Tracker.Name)
		fmt.Printf("Status:   %s\n", issue.Status.Name)
		fmt.Printf("Priority: %s\n", issue.Priority.Name)
		fmt.Printf("Assignee: %s\n", assigneeName(issue.AssignedTo))
		if issue.Category != nil {
			fmt.Printf("Category: %s\n", issue.Category.Name)
		}
		for _, f := range issue.CustomFields {
			fmt.Printf("%s (id %d): %s\n", f.Name, f.ID, f.Value)
		}
		if issue.Description != "" {
			fmt.Printf("\n%s\n", issue.Description)
		}
		return nil
	},
}

var issueCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new issue",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		subject, _ := cmd.Flags().GetString("subject")
		description, _ := cmd.Flags().GetString("description")
		trackerName, _ := cmd.Flags().GetString("tracker")
		priorityName, _ := cmd.Flags().GetString("priority")
		categoryName, _ := cmd.Flags().GetString("category")
		assignee, _ := cmd.Flags().GetInt("assignee")
		fieldArgs, _ := cmd.Flags().GetStringArray("field")

		customFields, err := parseCustomFields(fieldArgs)
		if err != nil {
			return err
		}

		client, err := newClient()
		if err != nil {
			return err
		}

		project, err = resolveProjectFilter(client, project)
		if err != nil {
			return err
		}

		req := redmine.CreateIssueRequest{
			ProjectID:    project,
			Subject:      subject,
			Description:  description,
			AssignedTo:   assignee,
			CustomFields: customFields,
		}
		if trackerName != "" {
			req.TrackerID, err = client.ResolveTrackerID(trackerName)
			if err != nil {
				return err
			}
		}
		if priorityName != "" {
			req.PriorityID, err = client.ResolveIssuePriorityID(priorityName)
			if err != nil {
				return err
			}
		}
		if categoryName != "" {
			req.CategoryID, err = client.ResolveIssueCategoryID(project, categoryName)
			if err != nil {
				return err
			}
		}

		issue, err := client.CreateIssue(req)
		if err != nil {
			return err
		}

		if wantsJSON() {
			return printJSON(issue)
		}
		fmt.Printf("Created issue #%d: %s\n", issue.ID, issue.Subject)
		return nil
	},
}

var issueUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseIssueID(args[0])
		if err != nil {
			return err
		}

		subject, _ := cmd.Flags().GetString("subject")
		description, _ := cmd.Flags().GetString("description")
		trackerName, _ := cmd.Flags().GetString("tracker")
		priorityName, _ := cmd.Flags().GetString("priority")
		statusName, _ := cmd.Flags().GetString("status")
		categoryName, _ := cmd.Flags().GetString("category")
		assignee, _ := cmd.Flags().GetInt("assignee")
		fieldArgs, _ := cmd.Flags().GetStringArray("field")

		customFields, err := parseCustomFields(fieldArgs)
		if err != nil {
			return err
		}

		client, err := newClient()
		if err != nil {
			return err
		}

		req := redmine.UpdateIssueRequest{
			Subject:      subject,
			Description:  description,
			AssignedTo:   assignee,
			CustomFields: customFields,
		}
		if trackerName != "" {
			req.TrackerID, err = client.ResolveTrackerID(trackerName)
			if err != nil {
				return err
			}
		}
		if priorityName != "" {
			req.PriorityID, err = client.ResolveIssuePriorityID(priorityName)
			if err != nil {
				return err
			}
		}
		if statusName != "" {
			req.StatusID, err = client.ResolveIssueStatusID(statusName)
			if err != nil {
				return err
			}
		}
		if categoryName != "" {
			issue, err := client.GetIssue(id)
			if err != nil {
				return err
			}
			req.CategoryID, err = client.ResolveIssueCategoryID(strconv.Itoa(issue.Project.ID), categoryName)
			if err != nil {
				return err
			}
		}

		if err := client.UpdateIssue(id, req); err != nil {
			return err
		}
		fmt.Printf("Updated issue #%d\n", id)
		return nil
	},
}

var issueCloseCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Close an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseIssueID(args[0])
		if err != nil {
			return err
		}
		statusName, _ := cmd.Flags().GetString("status")

		client, err := newClient()
		if err != nil {
			return err
		}

		var statusID int
		if statusName != "" {
			statusID, err = client.ResolveIssueStatusID(statusName)
		} else {
			statusID, err = client.DefaultClosedStatusID()
		}
		if err != nil {
			return err
		}

		if err := client.UpdateIssue(id, redmine.UpdateIssueRequest{StatusID: statusID}); err != nil {
			return err
		}
		fmt.Printf("Closed issue #%d\n", id)
		return nil
	},
}

var issueCommentCmd = &cobra.Command{
	Use:   "comment <id> <note>",
	Short: "Add a comment to an issue",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseIssueID(args[0])
		if err != nil {
			return err
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		if err := client.AddNote(id, args[1]); err != nil {
			return err
		}
		fmt.Printf("Commented on issue #%d\n", id)
		return nil
	},
}

func init() {
	issueListCmd.Flags().String("project", "", "filter by project ID, identifier, or name (e.g. \"AssetSonar Scrum Team\")")
	issueListCmd.Flags().String("status", "", "filter by status name (e.g. In Progress), status ID, or open/closed/*")
	issueListCmd.Flags().String("assignee", "", "filter by assignee user ID (or \"me\" for the authenticated user)")
	issueListCmd.Flags().String("tracker", "", "filter by tracker ID")
	issueListCmd.Flags().String("subject", "", "only issues whose subject contains this text")
	issueListCmd.Flags().String("updated-after", "", "only issues updated on or after this date (YYYY-MM-DD)")
	issueListCmd.Flags().String("updated-before", "", "only issues updated on or before this date (YYYY-MM-DD)")
	issueListCmd.Flags().String("due-after", "", "only issues due on or after this date (YYYY-MM-DD)")
	issueListCmd.Flags().String("due-before", "", "only issues due on or before this date (YYYY-MM-DD)")
	issueListCmd.Flags().Int("due-within", 0, "only issues due within this many days from today")
	issueListCmd.Flags().Bool("due-next-week", false, "only issues due next week (Mon-Sun)")
	issueListCmd.Flags().Int("limit", 25, "maximum number of issues to return")
	issueListCmd.Flags().Bool("all", false, "fetch every matching issue, ignoring --limit")

	issueCreateCmd.Flags().String("project", "", "project ID, identifier, or name (required)")
	issueCreateCmd.Flags().String("subject", "", "issue subject (required)")
	issueCreateCmd.Flags().String("description", "", "issue description")
	issueCreateCmd.Flags().String("tracker", "", "tracker name, e.g. Bug")
	issueCreateCmd.Flags().String("priority", "", "priority name, e.g. High")
	issueCreateCmd.Flags().String("category", "", "issue category name (project-specific; see `rmine project categories <project>`)")
	issueCreateCmd.Flags().Int("assignee", 0, "assignee user ID")
	issueCreateCmd.Flags().StringArray("field", nil, "custom field as id=value (repeatable); find IDs via `rmine issue view <id> -o json` on an existing issue")
	_ = issueCreateCmd.MarkFlagRequired("project")
	_ = issueCreateCmd.MarkFlagRequired("subject")

	issueUpdateCmd.Flags().String("subject", "", "new subject")
	issueUpdateCmd.Flags().String("description", "", "new description")
	issueUpdateCmd.Flags().String("tracker", "", "new tracker name")
	issueUpdateCmd.Flags().String("priority", "", "new priority name")
	issueUpdateCmd.Flags().String("status", "", "new status name")
	issueUpdateCmd.Flags().String("category", "", "new category name (project-specific; see `rmine project categories <project>`)")
	issueUpdateCmd.Flags().Int("assignee", 0, "new assignee user ID")
	issueUpdateCmd.Flags().StringArray("field", nil, "custom field as id=value (repeatable)")

	issueCloseCmd.Flags().String("status", "", "status name to close with (defaults to the server's first closed status)")

	issueCmd.AddCommand(issueListCmd, issueViewCmd, issueCreateCmd, issueUpdateCmd, issueCloseCmd, issueCommentCmd)
	rootCmd.AddCommand(issueCmd)
}

func assigneeName(a *redmine.IDName) string {
	if a == nil {
		return "-"
	}
	return a.Name
}

func parseIssueID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid issue ID %q", s)
	}
	return id, nil
}

// resolveStatusFilter passes numeric IDs and Redmine's open/closed/* keywords
// through unchanged, and resolves anything else (e.g. "New", "In Progress")
// to its status ID via the server's issue statuses.
func resolveStatusFilter(client *redmine.Client, status string) (string, error) {
	switch status {
	case "", "open", "closed", "*":
		return status, nil
	}
	if _, err := strconv.Atoi(status); err == nil {
		return status, nil
	}
	id, err := client.ResolveIssueStatusID(status)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(id), nil
}

// resolveProjectFilter passes numeric IDs through unchanged and resolves a
// display name (e.g. "AssetSonar Scrum Team") to its numeric ID via the
// server's projects. Anything else (e.g. an identifier slug) is passed
// through as-is for Redmine to validate.
func resolveProjectFilter(client *redmine.Client, project string) (string, error) {
	if project == "" {
		return "", nil
	}
	if _, err := strconv.Atoi(project); err == nil {
		return project, nil
	}
	if id, err := client.ResolveProjectID(project); err == nil {
		return strconv.Itoa(id), nil
	}
	return project, nil
}

// parseCustomFields turns repeated "--field id=value" flags into custom
// field updates. IDs (not names) are required: custom field definitions are
// per-instance and Redmine's enumeration endpoint for them is admin-only, so
// there's no reliable way to resolve a name to an ID for every server.
//
// Passing the same ID more than once collects the values into one
// multi-value field (Redmine requires an array to set 2+ options on a
// checkbox/multi-select field), e.g. --field 11=16 --field 11=27.
func parseCustomFields(raw []string) ([]redmine.CustomField, error) {
	order := make([]int, 0, len(raw))
	values := make(map[int][]string, len(raw))
	for _, kv := range raw {
		idStr, value, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --field %q, want id=value", kv)
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return nil, fmt.Errorf("invalid --field %q: id must be numeric", kv)
		}
		if _, seen := values[id]; !seen {
			order = append(order, id)
		}
		values[id] = append(values[id], value)
	}

	fields := make([]redmine.CustomField, 0, len(order))
	for _, id := range order {
		vs := values[id]
		if len(vs) == 1 {
			fields = append(fields, redmine.CustomField{ID: id, Value: redmine.FieldValue(vs[0])})
		} else {
			fields = append(fields, redmine.CustomField{ID: id, Values: vs})
		}
	}
	return fields, nil
}

// resolveDueDateRange combines the raw --due-after/--due-before dates with
// the --due-within/--due-next-week shortcuts into a single (after, before)
// range, relative to now. The shortcuts are mutually exclusive with each
// other and with the raw flags.
func resolveDueDateRange(now time.Time, after, before string, within int, nextWeek bool) (string, string, error) {
	shortcuts := 0
	if within > 0 {
		shortcuts++
	}
	if nextWeek {
		shortcuts++
	}
	if shortcuts > 1 {
		return "", "", fmt.Errorf("only one of --due-within or --due-next-week may be set")
	}
	if shortcuts > 0 && (after != "" || before != "") {
		return "", "", fmt.Errorf("--due-within/--due-next-week cannot be combined with --due-after/--due-before")
	}

	const layout = "2006-01-02"
	switch {
	case within > 0:
		return now.Format(layout), now.AddDate(0, 0, within).Format(layout), nil
	case nextWeek:
		monday := now.AddDate(0, 0, -((int(now.Weekday()) + 6) % 7))
		nextMonday := monday.AddDate(0, 0, 7)
		nextSunday := nextMonday.AddDate(0, 0, 6)
		return nextMonday.Format(layout), nextSunday.Format(layout), nil
	default:
		return after, before, nil
	}
}
