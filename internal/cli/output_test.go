package cli

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/mehboobali98/rmine/internal/redmine"
)

// captureStdout runs fn with os.Stdout redirected and returns what it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	done := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- b
	}()

	orig := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = orig
	w.Close()

	return string(<-done)
}

func TestPrintJSONRendersNilSliceAsEmptyArray(t *testing.T) {
	var issues []redmine.Issue // what a list call returns with no matches

	out := captureStdout(t, func() {
		if err := printJSON(issues); err != nil {
			t.Errorf("printJSON: %v", err)
		}
	})

	if out != "[]\n" {
		t.Errorf("printJSON(nil slice) = %q, want %q", out, "[]\n")
	}
}

func TestPrintJSONLeavesNonSlicesAlone(t *testing.T) {
	out := captureStdout(t, func() {
		if err := printJSON(map[string]int{"id": 1}); err != nil {
			t.Errorf("printJSON: %v", err)
		}
	})

	if want := "{\n  \"id\": 1\n}\n"; out != want {
		t.Errorf("printJSON(map) = %q, want %q", out, want)
	}
}

func TestPrintTableFlattensMultiLineCells(t *testing.T) {
	out := captureStdout(t, func() {
		printTable(
			[]string{"ID", "COMMENT"},
			[][]string{
				{"1", "first line\nsecond line"},
				{"2", "has\ta tab"},
				{"3", "windows\r\nnewline"},
			},
		)
	})

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4 (header + 3 rows):\n%s", len(lines), out)
	}
	for _, want := range []string{"first line second line", "has a tab", "windows newline"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing flattened cell %q:\n%s", want, out)
		}
	}
}

func TestFlattenCellLeavesPlainTextUntouched(t *testing.T) {
	const plain = "Fix the login redirect"
	if got := flattenCell(plain); got != plain {
		t.Errorf("flattenCell(%q) = %q, want it unchanged", plain, got)
	}
}
