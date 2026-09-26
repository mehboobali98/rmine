package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mehboobali98/rmine/internal/redmine"
)

var dryRunFlag bool

// finishDryRun turns the error a dry-run client stops a command with into
// the report of what would have been sent, and a successful exit.
func finishDryRun(err error) error {
	var dry *redmine.DryRunError
	if !errors.As(err, &dry) {
		return err
	}
	if wantsJSON() {
		return printJSON(map[string]any{"dry_run": true, "requests": dry.Planned})
	}
	fmt.Println("Dry run: nothing was sent. rmine would have made these requests:")
	for _, p := range dry.Planned {
		fmt.Printf("\n%s %s\n", p.Method, p.Path)
		if p.File != "" {
			fmt.Printf("file: %s\n", p.File)
		}
		if len(p.Body) > 0 {
			fmt.Println(indentJSON(p.Body))
		}
	}
	return nil
}

func indentJSON(raw json.RawMessage) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw)
	}
	return buf.String()
}

func addDryRunFlag(cmds ...*cobra.Command) {
	for _, c := range cmds {
		c.Flags().BoolVar(&dryRunFlag, "dry-run", false, "print the requests this would send, without sending them")
	}
}

func init() {
	addDryRunFlag(
		issueCreateCmd, issueUpdateCmd, issueCloseCmd, issueCommentCmd,
		issueRelateCmd, issueUnrelateCmd,
		timeLogCmd, timeEditCmd, timeDeleteCmd,
	)
}
