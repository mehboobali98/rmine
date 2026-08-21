package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"
)

// printTable renders rows as an aligned, human-readable table.
func printTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		cells := make([]string, len(row))
		for i, cell := range row {
			cells[i] = flattenCell(cell)
		}
		fmt.Fprintln(w, strings.Join(cells, "\t"))
	}
}

// flattenCell collapses a value onto a single line. Issue subjects and time
// entry comments routinely contain newlines, and one raw newline — or a tab,
// which tabwriter reads as a column break — misaligns every row after it.
func flattenCell(s string) string {
	if !strings.ContainsAny(s, "\n\r\t") {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		}
		return r
	}, s)
}

// printJSON renders v as indented JSON for scripting use.
//
// A nil slice is normalized to [] rather than null: "no results" is the
// common case for every list command, and callers — `jq '.[]'`, `jq length`,
// anything ranging over the result — fail on null but handle an empty array
// without special-casing it.
func printJSON(v any) error {
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Slice && rv.IsNil() {
		v = []any{}
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// actionResult is the JSON shape every mutating command emits under -o json.
// One flat, predictable object beats each command inventing its own: a caller
// can always read .status, and whichever id field is set names the thing that
// was acted on.
type actionResult struct {
	Status    string `json:"status"`
	Issue     int    `json:"issue,omitempty"`
	TimeEntry int    `json:"time_entry,omitempty"`
	Profile   string `json:"profile,omitempty"`
	Path      string `json:"path,omitempty"`
}

// printAction reports a mutating command's outcome: a human sentence by
// default, the same outcome as JSON under -o json. Both the README and the
// skill file promise that every command accepts -o json, but the write
// commands used to print prose either way, so a caller that passed the flag
// uniformly got unparseable output back from half the CLI.
func printAction(human string, result actionResult) error {
	if wantsJSON() {
		return printJSON(result)
	}
	fmt.Println(human)
	return nil
}

// promptf writes an interactive prompt or progress note to stderr. These are
// not results: routing them to stdout would interleave them with -o json
// output and break the parse.
func promptf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
}
