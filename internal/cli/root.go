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
//
// This applies only where a project is mandatory, never to a filter. Letting
// a stored default narrow `issue list` would mean the same command answered
// differently depending on configuration the user cannot see in the command
// they typed — and a quietly narrowed search looks exactly like a project
// with less work in it.
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

// wantsJSON reports whether -o/--output json was requested.
func wantsJSON() bool {
	return outputFlag == "json"
}
