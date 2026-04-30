package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/4ier/notion-cli/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// setupAuthTest creates a mock Notion API server and sets env vars for isolated testing.
// Returns the server (caller must defer server.Close()) and a cleanup function.
func setupAuthTest(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	// GET /v1/users/me — valid bot response
	mux.HandleFunc("/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer secret_valid_token" && auth != "Bearer secret_work_token" {
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{
				"code":    "unauthorized",
				"message": "API token is invalid.",
			})
			return
		}
		workspace := "Test Workspace"
		if auth == "Bearer secret_work_token" {
			workspace = "Work Workspace"
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "bot-123",
			"name": "Test Bot",
			"bot": map[string]interface{}{
				"workspace_name": workspace,
				"workspace_id":   "ws-123",
				"owner": map[string]interface{}{
					"type":      "workspace",
					"workspace": true,
				},
			},
		})
	})

	// POST /v1/search — for doctor check
	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []interface{}{
				map[string]interface{}{"id": "page-1"},
			},
		})
	})

	server := httptest.NewServer(mux)

	// Point client at mock server
	t.Setenv("NOTION_BASE_URL", server.URL)
	// Use temp config dir
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// Clear any real token
	t.Setenv("NOTION_TOKEN", "")

	return server
}

// executeCommand runs a cobra command with the given args and returns stdout, stderr, and error.
func executeCommand(args ...string) (string, string, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	cmd := rootCmd
	resetCommandFlags(cmd)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func resetCommandFlags(cmd interface{ Flags() *pflag.FlagSet; PersistentFlags() *pflag.FlagSet; Commands() []*cobra.Command }) {
	resetFlagSet(cmd.Flags())
	resetFlagSet(cmd.PersistentFlags())
	for _, sub := range cmd.Commands() {
		resetCommandFlags(sub)
	}
}

func resetFlagSet(fs *pflag.FlagSet) {
	if fs == nil {
		return
	}

	fs.VisitAll(func(f *pflag.Flag) {
		if sliceValue, ok := f.Value.(pflag.SliceValue); ok {
			_ = sliceValue.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	})
}

// --- auth login ---

func TestAuthLoginValidToken(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	// Simulate stdin with token
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.WriteString("secret_valid_token\n")
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	_, _, err := executeCommand("auth", "login", "--with-token")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify config was saved
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config load failed: %v", err)
	}
	profile := cfg.GetCurrentProfile()
	if profile == nil {
		t.Fatal("expected profile to be saved")
	}
	if profile.Token != "secret_valid_token" {
		t.Errorf("expected token 'secret_valid_token', got %q", profile.Token)
	}
	if profile.WorkspaceName != "Test Workspace" {
		t.Errorf("expected workspace 'Test Workspace', got %q", profile.WorkspaceName)
	}
}

func TestAuthLoginInvalidToken(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.WriteString("secret_bad_token\n")
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	_, _, err := executeCommand("auth", "login", "--with-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected 'authentication failed' in error, got: %v", err)
	}
}

func TestAuthLoginEmptyToken(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.WriteString("\n")
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	_, _, err := executeCommand("auth", "login", "--with-token")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
	if !strings.Contains(err.Error(), "no token provided") {
		t.Errorf("expected 'no token provided' in error, got: %v", err)
	}
}

func TestAuthLoginWithProfile(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.WriteString("secret_work_token\n")
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	_, _, err := executeCommand("auth", "login", "--with-token", "--profile", "work")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	cfg, _ := config.Load()
	if cfg.CurrentProfile != "work" {
		t.Errorf("expected current profile 'work', got %q", cfg.CurrentProfile)
	}
	if cfg.Profiles["work"].WorkspaceName != "Work Workspace" {
		t.Errorf("expected 'Work Workspace', got %q", cfg.Profiles["work"].WorkspaceName)
	}
}

// --- auth status ---

func TestAuthStatusAuthenticated(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	// Pre-save a valid config
	cfg := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]*config.Profile{
			"default": {Token: "secret_valid_token", WorkspaceName: "Test Workspace"},
		},
	}
	config.Save(cfg)

	// auth status prints to os.Stdout, not cmd.OutOrStdout() — capture via pipe
	// For now just verify no error
	_, _, err := executeCommand("auth", "status")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAuthStatusNotAuthenticated(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	// Empty config — not authenticated
	_, _, err := executeCommand("auth", "status")
	// status doesn't return error, it prints "Not authenticated"
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// --- auth logout ---

func TestAuthLogoutAll(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	// Pre-save config with two profiles
	cfg := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]*config.Profile{
			"default": {Token: "secret_valid_token"},
			"work":    {Token: "secret_work_token"},
		},
	}
	config.Save(cfg)

	_, _, err := executeCommand("auth", "logout")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify all profiles cleared
	cfg, _ = config.Load()
	if len(cfg.Profiles) != 0 {
		t.Errorf("expected 0 profiles after logout, got %d", len(cfg.Profiles))
	}
}

func TestAuthLogoutSpecificProfile(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	cfg := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]*config.Profile{
			"default": {Token: "secret_valid_token"},
			"work":    {Token: "secret_work_token"},
		},
	}
	config.Save(cfg)

	_, _, err := executeCommand("auth", "logout", "work")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	cfg, _ = config.Load()
	if _, ok := cfg.Profiles["work"]; ok {
		t.Error("expected 'work' profile to be removed")
	}
	if _, ok := cfg.Profiles["default"]; !ok {
		t.Error("expected 'default' profile to remain")
	}
}

func TestAuthLogoutNonexistentProfile(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	cfg := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]*config.Profile{
			"default": {Token: "secret_valid_token"},
		},
	}
	config.Save(cfg)

	_, _, err := executeCommand("auth", "logout", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent profile")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// --- auth switch ---

func TestAuthSwitchDirect(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	cfg := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]*config.Profile{
			"default": {Token: "secret_valid_token", WorkspaceName: "Default WS"},
			"work":    {Token: "secret_work_token", WorkspaceName: "Work WS"},
		},
	}
	config.Save(cfg)

	_, _, err := executeCommand("auth", "switch", "work")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	cfg, _ = config.Load()
	if cfg.CurrentProfile != "work" {
		t.Errorf("expected current profile 'work', got %q", cfg.CurrentProfile)
	}
}

func TestAuthSwitchNonexistent(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	cfg := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]*config.Profile{
			"default": {Token: "secret_valid_token"},
		},
	}
	config.Save(cfg)

	_, _, err := executeCommand("auth", "switch", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent profile")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestAuthSwitchAlreadyCurrent(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	cfg := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]*config.Profile{
			"default": {Token: "secret_valid_token"},
			"work":    {Token: "secret_work_token"},
		},
	}
	config.Save(cfg)

	// Switch to already-current profile should succeed without error
	_, _, err := executeCommand("auth", "switch", "default")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAuthSwitchNotAuthenticated(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	// No config at all
	_, _, err := executeCommand("auth", "switch", "anything")
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

// --- auth doctor ---

func TestAuthDoctorAllGood(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	cfg := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]*config.Profile{
			"default": {Token: "secret_valid_token", WorkspaceName: "Test Workspace"},
		},
	}
	config.Save(cfg)

	_, _, err := executeCommand("auth", "doctor")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAuthDoctorNoToken(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	// No config — doctor should report missing token (but not return error)
	_, _, err := executeCommand("auth", "doctor")
	if err != nil {
		t.Fatalf("expected no error (doctor reports inline), got: %v", err)
	}
}

func TestAuthDoctorInvalidToken(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	cfg := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]*config.Profile{
			"default": {Token: "secret_bad_token"},
		},
	}
	config.Save(cfg)

	_, _, err := executeCommand("auth", "doctor")
	// doctor prints diagnostics inline, doesn't return error
	if err != nil {
		t.Fatalf("expected no error (doctor reports inline), got: %v", err)
	}
}

// --- auth logout current profile fallback ---

func TestAuthLogoutCurrentProfileSwitchesToAnother(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	cfg := &config.Config{
		CurrentProfile: "work",
		Profiles: map[string]*config.Profile{
			"default": {Token: "secret_valid_token"},
			"work":    {Token: "secret_work_token"},
		},
	}
	config.Save(cfg)

	_, _, err := executeCommand("auth", "logout", "work")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	cfg, _ = config.Load()
	// Should have switched to remaining profile
	if cfg.CurrentProfile == "work" {
		t.Error("expected current profile to change from 'work'")
	}
	if len(cfg.Profiles) != 1 {
		t.Errorf("expected 1 remaining profile, got %d", len(cfg.Profiles))
	}
}

func TestAuthLoginMultipleProfiles(t *testing.T) {
	server := setupAuthTest(t)
	defer server.Close()

	// Verify multiple profiles can coexist via config API
	// (executeCommand shares cobra global state across tests, making
	// sequential logins unreliable — test the config layer directly)
	cfg := &config.Config{}
	cfg.SetProfile("default", &config.Profile{
		Token:         "secret_valid_token",
		WorkspaceName: "Test Workspace",
		WorkspaceID:   "ws-123",
		BotID:         "bot-123",
	})
	cfg.SetProfile("work", &config.Profile{
		Token:         "secret_work_token",
		WorkspaceName: "Work Workspace",
		WorkspaceID:   "ws-work",
		BotID:         "bot-work",
	})
	cfg.CurrentProfile = "work"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	cfg, _ = config.Load()
	if len(cfg.Profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(cfg.Profiles))
	}
	if cfg.CurrentProfile != "work" {
		t.Errorf("expected current profile 'work', got %q", cfg.CurrentProfile)
	}
	if cfg.Profiles["default"].WorkspaceName != "Test Workspace" {
		t.Errorf("default workspace wrong: %q", cfg.Profiles["default"].WorkspaceName)
	}
	if cfg.Profiles["work"].WorkspaceName != "Work Workspace" {
		t.Errorf("work workspace wrong: %q", cfg.Profiles["work"].WorkspaceName)
	}
}
