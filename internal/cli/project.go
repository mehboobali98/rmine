package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mehboobali98/rmine/internal/redmine"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Browse Redmine projects",
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		projects, err := client.ListProjects()
		if err != nil {
			return err
		}

		if wantsJSON() {
			return printJSON(projects)
		}

		rows := make([][]string, 0, len(projects))
		for _, p := range projects {
			rows = append(rows, []string{strconv.Itoa(p.ID), p.Identifier, p.Name})
		}
		printTable([]string{"ID", "IDENTIFIER", "NAME"}, rows)
		return nil
	},
}

var projectViewCmd = &cobra.Command{
	Use:   "view <project>",
	Short: "Show a project's details, trackers and categories",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		// Every other project-taking command accepts a display name, because
		// each resolves through here first; this one used to hand the raw
		// argument to Redmine, which only knows IDs and identifiers, so the
		// one spelling a person is most likely to have was the one spelling
		// that 404'd.
		project, err := resolveProjectFilter(client, args[0])
		if err != nil {
			return err
		}
		detail, err := client.GetProjectDetail(project)
		if err != nil {
			return err
		}

		if wantsJSON() {
			return printJSON(detail)
		}

		fmt.Printf("#%d %s (%s)\n", detail.ID, detail.Name, detail.Identifier)
		if detail.Parent != nil {
			printField("Parent", detail.Parent.Name)
		}
		if detail.Homepage != "" {
			printField("Homepage", detail.Homepage)
		}
		printField("Status", projectStatus(detail.Status))
		if detail.IsPublic != nil {
			printField("Public", strconv.FormatBool(*detail.IsPublic))
		}
		printNameList("Trackers", detail.Trackers)
		printNameList("Categories", detail.IssueCategories)
		printNameList("Modules", detail.EnabledModules)
		if detail.Description != "" {
			fmt.Printf("\n%s\n", detail.Description)
		}
		return nil
	},
}

var projectVersionsCmd = &cobra.Command{
	Use:   "versions <project>",
	Short: "List a project's target versions",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		project, err := resolveProjectFilter(client, args[0])
		if err != nil {
			return err
		}
		versions, err := client.ListVersions(project)
		if err != nil {
			return err
		}

		if wantsJSON() {
			return printJSON(versions)
		}
		if len(versions) == 0 {
			fmt.Printf("Project %s has no versions\n", args[0])
			return nil
		}
		rows := make([][]string, 0, len(versions))
		for _, v := range versions {
			rows = append(rows, []string{
				strconv.Itoa(v.ID),
				v.Name,
				v.Status,
				orDash(v.DueDate),
				v.Project.Name,
			})
		}
		// SHARED FROM, because a version inherited from a parent project is
		// settable here but is not listed under this project in the web UI —
		// which otherwise reads as rmine inventing versions.
		printTable([]string{"ID", "NAME", "STATUS", "DUE", "SHARED FROM"}, rows)
		return nil
	},
}

// projectStatus names Redmine's numeric project states.
func projectStatus(status int) string {
	switch status {
	case 1:
		return "active"
	case 5:
		return "closed"
	case 9:
		return "archived"
	default:
		return strconv.Itoa(status)
	}
}

// printField renders one labelled line. The width fits the longest label this
// view uses ("Categories:"), so every value starts in the same column.
func printField(label, value string) {
	fmt.Printf("%-11s %s\n", label+":", value)
}

// printNameList renders one of a project's embedded name lists, skipping the
// heading entirely when the server did not send that include.
func printNameList(label string, items []redmine.IDName) {
	if len(items) == 0 {
		return
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	printField(label, strings.Join(names, ", "))
}

var projectCategoriesCmd = &cobra.Command{
	Use:   "categories <project>",
	Short: "List a project's issue categories",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		project, err := resolveProjectFilter(client, args[0])
		if err != nil {
			return err
		}
		categories, err := client.ListIssueCategories(project)
		if err != nil {
			return err
		}

		if wantsJSON() {
			return printJSON(categories)
		}

		rows := make([][]string, 0, len(categories))
		for _, c := range categories {
			rows = append(rows, []string{strconv.Itoa(c.ID), c.Name})
		}
		printTable([]string{"ID", "NAME"}, rows)
		return nil
	},
}

func init() {
	projectCmd.AddCommand(projectListCmd, projectViewCmd, projectCategoriesCmd, projectVersionsCmd)
	rootCmd.AddCommand(projectCmd)
}
