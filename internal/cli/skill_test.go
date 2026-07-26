package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallSkillWritesFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := installSkill()
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
	if string(got) != string(skillMD) {
		t.Errorf("installed content doesn't match the embedded SKILL.md")
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

func TestInstallSkillOverwritesCleanly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := installSkill(); err != nil {
		t.Fatalf("first installSkill: %v", err)
	}
	path, err := installSkill()
	if err != nil {
		t.Fatalf("second installSkill: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading installed skill: %v", err)
	}
	if string(got) != string(skillMD) {
		t.Errorf("installed content doesn't match the embedded SKILL.md after reinstall")
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
