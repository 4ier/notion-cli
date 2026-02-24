package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func setupTestHome(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()

	// Use t.Setenv so env is automatically restored after each test.
	// Set XDG_CONFIG_HOME to a subdir of tmpDir — configDir() checks this first,
	// so we bypass os.UserHomeDir() entirely (avoids caching issues on CI).
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))
	t.Setenv("HOME", tmpDir)
}

func TestSaveAndLoad(t *testing.T) {
	setupTestHome(t)

	cfg := &Config{
		Token:         "test-token-value",
		WorkspaceName: "Test Workspace",
		WorkspaceID:   "ws-123",
		BotID:         "bot-456",
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Token != cfg.Token {
		t.Errorf("Token = %q, want %q", loaded.Token, cfg.Token)
	}
	if loaded.WorkspaceName != cfg.WorkspaceName {
		t.Errorf("WorkspaceName = %q, want %q", loaded.WorkspaceName, cfg.WorkspaceName)
	}
	if loaded.WorkspaceID != cfg.WorkspaceID {
		t.Errorf("WorkspaceID = %q, want %q", loaded.WorkspaceID, cfg.WorkspaceID)
	}
	if loaded.BotID != cfg.BotID {
		t.Errorf("BotID = %q, want %q", loaded.BotID, cfg.BotID)
	}
}

func TestLoadMissing(t *testing.T) {
	setupTestHome(t)

	_, err := Load()
	if err == nil {
		t.Error("Load() should error when config file doesn't exist")
	}
}

func TestConfigFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions not applicable on Windows")
	}
	setupTestHome(t)

	cfg := &Config{Token: "secret-token"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	configFile := configPath()
	info, err := os.Stat(configFile)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	perm := info.Mode().Perm()
	if perm&0077 != 0 {
		t.Errorf("Config file permissions = %o, want no group/other access", perm)
	}
}

func TestProfileSupport(t *testing.T) {
	setupTestHome(t)

	cfg := &Config{}

	// Add profiles
	cfg.SetProfile("default", &Profile{
		Token:         "default-token",
		WorkspaceName: "Default Workspace",
	})
	cfg.SetProfile("work", &Profile{
		Token:         "work-token",
		WorkspaceName: "Work Workspace",
	})
	cfg.CurrentProfile = "default"

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check profiles exist
	if len(loaded.Profiles) != 2 {
		t.Errorf("got %d profiles, want 2", len(loaded.Profiles))
	}

	// Check current profile
	profile := loaded.GetCurrentProfile()
	if profile == nil {
		t.Fatal("GetCurrentProfile() returned nil")
	}
	if profile.Token != "default-token" {
		t.Errorf("Token = %q, want %q", profile.Token, "default-token")
	}

	// Switch profile
	loaded.CurrentProfile = "work"
	profile = loaded.GetCurrentProfile()
	if profile.Token != "work-token" {
		t.Errorf("Token = %q, want %q", profile.Token, "work-token")
	}
}

func TestMigrateToProfiles(t *testing.T) {
	setupTestHome(t)

	// Create legacy config
	cfg := &Config{
		Token:         "legacy-token",
		WorkspaceName: "Legacy Workspace",
		WorkspaceID:   "ws-legacy",
		BotID:         "bot-legacy",
	}

	cfg.MigrateToProfiles()

	// Check migration
	if len(cfg.Profiles) != 1 {
		t.Errorf("got %d profiles, want 1", len(cfg.Profiles))
	}
	if cfg.CurrentProfile != "default" {
		t.Errorf("CurrentProfile = %q, want %q", cfg.CurrentProfile, "default")
	}

	profile := cfg.GetCurrentProfile()
	if profile == nil {
		t.Fatal("GetCurrentProfile() returned nil")
	}
	if profile.Token != "legacy-token" {
		t.Errorf("Token = %q, want %q", profile.Token, "legacy-token")
	}

	// Legacy fields should be cleared
	if cfg.Token != "" {
		t.Errorf("legacy Token should be cleared, got %q", cfg.Token)
	}
}

func TestListProfiles(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]*Profile{
			"work":     {Token: "t1"},
			"personal": {Token: "t2"},
			"default":  {Token: "t3"},
		},
	}

	profiles := cfg.ListProfiles()
	if len(profiles) != 3 {
		t.Errorf("got %d profiles, want 3", len(profiles))
	}

	// Should be sorted alphabetically
	if profiles[0] != "default" {
		t.Errorf("profiles[0] = %q, want %q", profiles[0], "default")
	}
	if profiles[1] != "personal" {
		t.Errorf("profiles[1] = %q, want %q", profiles[1], "personal")
	}
	if profiles[2] != "work" {
		t.Errorf("profiles[2] = %q, want %q", profiles[2], "work")
	}
}

func TestGetCurrentProfileLegacyFallback(t *testing.T) {
	// Test that GetCurrentProfile falls back to legacy fields
	cfg := &Config{
		Token:         "legacy-token",
		WorkspaceName: "Legacy",
	}

	profile := cfg.GetCurrentProfile()
	if profile == nil {
		t.Fatal("GetCurrentProfile() returned nil")
	}
	if profile.Token != "legacy-token" {
		t.Errorf("Token = %q, want %q", profile.Token, "legacy-token")
	}
}
