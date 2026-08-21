package cli

import (
	"bufio"
	"fmt"
	neturl "net/url"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/mehboobali98/rmine/internal/config"
	"github.com/mehboobali98/rmine/internal/redmine"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage rmine's server profiles",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up the default profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := addProfile("default", true); err != nil {
			return err
		}
		return promptInstallSkill()
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
		return printAction(fmt.Sprintf("Switched to profile %q", name), actionResult{Status: "switched", Profile: name})
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
		// Sorted, because ranging a map would order the output differently
		// on every run — noisy for a human and unusable for a diff.
		names := make([]string, 0, len(cfg.Profiles))
		for name := range cfg.Profiles {
			names = append(names, name)
		}
		sort.Strings(names)

		if wantsJSON() {
			list := make([]profileInfo, 0, len(names))
			for _, name := range names {
				list = append(list, profileInfo{
					Name:    name,
					URL:     cfg.Profiles[name].URL,
					Current: name == cfg.CurrentProfile,
				})
			}
			return printJSON(list)
		}

		if len(names) == 0 {
			fmt.Println("No profiles configured yet — run `rmine config init`.")
			return nil
		}

		rows := make([][]string, 0, len(names))
		for _, name := range names {
			current := ""
			if name == cfg.CurrentProfile {
				current = "*"
			}
			rows = append(rows, []string{current, name, cfg.Profiles[name].URL})
		}
		printTable([]string{"", "NAME", "URL"}, rows)
		return nil
	},
}

// readSecret reads a value without echoing it back when stdin is a terminal.
//
// When stdin is not a terminal there is nothing to hide, and reading the raw
// file descriptor would be wrong as well as pointless: the buffered reader
// above may already hold the line, so ReadPassword would block waiting for
// input that has in fact already arrived. Read from the same reader instead.
func readSecret(reader *bufio.Reader, prompt string) (string, error) {
	promptf("%s", prompt)

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		promptf("\n")
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}

	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// normalizeServerURL cleans up a hand-typed server URL. A bare hostname is
// the common slip, and left alone it surfaces much later as an
// "unsupported protocol scheme" error from deep inside the HTTP client.
func normalizeServerURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("a Redmine URL is required (set $RMINE_URL to supply one non-interactively)")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := neturl.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid Redmine URL %q: %w", raw, err)
	}
	switch u.Scheme {
	case "https":
	case "http":
		promptf("Warning: %s is plain HTTP — your API key will cross the network unencrypted.\n", raw)
	default:
		return "", fmt.Errorf("Redmine URL must use http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid Redmine URL %q: no host", raw)
	}
	return strings.TrimRight(raw, "/"), nil
}

// profileInfo is one configured profile as `config list-profiles -o json`
// reports it. The API key is deliberately not included: listing profiles is a
// routine call, and printing a secret makes it easy to leak into a log.
type profileInfo struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Current bool   `json:"current"`
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

	// $RMINE_URL and $RMINE_API_KEY let a container or CI job configure rmine
	// without a terminal to prompt at. The key is read from the environment
	// rather than a flag on purpose: a flag would put the secret in the shell
	// history and in the process list.
	rawURL := os.Getenv("RMINE_URL")
	if rawURL == "" {
		promptf("Redmine URL (e.g. https://redmine.example.com): ")
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return fmt.Errorf("reading URL: %w", err)
		}
		rawURL = line
	}
	url, err := normalizeServerURL(rawURL)
	if err != nil {
		return err
	}

	apiKey := os.Getenv("RMINE_API_KEY")
	if apiKey == "" {
		apiKey, err = readSecret(reader, "API key: ")
		if err != nil {
			return fmt.Errorf("reading API key: %w", err)
		}
	}
	if apiKey == "" {
		return fmt.Errorf("an API key is required (set $RMINE_API_KEY to supply one non-interactively)")
	}

	client := redmine.New(url, apiKey)
	user, err := client.Whoami()
	if err != nil {
		return fmt.Errorf("validating credentials: %w", err)
	}
	promptf("Authenticated as %s %s (%s)\n", user.FirstName, user.LastName, user.Login)

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

	return printAction(fmt.Sprintf("Saved profile %q", name), actionResult{Status: "saved", Profile: name})
}
