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
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
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
