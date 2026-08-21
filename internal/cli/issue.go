package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		overdue, _ := cmd.Flags().GetBool("overdue")
		sort, _ := cmd.Flags().GetString("sort")
		limit, _ := cmd.Flags().GetInt("limit")
		all, _ := cmd.Flags().GetBool("all")

		sort, err := normalizeSort(sort)
		if err != nil {
			return err
		}
		if err := validateDates(
			dateFlag{"--updated-after", updatedAfter},
			dateFlag{"--updated-before", updatedBefore},
			dateFlag{"--due-after", dueAfter},
			dateFlag{"--due-before", dueBefore},
		); err != nil {
			return err
		}

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
		tracker, err = resolveTrackerFilter(client, tracker)
		if err != nil {
			return err
		}
		assignee, err = resolveUserFilter("--assignee", assignee)
		if err != nil {
			return err
		}
		dueAfter, dueBefore, err = resolveDueDateRange(time.Now(), dueAfter, dueBefore, dueWithin, dueNextWeek, overdue)
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
			Sort:          sort,
			Limit:         limit,
			All:           all,
		}
		issues, err := client.ListIssues(filter)
		if err != nil {
			return err
		}

		if wantsJSON() {
			// The table has no room for a URL column, but a caller parsing
			// JSON is usually the one that needs to hand a human a link.
			decorated := make([]issueWithURL, 0, len(issues))
			for i := range issues {
				decorated = append(decorated, withIssueURL(client, &issues[i]))
			}
			return printJSON(decorated)
		}

		rows := make([][]string, 0, len(issues))
		for _, iss := range issues {
			rows = append(rows, []string{
				strconv.Itoa(iss.ID),
				iss.Project.Name,
				iss.Tracker.Name,
				iss.Status.Name,
				iss.Priority.Name,
				assigneeName(iss.AssignedTo),
				orDash(iss.DueDate),
				iss.Subject,
			})
		}
		// PROJECT because an unfiltered list spans projects and the rows are
		// otherwise indistinguishable; DUE because four of this command's
		// filters narrow by it and none of them used to show it.
		printTable([]string{"ID", "PROJECT", "TRACKER", "STATUS", "PRIORITY", "ASSIGNEE", "DUE", "SUBJECT"}, rows)
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
		comments, _ := cmd.Flags().GetBool("comments")
		client, err := newClient()
		if err != nil {
			return err
		}
		issue, err := client.GetIssue(id, comments)
		if err != nil {
			return err
		}

		if wantsJSON() {
			return printJSON(withIssueURL(client, issue))
		}

		fmt.Printf("#%d %s\n", issue.ID, issue.Subject)
		fmt.Printf("URL:      %s\n", issueURL(client, issue.ID))
		fmt.Printf("Project:  %s\n", issue.Project.Name)
		fmt.Printf("Tracker:  %s\n", issue.Tracker.Name)
		fmt.Printf("Status:   %s\n", issue.Status.Name)
		fmt.Printf("Priority: %s\n", issue.Priority.Name)
		fmt.Printf("Assignee: %s\n", assigneeName(issue.AssignedTo))
		if issue.Category != nil {
			fmt.Printf("Category: %s\n", issue.Category.Name)
		}
		if issue.Parent != nil {
			fmt.Printf("Parent:   #%d\n", issue.Parent.ID)
		}
		if issue.FixedVersion != nil {
			fmt.Printf("Version:  %s\n", issue.FixedVersion.Name)
		}
		if issue.StartDate != "" {
			fmt.Printf("Start:    %s\n", issue.StartDate)
		}
		if issue.DueDate != "" {
			fmt.Printf("Due:      %s\n", issue.DueDate)
		}
		if issue.DoneRatio > 0 {
			fmt.Printf("Progress: %d%%\n", issue.DoneRatio)
		}
		if issue.EstimatedHours != nil {
			fmt.Printf("Estimate: %.2fh\n", *issue.EstimatedHours)
		}
		if issue.SpentHours != nil {
			fmt.Printf("Spent:    %.2fh\n", *issue.SpentHours)
		}
		for _, f := range issue.CustomFields {
			fmt.Printf("%s (id %d): %s\n", f.Name, f.ID, f.Value)
		}
		if issue.Description != "" {
			fmt.Printf("\n%s\n", issue.Description)
		}
		if len(issue.Attachments) > 0 {
			fmt.Printf("\nAttachments:\n")
			for _, a := range issue.Attachments {
				fmt.Printf("  [%d] %s (%s, %d bytes)\n", a.ID, a.Filename, a.ContentType, a.Filesize)
			}
		}
		for _, j := range issue.Journals {
			if j.Notes == "" {
				continue // a bare field change, not a comment
			}
			fmt.Printf("\n--- %s, %s ---\n%s\n", j.User.Name, j.CreatedOn.Format("2006-01-02 15:04"), j.Notes)
		}
		return nil
	},
}

var issueAttachmentsCmd = &cobra.Command{
	Use:   "attachments <id>",
	Short: "List an issue's attachments, optionally downloading them",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseIssueID(args[0])
		if err != nil {
			return err
		}
		dir, _ := cmd.Flags().GetString("download")
		client, err := newClient()
		if err != nil {
			return err
		}
		issue, err := client.GetIssue(id, false)
		if err != nil {
			return err
		}

		if dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("creating download directory: %w", err)
			}
			saved := make([]downloadedFile, 0, len(issue.Attachments))
			for _, a := range issue.Attachments {
				path := filepath.Join(dir, filepath.Base(a.Filename))
				if err := downloadTo(client, a.ContentURL, path); err != nil {
					return fmt.Errorf("downloading %s: %w", a.Filename, err)
				}
				saved = append(saved, downloadedFile{ID: a.ID, Filename: a.Filename, Path: path})
				if !wantsJSON() {
					fmt.Printf("Downloaded %s\n", path)
				}
			}
			if wantsJSON() {
				return printJSON(saved)
			}
			if len(saved) == 0 {
				fmt.Printf("Issue #%d has no attachments\n", id)
			}
			return nil
		}

		if wantsJSON() {
			return printJSON(issue.Attachments)
		}
		if len(issue.Attachments) == 0 {
			fmt.Printf("Issue #%d has no attachments\n", id)
			return nil
		}
		rows := make([][]string, 0, len(issue.Attachments))
		for _, a := range issue.Attachments {
			rows = append(rows, []string{
				strconv.Itoa(a.ID),
				a.Filename,
				a.ContentType,
				strconv.Itoa(a.Filesize),
			})
		}
		printTable([]string{"ID", "FILENAME", "TYPE", "BYTES"}, rows)
		return nil
	},
}

// downloadFile is one attachment saved by `issue attachments --download`, so
// that a -o json caller learns where each file landed rather than having to
// re-derive the paths.
type downloadedFile struct {
	ID       int    `json:"id"`
	Filename string `json:"filename"`
	Path     string `json:"path"`
}

// downloadTo streams one attachment to path, making sure a failed download
// doesn't leave a half-written file behind for a caller to parse as a spec.
func downloadTo(client *redmine.Client, contentURL, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := client.Download(contentURL, f); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	return f.Close()
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
		parent, _ := cmd.Flags().GetInt("parent")
		startDate, _ := cmd.Flags().GetString("start-date")
		dueDate, _ := cmd.Flags().GetString("due-date")
		estimated, _ := cmd.Flags().GetFloat64("estimated-hours")
		doneRatio, _ := cmd.Flags().GetInt("done-ratio")
		fieldArgs, _ := cmd.Flags().GetStringArray("field")

		if err := validateDates(dateFlag{"--start-date", startDate}, dateFlag{"--due-date", dueDate}); err != nil {
			return err
		}
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
			ProjectID:      project,
			Subject:        subject,
			Description:    description,
			AssignedTo:     assignee,
			ParentID:       parent,
			StartDate:      startDate,
			DueDate:        dueDate,
			EstimatedHours: estimated,
			DoneRatio:      doneRatio,
			CustomFields:   customFields,
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

		trackerName, _ := cmd.Flags().GetString("tracker")
		priorityName, _ := cmd.Flags().GetString("priority")
		statusName, _ := cmd.Flags().GetString("status")
		categoryName, _ := cmd.Flags().GetString("category")
		startDate, _ := cmd.Flags().GetString("start-date")
		dueDate, _ := cmd.Flags().GetString("due-date")
		doneRatio, _ := cmd.Flags().GetInt("done-ratio")
		fieldArgs, _ := cmd.Flags().GetStringArray("field")

		if err := validateDates(dateFlag{"--start-date", startDate}, dateFlag{"--due-date", dueDate}); err != nil {
			return err
		}
		if cmd.Flags().Changed("done-ratio") && (doneRatio < 0 || doneRatio > 100) {
			return fmt.Errorf("--done-ratio must be between 0 and 100, got %d", doneRatio)
		}
		customFields, err := parseCustomFields(fieldArgs)
		if err != nil {
			return err
		}

		client, err := newClient()
		if err != nil {
			return err
		}

		// Only flags the user actually typed become part of the request, so
		// an update never rewrites a field it was not asked about.
		req := redmine.UpdateIssueRequest{
			Subject:        flagString(cmd, "subject"),
			Description:    flagString(cmd, "description"),
			AssignedTo:     flagInt(cmd, "assignee"),
			ParentID:       flagInt(cmd, "parent"),
			StartDate:      flagString(cmd, "start-date"),
			DueDate:        flagString(cmd, "due-date"),
			EstimatedHours: flagFloat64(cmd, "estimated-hours"),
			DoneRatio:      flagInt(cmd, "done-ratio"),
			CustomFields:   customFields,
		}
		if trackerName != "" {
			id, err := client.ResolveTrackerID(trackerName)
			if err != nil {
				return err
			}
			req.TrackerID = &id
		}
		if priorityName != "" {
			id, err := client.ResolveIssuePriorityID(priorityName)
			if err != nil {
				return err
			}
			req.PriorityID = &id
		}
		if statusName != "" {
			id, err := client.ResolveIssueStatusID(statusName)
			if err != nil {
				return err
			}
			req.StatusID = &id
		}
		if cmd.Flags().Changed("category") {
			categoryID := 0 // --category "" clears it
			if categoryName != "" {
				issue, err := client.GetIssue(id, false)
				if err != nil {
					return err
				}
				categoryID, err = client.ResolveIssueCategoryID(strconv.Itoa(issue.Project.ID), categoryName)
				if err != nil {
					return err
				}
			}
			req.CategoryID = &categoryID
		}

		if err := client.UpdateIssue(id, req); err != nil {
			return err
		}
		return printAction(fmt.Sprintf("Updated issue #%d", id), actionResult{Status: "updated", Issue: id})
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

		if err := client.UpdateIssue(id, redmine.UpdateIssueRequest{StatusID: &statusID}); err != nil {
			return err
		}
		return printAction(fmt.Sprintf("Closed issue #%d", id), actionResult{Status: "closed", Issue: id})
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
		return printAction(fmt.Sprintf("Commented on issue #%d", id), actionResult{Status: "commented", Issue: id})
	},
}

func init() {
	issueListCmd.Flags().String("project", "", "filter by project ID, identifier, or name (e.g. \"AssetSonar Scrum Team\")")
	issueListCmd.Flags().String("status", "", "filter by status name (e.g. In Progress), status ID, or open/closed/*")
	issueListCmd.Flags().String("assignee", "", "filter by assignee user ID (or \"me\" for the authenticated user)")
	issueListCmd.Flags().String("tracker", "", "filter by tracker name (e.g. Bug) or tracker ID")
	issueListCmd.Flags().String("subject", "", "only issues whose subject contains this text")
	issueListCmd.Flags().String("updated-after", "", "only issues updated on or after this date (YYYY-MM-DD)")
	issueListCmd.Flags().String("updated-before", "", "only issues updated on or before this date (YYYY-MM-DD)")
	issueListCmd.Flags().String("due-after", "", "only issues due on or after this date (YYYY-MM-DD)")
	issueListCmd.Flags().String("due-before", "", "only issues due on or before this date (YYYY-MM-DD)")
	issueListCmd.Flags().Int("due-within", 0, "only issues due within this many days from today")
	issueListCmd.Flags().Bool("due-next-week", false, "only issues due next week (Mon-Sun)")
	issueListCmd.Flags().Bool("overdue", false, "only issues whose due date has already passed")
	issueListCmd.Flags().String("sort", "", "sort order, e.g. due_date or \"priority:desc,due_date:asc\"")
	issueListCmd.Flags().Int("limit", 25, "maximum number of issues to return")
	issueListCmd.Flags().Bool("all", false, "fetch every matching issue, ignoring --limit")

	issueCreateCmd.Flags().String("project", "", "project ID, identifier, or name (required)")
	issueCreateCmd.Flags().String("subject", "", "issue subject (required)")
	issueCreateCmd.Flags().String("description", "", "issue description")
	issueCreateCmd.Flags().String("tracker", "", "tracker name, e.g. Bug")
	issueCreateCmd.Flags().String("priority", "", "priority name, e.g. High")
	issueCreateCmd.Flags().String("category", "", "issue category name (project-specific; see `rmine project categories <project>`)")
	issueCreateCmd.Flags().Int("assignee", 0, "assignee user ID")
	issueCreateCmd.Flags().Int("parent", 0, "parent issue ID")
	issueCreateCmd.Flags().String("start-date", "", "start date (YYYY-MM-DD)")
	issueCreateCmd.Flags().String("due-date", "", "due date (YYYY-MM-DD)")
	issueCreateCmd.Flags().Float64("estimated-hours", 0, "estimated hours")
	issueCreateCmd.Flags().Int("done-ratio", 0, "percent complete (0-100)")
	issueCreateCmd.Flags().StringArray("field", nil, "custom field as id=value (repeatable); find IDs via `rmine issue view <id> -o json` on an existing issue")
	_ = issueCreateCmd.MarkFlagRequired("project")
	_ = issueCreateCmd.MarkFlagRequired("subject")

	issueUpdateCmd.Flags().String("subject", "", "new subject")
	issueUpdateCmd.Flags().String("description", "", "new description")
	issueUpdateCmd.Flags().String("tracker", "", "new tracker name")
	issueUpdateCmd.Flags().String("priority", "", "new priority name")
	issueUpdateCmd.Flags().String("status", "", "new status name")
	issueUpdateCmd.Flags().String("category", "", "new category name, or \"\" to clear (project-specific; see `rmine project categories <project>`)")
	issueUpdateCmd.Flags().Int("assignee", 0, "new assignee user ID (0 unassigns)")
	issueUpdateCmd.Flags().Int("parent", 0, "new parent issue ID (0 detaches from the parent)")
	issueUpdateCmd.Flags().String("start-date", "", "new start date (YYYY-MM-DD)")
	issueUpdateCmd.Flags().String("due-date", "", "new due date (YYYY-MM-DD)")
	issueUpdateCmd.Flags().Float64("estimated-hours", 0, "new estimated hours (0 clears the estimate)")
	issueUpdateCmd.Flags().Int("done-ratio", 0, "new percent complete (0-100)")
	issueUpdateCmd.Flags().StringArray("field", nil, "custom field as id=value (repeatable)")

	issueCloseCmd.Flags().String("status", "", "status name to close with (defaults to the server's first closed status)")

	issueViewCmd.Flags().Bool("comments", false, "also fetch and show the issue's comments")

	issueAttachmentsCmd.Flags().String("download", "", "download every attachment into this directory")

	issueCmd.AddCommand(issueListCmd, issueViewCmd, issueAttachmentsCmd, issueCreateCmd, issueUpdateCmd, issueCloseCmd, issueCommentCmd)
	rootCmd.AddCommand(issueCmd)
}

// flagString, flagInt and flagFloat64 return a pointer to a flag's value if
// the user set it on the command line, and nil otherwise. Update requests
// send only their non-nil fields, so an unset flag leaves the server's value
// alone while an explicitly-passed empty one clears it.
func flagString(cmd *cobra.Command, name string) *string {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	v, _ := cmd.Flags().GetString(name)
	return &v
}

func flagInt(cmd *cobra.Command, name string) *int {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	v, _ := cmd.Flags().GetInt(name)
	return &v
}

func flagFloat64(cmd *cobra.Command, name string) *float64 {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	v, _ := cmd.Flags().GetFloat64(name)
	return &v
}

// normalizeSort checks the shape of a Redmine sort spec — comma-separated
// columns, each optionally suffixed with :asc or :desc — and returns it with
// incidental whitespace removed. It normalizes rather than only validating
// so that a spec accepted here is exactly what reaches the server: `--sort
// "priority:desc, due_date:asc"` is a natural thing to type, and the space
// after the comma would otherwise travel into the query.
//
// Column names are deliberately not checked against a list. Redmine sorts on
// custom fields too (cf_12), which vary per instance, so a whitelist would
// reject valid input; a bad column is one of the few things Redmine does
// report clearly.
func normalizeSort(spec string) (string, error) {
	if strings.TrimSpace(spec) == "" {
		return "", nil
	}

	parts := strings.Split(spec, ",")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		column, direction, hasDirection := strings.Cut(strings.TrimSpace(part), ":")
		column = strings.TrimSpace(column)
		direction = strings.TrimSpace(direction)

		if column == "" {
			return "", fmt.Errorf("--sort has an empty column in %q", spec)
		}
		if !hasDirection {
			cleaned = append(cleaned, column)
			continue
		}
		if direction != "asc" && direction != "desc" {
			return "", fmt.Errorf("--sort direction must be asc or desc, got %q in %q", direction, spec)
		}
		cleaned = append(cleaned, column+":"+direction)
	}
	return strings.Join(cleaned, ","), nil
}

// dateFlag pairs a flag name with the value the user gave it, so a rejection
// can name the flag that was wrong.
type dateFlag struct {
	name  string
	value string
}

// validateDates checks every non-empty date flag against Redmine's YYYY-MM-DD
// format. Redmine answers a malformed filter date with an empty result rather
// than an error, so an unchecked typo reads as "nothing matched".
func validateDates(flags ...dateFlag) error {
	for _, f := range flags {
		if f.value == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", f.value); err != nil {
			return fmt.Errorf("%s must be a YYYY-MM-DD date, got %q", f.name, f.value)
		}
	}
	return nil
}

// issueWithURL decorates an issue with its address in the Redmine web UI.
// redmine.Issue mirrors the API response, so a field the API never sends
// belongs here rather than in that type.
type issueWithURL struct {
	*redmine.Issue
	URL string `json:"url"`
}

func withIssueURL(client *redmine.Client, issue *redmine.Issue) issueWithURL {
	return issueWithURL{Issue: issue, URL: issueURL(client, issue.ID)}
}

// issueURL builds the browser link for an issue. rmine reports issues by
// number, which is not something a person can open — and an agent relaying a
// result had no way to produce a link at all.
func issueURL(client *redmine.Client, id int) string {
	return fmt.Sprintf("%s/issues/%d", client.BaseURL(), id)
}

// orDash renders an empty optional value as "-" so a column never collapses
// to whitespace the eye has to count.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
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

// resolveTrackerFilter resolves a tracker name (e.g. "Bug") to its ID and
// passes numeric IDs through unchanged.
//
// `issue create --tracker Bug` always took a name, while `issue list
// --tracker` sent whatever it was given straight to Redmine as tracker_id.
// Redmine casts a non-numeric tracker_id to 0, so the name spelling matched
// nothing and returned an empty list with a 200 — the same answer as "no such
// issues", with nothing to distinguish the two.
func resolveTrackerFilter(client *redmine.Client, tracker string) (string, error) {
	if tracker == "" {
		return "", nil
	}
	if _, err := strconv.Atoi(tracker); err == nil {
		return tracker, nil
	}
	id, err := client.ResolveTrackerID(tracker)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(id), nil
}

// resolveUserFilter validates a user filter, which Redmine accepts only as a
// numeric ID or the literal "me". rmine has no name-to-ID lookup, and passing
// a name through would filter on a user_id of 0 and quietly return nothing,
// so a name is rejected with an error that says as much rather than being
// answered with a plausible-looking empty result.
func resolveUserFilter(flag, value string) (string, error) {
	if value == "" || value == "me" {
		return value, nil
	}
	if _, err := strconv.Atoi(value); err == nil {
		return value, nil
	}
	return "", fmt.Errorf("%s takes a numeric Redmine user ID or \"me\", not a name like %q — rmine cannot look users up by name", flag, value)
}

// resolveIDFilter validates a filter Redmine only accepts as a numeric ID,
// for the same reason as resolveUserFilter: a non-numeric value is cast to 0
// server-side and silently matches nothing.
func resolveIDFilter(flag, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if _, err := strconv.Atoi(value); err == nil {
		return value, nil
	}
	return "", fmt.Errorf("%s takes a numeric ID, got %q", flag, value)
}

// resolveProjectFilter passes numeric IDs through unchanged and resolves a
// display name or identifier to its numeric ID via the server's projects.
//
// It used to swallow every lookup error and pass the raw string on, which hid
// two different problems behind one confusing server-side rejection: a
// misspelled project name, and a lookup that never completed. The two are now
// separated, because only one of them means the user got something wrong.
func resolveProjectFilter(client *redmine.Client, project string) (string, error) {
	if project == "" {
		return "", nil
	}
	if _, err := strconv.Atoi(project); err == nil {
		return project, nil
	}

	id, err := client.ResolveProjectID(project)
	if err == nil {
		return strconv.Itoa(id), nil
	}
	if errors.Is(err, redmine.ErrNoMatch) {
		// The lookup worked and nothing matched: a typo, or a project this
		// API key cannot see. Say so.
		return "", err
	}
	// The lookup itself failed — no permission to list projects, a transport
	// error. We have not established that the value is wrong, so pass it
	// through: Redmine resolves identifiers server-side, and if the failure
	// is real the request that follows surfaces it.
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
// the --due-within/--due-next-week/--overdue shortcuts into a single
// (after, before) range, relative to now. The shortcuts are mutually
// exclusive with each other and with the raw flags.
func resolveDueDateRange(now time.Time, after, before string, within int, nextWeek, overdue bool) (string, string, error) {
	shortcuts := 0
	if within > 0 {
		shortcuts++
	}
	if nextWeek {
		shortcuts++
	}
	if overdue {
		shortcuts++
	}
	if shortcuts > 1 {
		return "", "", fmt.Errorf("only one of --due-within, --due-next-week or --overdue may be set")
	}
	if shortcuts > 0 && (after != "" || before != "") {
		return "", "", fmt.Errorf("--due-within/--due-next-week/--overdue cannot be combined with --due-after/--due-before")
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
	case overdue:
		// Everything due strictly before today. Redmine's plain filters
		// already restrict to open issues unless --status says otherwise, so
		// this does not drag in issues that were closed late.
		return "", now.AddDate(0, 0, -1).Format(layout), nil
	default:
		return after, before, nil
	}
}
