package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
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
	Use:   "view <id-or-identifier>",
	Short: "Show a project's details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		project, err := client.GetProject(args[0])
		if err != nil {
			return err
		}

		if wantsJSON() {
			return printJSON(project)
		}

		fmt.Printf("#%d %s (%s)\n", project.ID, project.Name, project.Identifier)
		if project.Description != "" {
			fmt.Println(project.Description)
		}
		return nil
	},
}

func init() {
	projectCmd.AddCommand(projectListCmd, projectViewCmd)
	rootCmd.AddCommand(projectCmd)
}
