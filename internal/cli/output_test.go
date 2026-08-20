package cli

import (
	"io"
	"os"
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
