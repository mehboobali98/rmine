package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// docFiles are the two references that must describe the CLI: the README for
// people, and SKILL.md for the agent that `rmine skill install` hands it to.
var docFiles = map[string]string{
	"README.md": filepath.Join("..", "..", "README.md"),
	"SKILL.md":  "SKILL.md",
}

// cobra generates these itself; they are not part of rmine's surface. The
// completion command in particular is added lazily during Execute(), so its
// flags only become visible once some other test has run a command — a
// subtlety that made this check order-dependent until it was excluded here
// rather than only in leafCommands.
var generatedCommands = map[string]bool{
	"completion": true,
	"help":       true,
}

var generatedFlags = map[string]bool{
	"help":    true,
	"version": true,
}

// TestDocsCoverEveryFlag is the check that was missing when --parent was
// added: it landed in SKILL.md and never reached the README, and nothing
// noticed. Adding a flag now means documenting it in both places.
func TestDocsCoverEveryFlag(t *testing.T) {
	docs := readDocs(t)

	for _, flag := range registeredFlags(rootCmd) {
		for name, text := range docs {
			if !strings.Contains(text, "--"+flag) {
				t.Errorf("%s does not document --%s", name, flag)
			}
		}
	}
}

// TestDocsCoverEveryCommand does the same for the command tree itself.
func TestDocsCoverEveryCommand(t *testing.T) {
	docs := readDocs(t)

	for _, cmd := range leafCommands(rootCmd) {
		for name, text := range docs {
			if !strings.Contains(text, cmd) {
				t.Errorf("%s does not mention `%s`", name, cmd)
			}
		}
	}
}

func readDocs(t *testing.T) map[string]string {
	t.Helper()
	docs := make(map[string]string, len(docFiles))
	for name, path := range docFiles {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		// Collapse whitespace so a command name that happens to be split
		// across two wrapped lines still counts as documented.
		docs[name] = strings.Join(strings.Fields(string(b)), " ")
	}
	return docs
}

// registeredFlags returns every distinct flag name in the command tree.
func registeredFlags(root *cobra.Command) []string {
	seen := map[string]bool{}
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		collect := func(fs *pflag.FlagSet) {
			fs.VisitAll(func(f *pflag.Flag) {
				if !generatedFlags[f.Name] {
					seen[f.Name] = true
				}
			})
		}
		collect(cmd.Flags())
		collect(cmd.PersistentFlags())
		for _, sub := range cmd.Commands() {
			if generatedCommands[sub.Name()] {
				continue
			}
			walk(sub)
		}
	}
	walk(root)

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// leafCommands returns the runnable commands as a user types them, e.g.
// "rmine issue attachments". Groups like "rmine issue" are skipped: they do
// nothing on their own.
func leafCommands(root *cobra.Command) []string {
	var out []string
	var walk func(*cobra.Command, string)
	walk = func(cmd *cobra.Command, prefix string) {
		name := strings.Fields(cmd.Use)[0]
		path := strings.TrimSpace(prefix + " " + name)

		runnable := 0
		for _, sub := range cmd.Commands() {
			if generatedCommands[sub.Name()] {
				continue
			}
			runnable++
			walk(sub, path)
		}
		if runnable == 0 && cmd != root {
			out = append(out, path)
		}
	}
	walk(root, "")
	sort.Strings(out)
	return out
}
