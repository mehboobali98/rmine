package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// pflag takes the first backquoted span of a flag's usage as the name of its
// value in --help, so a quoted command there renders as the flag's type.
func TestFlagValueNamesAreSingleWords(t *testing.T) {
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if name, _ := pflag.UnquoteUsage(f); strings.ContainsAny(name, " <>") {
				t.Errorf("%s --%s: help shows its value as %q", cmd.CommandPath(), f.Name, name)
			}
		})
		for _, sub := range cmd.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
}
