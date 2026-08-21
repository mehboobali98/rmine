// Package cli wires up rmine's cobra command tree.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mehboobali98/rmine/internal/config"
	"github.com/mehboobali98/rmine/internal/redmine"
)

var (
	profileFlag string
	outputFlag  string
)

var rootCmd = &cobra.Command{
	Use:           "rmine",
	Short:         "A command-line client for Redmine",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI; it's the sole entrypoint called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&profileFlag, "profile", "", "profile to use (overrides $RMINE_PROFILE and the configured default)")
	rootCmd.PersistentFlags().StringVarP(&outputFlag, "output", "o", "table", "output format: table|json")
}

// activeProfile resolves the profile this invocation should use.
func activeProfile() (config.Profile, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Profile{}, err
	}
	return cfg.Resolve(profileFlag)
}

// newClient loads the config, resolves the active profile, and returns a
// ready-to-use Redmine client.
func newClient() (*redmine.Client, error) {
	profile, err := activeProfile()
	if err != nil {
		return nil, err
	}
	return redmine.New(profile.URL, profile.APIKey), nil
}

// projectOrDefault supplies the active profile's default project when a
// command that requires one was not given it.
func projectOrDefault(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	profile, err := activeProfile()
	if err != nil {
		return "", err
	}
	return profile.DefaultProject, nil
}

// projectFilterOrDefault does the same for a listing command, where
// --all-projects opts back out.
//
// Narrowing a search is a bigger deal than filling in a required field: the
// scoping comes from stored configuration that does not appear in the command
// the user typed, and a filtered result looks much like a quiet week. So when
// the default is what took effect, say so — on stderr, where it reaches a
// person without disturbing stdout for anything parsing it.
//
// explicitScope reports that the caller already pinned the result set some
// other way — `time list --issue 1234` names one issue, and that issue is in
// whatever project it is in. Applying the default on top would ask for
// entries on that issue *and* in an unrelated project, which matches nothing
// at all, and an empty list is a valid-looking answer.
func projectFilterOrDefault(flagValue string, allProjects, explicitScope bool) (string, error) {
	if flagValue != "" && allProjects {
		return "", fmt.Errorf("--project and --all-projects contradict each other")
	}
	if flagValue != "" || allProjects || explicitScope {
		return flagValue, nil
	}

	profile, err := activeProfile()
	if err != nil {
		return "", err
	}
	if profile.DefaultProject == "" {
		return "", nil
	}
	promptf("Scoped to the profile's default project %q — pass --all-projects to search all of them.\n", profile.DefaultProject)
	return profile.DefaultProject, nil
}

// wantsJSON reports whether -o/--output json was requested.
func wantsJSON() bool {
	return outputFlag == "json"
}
