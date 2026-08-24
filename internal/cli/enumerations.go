package cli

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mehboobali98/rmine/internal/redmine"
)

// The server-wide vocabularies. Every one of these already backed a
// name-matching flag — `--tracker Bug`, `--status "In Progress"`,
// `--priority High`, `--activity Development` — with no way to ask what the
// valid spellings were, so the only route to a working command was to guess
// one, or to sample an existing issue and read the name off it.

var trackerCmd = &cobra.Command{
	Use:   "tracker",
	Short: "Browse issue trackers",
}

var trackerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the trackers defined on this server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listNamed(func(c *redmine.Client) ([]redmine.IDName, error) {
			return c.ListTrackers()
		})
	},
}

var priorityCmd = &cobra.Command{
	Use:   "priority",
	Short: "Browse issue priorities",
}

var priorityListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the issue priorities defined on this server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listNamed(func(c *redmine.Client) ([]redmine.IDName, error) {
			return c.ListIssuePriorities()
		})
	},
}

var activityCmd = &cobra.Command{
	Use:   "activity",
	Short: "Browse time-entry activities",
}

var activityListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the time-entry activities defined on this server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listNamed(func(c *redmine.Client) ([]redmine.IDName, error) {
			return c.ListTimeEntryActivities()
		})
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Browse issue statuses",
}

// Statuses get their own command rather than going through listNamed: which
// statuses close an issue is the question `issue close` answers by picking
// the first closed one, and a caller checking that guess needs the flag.
var statusListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the issue statuses defined on this server",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		statuses, err := client.ListIssueStatuses()
		if err != nil {
			return err
		}

		if wantsJSON() {
			return printJSON(statuses)
		}
		rows := make([][]string, 0, len(statuses))
		for _, s := range statuses {
			rows = append(rows, []string{strconv.Itoa(s.ID), s.Name, strconv.FormatBool(s.IsClosed)})
		}
		printTable([]string{"ID", "NAME", "CLOSED"}, rows)
		return nil
	},
}

// listNamed prints one of the server's {id, name} vocabularies.
func listNamed(fetch func(*redmine.Client) ([]redmine.IDName, error)) error {
	client, err := newClient()
	if err != nil {
		return err
	}
	items, err := fetch(client)
	if err != nil {
		return err
	}

	if wantsJSON() {
		return printJSON(items)
	}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{strconv.Itoa(item.ID), item.Name})
	}
	printTable([]string{"ID", "NAME"}, rows)
	return nil
}

func init() {
	trackerCmd.AddCommand(trackerListCmd)
	statusCmd.AddCommand(statusListCmd)
	priorityCmd.AddCommand(priorityListCmd)
	activityCmd.AddCommand(activityListCmd)
	rootCmd.AddCommand(trackerCmd, statusCmd, priorityCmd, activityCmd)
}
