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
	Use:   "log <issue-id>",
	Short: "Log time against an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueID, err := parseIssueID(args[0])
		if err != nil {
			return err
		}

		hours, _ := cmd.Flags().GetFloat64("hours")
		date, _ := cmd.Flags().GetString("date")
		activityName, _ := cmd.Flags().GetString("activity")
		comment, _ := cmd.Flags().GetString("comment")

		client, err := newClient()
		if err != nil {
			return err
		}

		req := redmine.CreateTimeEntryRequest{
			IssueID:  issueID,
			Hours:    hours,
			Comments: comment,
			SpentOn:  date,
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
		fmt.Printf("Logged %.2fh on issue #%d (entry #%d)\n", entry.Hours, issueID, entry.ID)
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
		limit, _ := cmd.Flags().GetInt("limit")
		all, _ := cmd.Flags().GetBool("all")

		client, err := newClient()
		if err != nil {
			return err
		}

		project, err = resolveProjectFilter(client, project)
		if err != nil {
			return err
		}

		filter := redmine.TimeEntryListFilter{
			IssueID:   issue,
			ProjectID: project,
			UserID:    user,
			From:      from,
			To:        to,
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

		hours, _ := cmd.Flags().GetFloat64("hours")
		date, _ := cmd.Flags().GetString("date")
		activityName, _ := cmd.Flags().GetString("activity")
		comment, _ := cmd.Flags().GetString("comment")

		client, err := newClient()
		if err != nil {
			return err
		}

		req := redmine.UpdateTimeEntryRequest{
			Hours:    hours,
			Comments: comment,
			SpentOn:  date,
		}
		if activityName != "" {
			req.ActivityID, err = client.ResolveTimeEntryActivityID(activityName)
			if err != nil {
				return err
			}
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
	timeLogCmd.Flags().Float64("hours", 0, "hours spent (required)")
	timeLogCmd.Flags().String("date", "", "date the time was spent, YYYY-MM-DD (defaults to today)")
	timeLogCmd.Flags().String("activity", "", "activity name, e.g. Development")
	timeLogCmd.Flags().String("comment", "", "comment for this entry")
	_ = timeLogCmd.MarkFlagRequired("hours")

	timeListCmd.Flags().String("issue", "", "filter by issue ID")
	timeListCmd.Flags().String("project", "", "filter by project ID, identifier, or name")
	timeListCmd.Flags().String("user", "", "filter by user ID")
	timeListCmd.Flags().String("from", "", "only entries spent on or after this date (YYYY-MM-DD)")
	timeListCmd.Flags().String("to", "", "only entries spent on or before this date (YYYY-MM-DD)")
	timeListCmd.Flags().Int("limit", 25, "maximum number of entries to return")
	timeListCmd.Flags().Bool("all", false, "fetch every matching entry, ignoring --limit")

	timeEditCmd.Flags().Float64("hours", 0, "new hours value")
	timeEditCmd.Flags().String("date", "", "new date, YYYY-MM-DD")
	timeEditCmd.Flags().String("activity", "", "new activity name")
	timeEditCmd.Flags().String("comment", "", "new comment")

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
