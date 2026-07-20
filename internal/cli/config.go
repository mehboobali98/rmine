package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/mehboobali98/rdm/internal/config"
	"github.com/mehboobali98/rdm/internal/redmine"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage rdm's server profiles",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up the default profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		return addProfile("default", true)
	},
}

var configAddProfileCmd = &cobra.Command{
	Use:   "add-profile <name>",
	Short: "Add another server profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return addProfile(args[0], false)
	},
}

var configUseProfileCmd = &cobra.Command{
	Use:   "use-profile <name>",
	Short: "Switch the active profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if _, ok := cfg.Profiles[name]; !ok {
			return fmt.Errorf("no such profile %q", name)
		}
		cfg.CurrentProfile = name
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("Switched to profile %q\n", name)
		return nil
	},
}

var configListProfilesCmd = &cobra.Command{
	Use:   "list-profiles",
	Short: "List configured server profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if len(cfg.Profiles) == 0 {
			fmt.Println("No profiles configured yet — run `rdm config init`.")
			return nil
		}

		rows := make([][]string, 0, len(cfg.Profiles))
		for name, p := range cfg.Profiles {
			current := ""
			if name == cfg.CurrentProfile {
				current = "*"
			}
			rows = append(rows, []string{current, name, p.URL})
		}
		printTable([]string{"", "NAME", "URL"}, rows)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd, configAddProfileCmd, configUseProfileCmd, configListProfilesCmd)
	rootCmd.AddCommand(configCmd)
}

// addProfile prompts for a server URL and API key, validates them against
// the server, and saves them under name. If makeCurrent is set, or no
// current profile exists yet, it becomes the active profile.
func addProfile(name string, makeCurrent bool) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Redmine URL (e.g. https://redmine.example.com): ")
	url, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading URL: %w", err)
	}
	url = strings.TrimSpace(url)

	fmt.Print("API key: ")
	keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("reading API key: %w", err)
	}
	apiKey := strings.TrimSpace(string(keyBytes))

	client := redmine.New(url, apiKey)
	user, err := client.Whoami()
	if err != nil {
		return fmt.Errorf("validating credentials: %w", err)
	}
	fmt.Printf("Authenticated as %s %s (%s)\n", user.FirstName, user.LastName, user.Login)

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Profiles[name] = config.Profile{URL: url, APIKey: apiKey}
	if makeCurrent || cfg.CurrentProfile == "" {
		cfg.CurrentProfile = name
	}
	if err := cfg.Save(); err != nil {
		return err
	}

	fmt.Printf("Saved profile %q\n", name)
	return nil
}
