package cli

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// version is stamped at release time with -ldflags (see .goreleaser.yml).
var version = "dev"

// Version reports the running rmine's version. A build that was not stamped
// falls back to the module version the go tool recorded, which is what
// `go install ...@latest` produces — so an installed binary can still say
// what it is.
//
// The two sources disagree on spelling: the go tool records the tag verbatim
// ("v0.5.1") while goreleaser's template drops the leading v ("0.5.1"), so
// the same release reported itself two different ways depending on how it had
// been installed. Normalizing here rather than in .goreleaser.yml keeps the
// two in step whatever a future build system passes in.
func Version() string {
	v := version
	if v == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			if mod := bi.Main.Version; mod != "" && mod != "(devel)" {
				v = mod
			}
		}
	}
	if v == "dev" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
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
