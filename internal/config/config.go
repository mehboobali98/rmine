// Package config manages rmine's on-disk configuration: named Redmine server
// profiles (URL + API key) and which one is currently active.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Profile holds the connection details for one Redmine server.
type Profile struct {
	URL    string `yaml:"url"`
	APIKey string `yaml:"api_key"`
}

type Config struct {
	CurrentProfile string             `yaml:"current_profile"`
	Profiles       map[string]Profile `yaml:"profiles"`

	path string
}

// Path returns the config file location, honoring $XDG_CONFIG_HOME and
// falling back to ~/.config/rmine/config.yml.
func Path() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "rmine", "config.yml"), nil
}

// Load reads the config file, returning an empty Config if it doesn't exist yet.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	cfg := &Config{Profiles: map[string]Profile{}, path: path}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	cfg.path = path
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	return cfg, nil
}

// Save writes the config file, creating its directory if needed, with
// permissions restricted to the owner since it contains API keys.
func (c *Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(c.path, data, 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}

// Resolve picks the active profile: an explicit flag value wins, then
// $RMINE_PROFILE, then the config's current_profile.
func (c *Config) Resolve(profileFlag string) (Profile, error) {
	name := profileFlag
	if name == "" {
		name = os.Getenv("RMINE_PROFILE")
	}
	if name == "" {
		name = c.CurrentProfile
	}
	if name == "" {
		return Profile{}, fmt.Errorf("no profile configured — run `rmine config init`")
	}

	p, ok := c.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("no such profile %q", name)
	}
	return p, nil
}
