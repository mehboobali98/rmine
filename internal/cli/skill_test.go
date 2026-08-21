package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSkillWritesFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir, err := skillDir(false)
	if err != nil {
		t.Fatalf("skillDir: %v", err)
	}
	path, err := installSkill(dir, false)
	if err != nil {
		t.Fatalf("installSkill: %v", err)
	}

	want := filepath.Join(home, ".claude", "skills", "rmine", "SKILL.md")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading installed skill: %v", err)
	}
	if !strings.Contains(string(got), "# Reference") {
		t.Error("installed file is missing the embedded reference")
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat skill dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o755 {
		t.Errorf("skill dir perm = %o, want 0755", perm)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat skill file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o644 {
		t.Errorf("skill file perm = %o, want 0644", perm)
	}
}

// The file tells its reader to regenerate it after upgrading rmine, which is
// only actionable if it records which rmine wrote it.
func TestInstalledSkillRecordsItsVersion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir, _ := skillDir(false)
	path, err := installSkill(dir, false)
	if err != nil {
		t.Fatalf("installSkill: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading installed skill: %v", err)
	}
	if !strings.Contains(string(got), generatedMarker) {
		t.Errorf("installed skill has no provenance marker:\n%s", firstLines(string(got), 8))
	}
	if !strings.Contains(string(got), Version()) {
		t.Errorf("installed skill does not record the version %q", Version())
	}

	// The stamp must not displace the front matter, which Claude Code parses
	// from the top of the file.
	if !strings.HasPrefix(string(got), "---\nname: rmine\n") {
		t.Errorf("front matter no longer leads the file:\n%s", firstLines(string(got), 8))
	}
}

func TestInstallSkillOverwritesItsOwnFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, _ := skillDir(false)

	if _, err := installSkill(dir, false); err != nil {
		t.Fatalf("first installSkill: %v", err)
	}
	path, err := installSkill(dir, false)
	if err != nil {
		t.Fatalf("second installSkill: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading installed skill: %v", err)
	}
	if string(got) != string(stampedSkill()) {
		t.Error("reinstall did not reproduce the generated file")
	}
}

// "rmine" is a plausible name for a hand-written skill, and destroying one
// is not a reasonable side effect of installing a CLI.
func TestInstallSkillRefusesToClobberAHandWrittenFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, _ := skillDir(false)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, "SKILL.md")
	const handWritten = "---\nname: rmine\n---\n\nMy own notes.\n"
	if err := os.WriteFile(path, []byte(handWritten), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := installSkill(dir, false); err == nil {
		t.Fatal("expected installSkill to refuse to overwrite a hand-written file")
	}
	got, _ := os.ReadFile(path)
	if string(got) != handWritten {
		t.Error("hand-written skill was modified despite the refusal")
	}

	if _, err := installSkill(dir, true); err != nil {
		t.Fatalf("installSkill with force: %v", err)
	}
	got, _ = os.ReadFile(path)
	if !strings.Contains(string(got), generatedMarker) {
		t.Error("--force did not overwrite the file")
	}
}

func TestSkillInstallLocalUsesProjectDirectory(t *testing.T) {
	wd := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(wd); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	dir, err := skillDir(true)
	if err != nil {
		t.Fatalf("skillDir: %v", err)
	}
	// macOS resolves TempDir through a symlink, so compare the suffix.
	if !strings.HasSuffix(dir, filepath.Join(".claude", "skills", "rmine")) {
		t.Errorf("local skill dir = %q, want it under ./.claude/skills/rmine", dir)
	}
	if _, err := installSkill(dir, false); err != nil {
		t.Fatalf("installSkill: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Errorf("local skill not written: %v", err)
	}
}

// The README used to tell people to rm -rf the directory by hand.
func TestSkillUninstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, _ := skillDir(false)

	path, err := installSkill(dir, false)
	if err != nil {
		t.Fatalf("installSkill: %v", err)
	}

	runCLI(t, "skill", "uninstall")

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("skill file still present after uninstall: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("empty skill directory left behind: %v", err)
	}

	// Uninstalling again is not an error, it just has nothing to do.
	out := runCLI(t, "skill", "uninstall")
	if !strings.Contains(out, "No rmine skill installed") {
		t.Errorf("second uninstall said %q", out)
	}
}

func TestSkillUninstallRefusesAHandWrittenFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, _ := skillDir(false)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte("mine, not rmine's\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, _, err := runCLIErr(t, "skill", "uninstall"); err == nil {
		t.Fatal("expected uninstall to refuse a file rmine did not write")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("hand-written file was removed anyway: %v", err)
	}
}

func TestConfirmDefaultAnswer(t *testing.T) {
	cases := []struct {
		input      string
		defaultYes bool
		want       bool
	}{
		{"\n", true, true},
		{"\n", false, false},
		{"n\n", true, false},
		{"y\n", false, true},
		{"yes\n", false, true},
	}

	for _, c := range cases {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe: %v", err)
		}
		if _, err := w.WriteString(c.input); err != nil {
			t.Fatalf("writing fake stdin: %v", err)
		}
		w.Close()

		origStdin := os.Stdin
		os.Stdin = r
		got := confirm("test?", c.defaultYes)
		os.Stdin = origStdin

		if got != c.want {
			t.Errorf("confirm(%q, %v) = %v, want %v", c.input, c.defaultYes, got, c.want)
		}
	}
}

// firstLines trims a file down to something readable in a failure message.
func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
