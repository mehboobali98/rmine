// Package cli wires up rmine's cobra command tree.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// An unrecognized format used to fall through to the table branch,
		// so `-o jsno` printed a table and exited 0 — the shape a caller was
		// about to parse, silently not the one it asked for.
		switch outputFlag {
		case "table", "json":
			return nil
		}
		return fmt.Errorf("--output must be table or json, got %q", outputFlag)
	},
}

// invocationArgs is the command line Execute was handed. It exists for the
// one decision that has to be made even when parsing never got far enough to
// set a flag: whether the caller asked for JSON.
var invocationArgs []string

// Execute runs the CLI; it's the sole entrypoint called from main.
func Execute() {
	invocationArgs = os.Args[1:]
	if err := rootCmd.Execute(); err != nil {
		reportError(err)
		os.Exit(1)
	}
}

// errorPayload is what a failure looks like under -o json.
//
// Status and Errors are only set for a rejection that came from Redmine, so
// a caller can tell a 422 it might fix by resending from a transport failure
// it should not resend at all — which matters most on create, where a blind
// retry is how you end up with two tickets.
type errorPayload struct {
	Message string   `json:"message"`
	Status  int      `json:"status,omitempty"`
	Errors  []string `json:"errors,omitempty"`
}

// reportError renders a command failure.
//
// The human sentence always goes to stderr. Under -o json a machine-readable
// object also goes to stdout, because a caller that asked for JSON gets JSON
// for every outcome or it gets none: stdout used to be *empty* on failure,
// which is not a parse error a caller can act on — it is a parse error that
// looks exactly like a crash.
func reportError(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)

	if !wantsJSON() {
		return
	}
	payload := errorPayload{Message: err.Error()}
	var apiErr *redmine.APIError
	if errors.As(err, &apiErr) {
		payload.Status = apiErr.StatusCode
		payload.Errors = apiErr.Errors
	}
	if data, mErr := json.MarshalIndent(map[string]errorPayload{"error": payload}, "", "  "); mErr == nil {
		fmt.Println(string(data))
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
//
// It falls back to the raw arguments because outputFlag is only set once
// cobra has parsed the command line, and the failures most in need of a
// parseable answer happen before that — an unknown flag, an unknown
// subcommand. pflag stops at the first thing it doesn't recognize, so
// `rmine issue list --bogus -o json` never applies the -o. Without this,
// exactly those errors would come back as the empty stdout the JSON error
// payload exists to eliminate.
func wantsJSON() bool {
	if outputFlag == "json" {
		return true
	}
	return jsonInArgs(invocationArgs)
}

func jsonInArgs(args []string) bool {
	for i, arg := range args {
		switch {
		case arg == "--output=json", arg == "-o=json":
			return true
		case arg == "-o", arg == "--output":
			return i+1 < len(args) && args[i+1] == "json"
		case strings.HasPrefix(arg, "-o") && len(arg) > 2 && !strings.HasPrefix(arg, "--"):
			// The attached short form, `-ojson`.
			return arg[2:] == "json"
		}
	}
	return false
}
