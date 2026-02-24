package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// Profile represents a single workspace authentication profile.
type Profile struct {
	Token         string `json:"token"`
	WorkspaceName string `json:"workspace_name,omitempty"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	BotID         string `json:"bot_id,omitempty"`
}

// Config holds the CLI configuration with support for multiple profiles.
type Config struct {
	// CurrentProfile is the name of the active profile
	CurrentProfile string `json:"current_profile,omitempty"`
	// Profiles maps profile names to their configuration
	Profiles map[string]*Profile `json:"profiles,omitempty"`

	// Legacy fields for backward compatibility
	Token         string `json:"token,omitempty"`
	WorkspaceName string `json:"workspace_name,omitempty"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	BotID         string `json:"bot_id,omitempty"`
}

// GetCurrentProfile returns the current profile configuration.
// It handles migration from legacy single-token format.
func (c *Config) GetCurrentProfile() *Profile {
	// If we have profiles, use the current one
	if c.Profiles != nil && len(c.Profiles) > 0 {
		profileName := c.CurrentProfile
		if profileName == "" {
			profileName = "default"
		}
		if p, ok := c.Profiles[profileName]; ok {
			return p
		}
	}

	// Fall back to legacy format
	if c.Token != "" {
		return &Profile{
			Token:         c.Token,
			WorkspaceName: c.WorkspaceName,
			WorkspaceID:   c.WorkspaceID,
			BotID:         c.BotID,
		}
	}

	return nil
}

// SetProfile sets or updates a profile in the config.
func (c *Config) SetProfile(name string, profile *Profile) {
	if c.Profiles == nil {
		c.Profiles = make(map[string]*Profile)
	}
	c.Profiles[name] = profile
}

// ListProfiles returns a sorted list of profile names.
func (c *Config) ListProfiles() []string {
	if c.Profiles == nil || len(c.Profiles) == 0 {
		// If using legacy format with a token, return "default"
		if c.Token != "" {
			return []string{"default"}
		}
		return nil
	}

	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// MigrateToProfiles migrates legacy single-token config to profiles format.
func (c *Config) MigrateToProfiles() {
	if c.Token != "" && (c.Profiles == nil || len(c.Profiles) == 0) {
		c.Profiles = map[string]*Profile{
			"default": {
				Token:         c.Token,
				WorkspaceName: c.WorkspaceName,
				WorkspaceID:   c.WorkspaceID,
				BotID:         c.BotID,
			},
		}
		c.CurrentProfile = "default"
		// Clear legacy fields
		c.Token = ""
		c.WorkspaceName = ""
		c.WorkspaceID = ""
		c.BotID = ""
	}
}

func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "notion-cli")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "notion-cli")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

func Load() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return &Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &Config{}, err
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0600)
}
