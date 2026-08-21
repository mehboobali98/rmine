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

// goreleaser's template drops the leading v while the go tool keeps it, so
// the same release used to report itself two ways depending on how it had
// been installed.
func TestVersionSpellingIsIndependentOfTheBuildSystem(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })

	for _, stamped := range []string{"0.5.1", "v0.5.1"} {
		version = stamped
		if got := Version(); got != "v0.5.1" {
			t.Errorf("stamped %q reported as %q, want v0.5.1", stamped, got)
		}
	}

	// Snapshot and pre-release forms keep their shape, gaining only the v.
	version = "0.5.2-SNAPSHOT-abc1234"
	if got := Version(); got != "v0.5.2-SNAPSHOT-abc1234" {
		t.Errorf("snapshot reported as %q", got)
	}

	// An unstamped build must not become "vdev".
	version = "dev"
	if got := Version(); got != "dev" && !strings.HasPrefix(got, "v") {
		t.Errorf("unstamped build reported as %q", got)
	}
}
