package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mehboobali98/rmine/internal/redmine"
)

var timeCmd = &cobra.Command{
	Use:   "time",
	Short: "Log and manage time entries",
}

var timeLogCmd = &cobra.Command{
	Use:   "log [issue-id]",
	Short: "Log time against an issue, or against a project with --project",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		hours, _ := cmd.Flags().GetFloat64("hours")
		date, _ := cmd.Flags().GetString("date")
		activityName, _ := cmd.Flags().GetString("activity")
		comment, _ := cmd.Flags().GetString("comment")

		// Redmine attaches a time entry to exactly one of an issue or a
		// project, so requiring the same here turns an ambiguous command into
		// an error before it reaches the server.
		var issueID int
		switch {
		case len(args) == 1 && project != "":
			return fmt.Errorf("pass an issue ID or --project, not both")
		case len(args) == 0 && project == "":
			return fmt.Errorf("pass an issue ID, or --project to log against a project instead")
		case len(args) == 1:
			var err error
			if issueID, err = parseIssueID(args[0]); err != nil {
				return err
			}
		}

		if err := validateDates(dateFlag{"--date", date}); err != nil {
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

		req := redmine.CreateTimeEntryRequest{
			IssueID:   issueID,
			ProjectID: project,
			Hours:     hours,
			Comments:  comment,
			SpentOn:   date,
		}
		if activityName != "" {
			req.ActivityID, err = client.ResolveTimeEntryActivityID(activityName)
			if err != nil {
				return err
			}
		}

		entry, err := client.CreateTimeEntry(req)
		if err != nil {
			return err
		}

		if wantsJSON() {
			return printJSON(entry)
		}
		if issueID != 0 {
			fmt.Printf("Logged %.2fh on issue #%d (entry #%d)\n", entry.Hours, issueID, entry.ID)
		} else {
			fmt.Printf("Logged %.2fh on project %s (entry #%d)\n", entry.Hours, entry.Project.Name, entry.ID)
		}
		return nil
	},
}

var timeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List time entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		issue, _ := cmd.Flags().GetString("issue")
		project, _ := cmd.Flags().GetString("project")
		user, _ := cmd.Flags().GetString("user")
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		allProjects, _ := cmd.Flags().GetBool("all-projects")
		sort, _ := cmd.Flags().GetString("sort")
		limit, _ := cmd.Flags().GetInt("limit")
		all, _ := cmd.Flags().GetBool("all")

		sort, err := normalizeSort(sort)
		if err != nil {
			return err
		}
		if err := validateDates(dateFlag{"--from", from}, dateFlag{"--to", to}); err != nil {
			return err
		}

		project, err = projectFilterOrDefault(project, allProjects)
		if err != nil {
			return err
		}

		client, err := newClient()
		if err != nil {
			return err
		}

		projectArg := project
		project, err = resolveProjectFilter(client, project)
		if err != nil {
			return err
		}
		user, err = resolveUserFilter(client, "--user", user, projectArg)
		if err != nil {
			return err
		}
		issue, err = resolveIDFilter("--issue", issue)
		if err != nil {
			return err
		}

		filter := redmine.TimeEntryListFilter{
			IssueID:   issue,
			ProjectID: project,
			UserID:    user,
			From:      from,
			To:        to,
			Sort:      sort,
			Limit:     limit,
			All:       all,
		}
		entries, err := client.ListTimeEntries(filter)
		if err != nil {
			return err
		}

		if wantsJSON() {
			return printJSON(entries)
		}

		var total float64
		rows := make([][]string, 0, len(entries))
		for _, e := range entries {
			issueCol := "-"
			if e.Issue != nil {
				issueCol = "#" + strconv.Itoa(e.Issue.ID)
			}
			rows = append(rows, []string{
				strconv.Itoa(e.ID),
				e.SpentOn,
				issueCol,
				e.Project.Name,
				e.Activity.Name,
				strconv.FormatFloat(e.Hours, 'f', 2, 64),
				e.Comments,
			})
			total += e.Hours
		}
		printTable([]string{"ID", "DATE", "ISSUE", "PROJECT", "ACTIVITY", "HOURS", "COMMENT"}, rows)
		fmt.Printf("\nTotal: %.2fh across %d entries\n", total, len(entries))
		return nil
	},
}

var timeEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit a time entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseIssueID(args[0])
		if err != nil {
			return err
		}

		date, _ := cmd.Flags().GetString("date")
		activityName, _ := cmd.Flags().GetString("activity")

		if err := validateDates(dateFlag{"--date", date}); err != nil {
			return err
		}

		client, err := newClient()
		if err != nil {
			return err
		}

		// Only the flags actually passed are sent, so editing the hours on an
		// entry no longer blanks nothing else by accident — and --comment ""
		// now genuinely clears the comment.
		req := redmine.UpdateTimeEntryRequest{
			Hours:    flagFloat64(cmd, "hours"),
			Comments: flagString(cmd, "comment"),
			SpentOn:  flagString(cmd, "date"),
		}
		if activityName != "" {
			activityID, err := client.ResolveTimeEntryActivityID(activityName)
			if err != nil {
				return err
			}
			req.ActivityID = &activityID
		}

		if err := client.UpdateTimeEntry(id, req); err != nil {
			return err
		}
		return printAction(fmt.Sprintf("Updated time entry #%d", id), actionResult{Status: "updated", TimeEntry: id})
	},
}

var timeDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a time entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseIssueID(args[0])
		if err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")

		if !force && !confirm(fmt.Sprintf("Delete time entry #%d?", id), false) {
			return printAction("Aborted.", actionResult{Status: "aborted", TimeEntry: id})
		}

		client, err := newClient()
		if err != nil {
			return err
		}
		if err := client.DeleteTimeEntry(id); err != nil {
			return err
		}
		return printAction(fmt.Sprintf("Deleted time entry #%d", id), actionResult{Status: "deleted", TimeEntry: id})
	},
}

func init() {
	timeLogCmd.Flags().String("project", "", "log against this project instead of an issue (ID, identifier, or name)")
	timeLogCmd.Flags().Float64("hours", 0, "hours spent (required)")
	timeLogCmd.Flags().String("date", "", "date the time was spent, YYYY-MM-DD (defaults to today)")
	timeLogCmd.Flags().String("activity", "", "activity name, e.g. Development")
	timeLogCmd.Flags().String("comment", "", "comment for this entry")
	_ = timeLogCmd.MarkFlagRequired("hours")

	timeListCmd.Flags().String("issue", "", "filter by issue ID")
	timeListCmd.Flags().String("project", "", "filter by project ID, identifier, or name (defaults to the profile's default project)")
	timeListCmd.Flags().Bool("all-projects", false, "include every project, ignoring the profile's default project")
	timeListCmd.Flags().String("user", "", "filter by user: user ID, \"me\", or a name (needs --project to resolve a name)")
	timeListCmd.Flags().String("from", "", "only entries spent on or after this date (YYYY-MM-DD)")
	timeListCmd.Flags().String("to", "", "only entries spent on or before this date (YYYY-MM-DD)")
	timeListCmd.Flags().String("sort", "", "sort order, e.g. spent_on or \"spent_on:desc\"")
	timeListCmd.Flags().Int("limit", 25, "maximum number of entries to return")
	timeListCmd.Flags().Bool("all", false, "fetch every matching entry, ignoring --limit")

	timeEditCmd.Flags().Float64("hours", 0, "new hours value")
	timeEditCmd.Flags().String("date", "", "new date, YYYY-MM-DD")
	timeEditCmd.Flags().String("activity", "", "new activity name")
	timeEditCmd.Flags().String("comment", "", "new comment (\"\" clears it)")

	timeDeleteCmd.Flags().BoolP("force", "y", false, "skip the confirmation prompt")

	timeCmd.AddCommand(timeLogCmd, timeListCmd, timeEditCmd, timeDeleteCmd)
	rootCmd.AddCommand(timeCmd)
}

// confirm asks a yes/no question on stdin. defaultYes controls what
// pressing enter with no answer means.
func confirm(prompt string, defaultYes bool) bool {
	hint := "[y/N]"
	if defaultYes {
		hint = "[Y/n]"
	}
	promptf("%s %s ", prompt, hint)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer == "" {
		return defaultYes
	}
	return answer == "y" || answer == "yes"
}
