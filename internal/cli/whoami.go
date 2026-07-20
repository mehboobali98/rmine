package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the user associated with the active profile's API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		user, err := client.Whoami()
		if err != nil {
			return err
		}

		if wantsJSON() {
			return printJSON(user)
		}
		fmt.Printf("%s %s (%s) <%s>\n", user.FirstName, user.LastName, user.Login, user.Mail)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}
