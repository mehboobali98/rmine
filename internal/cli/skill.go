package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed SKILL.md
var skillMD []byte

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage the rmine Claude Code skill",
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install rmine's reference as a Claude Code skill",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := installSkill()
		if err != nil {
			return err
		}
		fmt.Printf("Installed Claude Code skill to %s\n", path)
		return nil
	},
}

func init() {
	skillCmd.AddCommand(skillInstallCmd)
	rootCmd.AddCommand(skillCmd)
}

// installSkill writes the embedded SKILL.md to Claude Code's global,
// user-level skill directory, overwriting unconditionally — the file is
// fully generated and never meant to be hand-edited, so there's no state
// an overwrite could destroy.
func installSkill() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	dir := filepath.Join(home, ".claude", "skills", "rmine")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating skill directory: %w", err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, skillMD, 0o644); err != nil {
		return "", fmt.Errorf("writing skill file: %w", err)
	}
	return path, nil
}

// promptInstallSkill asks (default yes) whether to install the Claude Code
// skill, once, at the end of `config init`.
func promptInstallSkill() error {
	if !confirm("Install the rmine Claude Code skill (teaches AI assistants how to drive rmine)?", true) {
		return nil
	}
	path, err := installSkill()
	if err != nil {
		return err
	}
	fmt.Printf("Installed Claude Code skill to %s\n", path)
	return nil
}
