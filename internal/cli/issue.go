package cli

import (
	"fmt"
	"strconv"

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

		filter := redmine.IssueListFilter{
			ProjectID:     project,
			StatusID:      status,
			AssignedTo:    assignee,
			TrackerID:     tracker,
			Subject:       subject,
			UpdatedAfter:  updatedAfter,
			UpdatedBefore: updatedBefore,
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
		assignee, _ := cmd.Flags().GetInt("assignee")

		client, err := newClient()
		if err != nil {
			return err
		}

		req := redmine.CreateIssueRequest{
			ProjectID:   project,
			Subject:     subject,
			Description: description,
			AssignedTo:  assignee,
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
		assignee, _ := cmd.Flags().GetInt("assignee")

		client, err := newClient()
		if err != nil {
			return err
		}

		req := redmine.UpdateIssueRequest{
			Subject:     subject,
			Description: description,
			AssignedTo:  assignee,
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
	issueListCmd.Flags().String("project", "", "filter by project ID or identifier")
	issueListCmd.Flags().String("status", "", "filter by status name (e.g. New), status ID, or open/closed/*")
	issueListCmd.Flags().String("assignee", "", "filter by assignee user ID")
	issueListCmd.Flags().String("tracker", "", "filter by tracker ID")
	issueListCmd.Flags().String("subject", "", "only issues whose subject contains this text")
	issueListCmd.Flags().String("updated-after", "", "only issues updated on or after this date (YYYY-MM-DD)")
	issueListCmd.Flags().String("updated-before", "", "only issues updated on or before this date (YYYY-MM-DD)")
	issueListCmd.Flags().Int("limit", 25, "maximum number of issues to return")
	issueListCmd.Flags().Bool("all", false, "fetch every matching issue, ignoring --limit")

	issueCreateCmd.Flags().String("project", "", "project ID or identifier (required)")
	issueCreateCmd.Flags().String("subject", "", "issue subject (required)")
	issueCreateCmd.Flags().String("description", "", "issue description")
	issueCreateCmd.Flags().String("tracker", "", "tracker name, e.g. Bug")
	issueCreateCmd.Flags().String("priority", "", "priority name, e.g. High")
	issueCreateCmd.Flags().Int("assignee", 0, "assignee user ID")
	_ = issueCreateCmd.MarkFlagRequired("project")
	_ = issueCreateCmd.MarkFlagRequired("subject")

	issueUpdateCmd.Flags().String("subject", "", "new subject")
	issueUpdateCmd.Flags().String("description", "", "new description")
	issueUpdateCmd.Flags().String("tracker", "", "new tracker name")
	issueUpdateCmd.Flags().String("priority", "", "new priority name")
	issueUpdateCmd.Flags().String("status", "", "new status name")
	issueUpdateCmd.Flags().Int("assignee", 0, "new assignee user ID")

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
