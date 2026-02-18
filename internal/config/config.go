package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Token         string `json:"token"`
	WorkspaceName string `json:"workspace_name,omitempty"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	BotID         string `json:"bot_id,omitempty"`
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
