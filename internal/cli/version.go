package cli

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is stamped at release time with -ldflags (see .goreleaser.yml).
var version = "dev"

// Version reports the running rmine's version. A build that was not stamped
// falls back to the module version the go tool recorded, which is what
// `go install ...@latest` produces — so an installed binary can still say
// what it is.
func Version() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the rmine version",
	RunE: func(cmd *cobra.Command, args []string) error {
		if wantsJSON() {
			return printJSON(map[string]string{"version": Version()})
		}
		fmt.Println(Version())
		return nil
	},
}

func init() {
	rootCmd.Version = Version() // also gives the root command --version
	rootCmd.AddCommand(versionCmd)
}
