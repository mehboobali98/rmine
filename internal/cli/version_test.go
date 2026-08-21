package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	out := strings.TrimSpace(runCLI(t, "version"))
	if out == "" {
		t.Fatal("version printed nothing")
	}
	if out != Version() {
		t.Errorf("version = %q, want %q", out, Version())
	}
}

func TestVersionCommandJSON(t *testing.T) {
	out := runCLI(t, "version", "-o", "json")

	var got map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output isn't valid JSON: %v\noutput: %s", err, out)
	}
	if got["version"] != Version() {
		t.Errorf("version = %q, want %q", got["version"], Version())
	}
}

// A released binary must report the tag it was built from, not "dev";
// .goreleaser.yml stamps this variable, so the fallback only applies to
// builds from source.
func TestVersionUsesStampedValue(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })

	version = "v1.2.3"
	if got := Version(); got != "v1.2.3" {
		t.Errorf("Version() = %q, want the stamped v1.2.3", got)
	}
}
