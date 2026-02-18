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
