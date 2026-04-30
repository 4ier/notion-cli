package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/4ier/notion-cli/internal/client"
	"github.com/4ier/notion-cli/internal/config"
	"github.com/4ier/notion-cli/internal/render"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Notion",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to Notion",
	Long: `Authenticate with Notion using an integration token.

Use --profile to save credentials under a named profile for multi-workspace support.

Examples:
  notion auth login
  notion auth login --with-token
  notion auth login --profile work
  echo "secret_xxx" | notion auth login --with-token --profile personal`,
	RunE: func(cmd *cobra.Command, args []string) error {
		withToken, _ := cmd.Flags().GetBool("with-token")
		profileName, _ := cmd.Flags().GetString("profile")

		if profileName == "" {
			profileName = "default"
		}

		var token string
		if withToken {
			// Read from stdin
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				token = strings.TrimSpace(scanner.Text())
			}
		} else {
			// Interactive prompt
			fmt.Print("Paste your integration token: ")
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				token = strings.TrimSpace(scanner.Text())
			}
		}

		if token == "" {
			return fmt.Errorf("no token provided")
		}

		// Validate token by calling the API
		c := client.New(token)
		me, err := c.GetMe()
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// Extract workspace info
		botInfo, _ := me["bot"].(map[string]interface{})
		workspaceName, _ := botInfo["workspace_name"].(string)
		workspaceID, _ := botInfo["workspace_id"].(string)
		botID, _ := me["id"].(string)

		// Load existing config or create new
		cfg, _ := config.Load()
		if cfg == nil {
			cfg = &config.Config{}
		}

		// Migrate legacy config if needed
		cfg.MigrateToProfiles()

		// Set the profile
		cfg.SetProfile(profileName, &config.Profile{
			Token:         token,
			WorkspaceName: workspaceName,
			WorkspaceID:   workspaceID,
			BotID:         botID,
		})

		// Set as current profile
		cfg.CurrentProfile = profileName

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		render.Title("✓", fmt.Sprintf("Logged in to %s", workspaceName))
		if profileName != "default" {
			render.Field("Profile", profileName)
		}
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	Long: `Show current authentication status and profile information.

Examples:
  notion auth status`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			fmt.Println("✗ Not authenticated")
			return nil
		}

		profile := cfg.GetCurrentProfile()
		if profile == nil || profile.Token == "" {
			fmt.Println("✗ Not authenticated")
			return nil
		}

		c := client.New(profile.Token)
		me, err := c.GetMe()
		if err != nil {
			return fmt.Errorf("token is invalid: %w", err)
		}

		botInfo, _ := me["bot"].(map[string]interface{})
		workspaceName, _ := botInfo["workspace_name"].(string)
		name, _ := me["name"].(string)
		integrationType := detectIntegrationType(botInfo)

		render.Title("✓", "Authenticated")

		// Show current profile name
		profileName := cfg.CurrentProfile
		if profileName == "" {
			profileName = "default"
		}
		profiles := cfg.ListProfiles()
		if len(profiles) > 1 {
			render.Field("Profile", profileName)
		}

		render.Field("Workspace", workspaceName)
		render.Field("Bot", name)
		if integrationType != "" {
			render.Field("Integration", integrationType)
			if integrationType == "internal" {
				render.Field("Root page", "not allowed (share a parent page)")
			}
		}

		// Show other available profiles
		if len(profiles) > 1 {
			var others []string
			for _, p := range profiles {
				if p != profileName {
					others = append(others, p)
				}
			}
			if len(others) > 0 {
				render.Field("Other profiles", strings.Join(others, ", "))
			}
		}
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout [profile]",
	Short: "Log out of Notion",
	Long: `Log out of Notion. Optionally specify a profile to remove.

Without arguments, clears all profiles. With a profile name, removes only that profile.

Examples:
  notion auth logout
  notion auth logout work`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			// Clear all profiles
			cfg := &config.Config{}
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Println("✓ Logged out (all profiles)")
			return nil
		}

		// Remove specific profile
		profileName := args[0]
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if cfg.Profiles == nil {
			return fmt.Errorf("profile %q not found", profileName)
		}

		if _, ok := cfg.Profiles[profileName]; !ok {
			return fmt.Errorf("profile %q not found", profileName)
		}

		delete(cfg.Profiles, profileName)

		// If we removed the current profile, switch to another
		if cfg.CurrentProfile == profileName {
			profiles := cfg.ListProfiles()
			if len(profiles) > 0 {
				cfg.CurrentProfile = profiles[0]
			} else {
				cfg.CurrentProfile = ""
			}
		}

		if err := config.Save(cfg); err != nil {
			return err
		}

		fmt.Printf("✓ Logged out from profile %q\n", profileName)
		return nil
	},
}

var authSwitchCmd = &cobra.Command{
	Use:   "switch [profile]",
	Short: "Switch between workspace profiles",
	Long: `Switch between saved workspace profiles.

Without arguments, shows an interactive list of profiles to choose from.
With a profile name, switches directly to that profile.

Examples:
  notion auth switch
  notion auth switch work
  notion auth switch personal`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("not authenticated. Run 'notion auth login' first")
		}

		// Migrate legacy config if needed
		cfg.MigrateToProfiles()

		profiles := cfg.ListProfiles()
		if len(profiles) == 0 {
			return fmt.Errorf("no profiles found. Run 'notion auth login' first")
		}

		var targetProfile string

		if len(args) == 1 {
			// Direct switch
			targetProfile = args[0]
			if _, ok := cfg.Profiles[targetProfile]; !ok {
				return fmt.Errorf("profile %q not found. Available: %s", targetProfile, strings.Join(profiles, ", "))
			}
		} else {
			// Interactive selection
			if len(profiles) == 1 {
				fmt.Println("Only one profile available.")
				return nil
			}

			fmt.Println("Available profiles:")
			fmt.Println()
			for i, name := range profiles {
				profile := cfg.Profiles[name]
				marker := "  "
				if name == cfg.CurrentProfile {
					marker = "→ "
				}
				workspace := profile.WorkspaceName
				if workspace == "" {
					workspace = "(unknown workspace)"
				}
				fmt.Printf("%s%d. %s (%s)\n", marker, i+1, name, workspace)
			}
			fmt.Println()
			fmt.Print("Select profile (number or name): ")

			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				return nil
			}
			input := strings.TrimSpace(scanner.Text())

			if input == "" {
				return nil
			}

			// Try as number first
			if num, err := strconv.Atoi(input); err == nil {
				if num >= 1 && num <= len(profiles) {
					targetProfile = profiles[num-1]
				} else {
					return fmt.Errorf("invalid selection: %d", num)
				}
			} else {
				// Try as profile name
				targetProfile = input
				if _, ok := cfg.Profiles[targetProfile]; !ok {
					return fmt.Errorf("profile %q not found", targetProfile)
				}
			}
		}

		if targetProfile == cfg.CurrentProfile {
			fmt.Printf("Already using profile %q\n", targetProfile)
			return nil
		}

		cfg.CurrentProfile = targetProfile
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		profile := cfg.Profiles[targetProfile]
		workspace := profile.WorkspaceName
		if workspace == "" {
			workspace = "(unknown)"
		}

		render.Title("✓", fmt.Sprintf("Switched to profile %q", targetProfile))
		render.Field("Workspace", workspace)
		return nil
	},
}

var authDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check authentication and API connectivity",
	Long: `Run health checks on your Notion CLI setup.

Validates:
  - Config file exists and has a token
  - Token is valid (API responds)
  - Workspace is accessible
  - Can list databases

Examples:
  notion auth doctor`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Notion CLI Health Check")
		fmt.Println()

		// Check 1: Config file
		cfg, err := config.Load()
		profile := cfg.GetCurrentProfile()
		token := cfg.Token
		if token == "" && profile != nil {
			token = profile.Token
		}
		if err != nil || token == "" {
			fmt.Println("  ✗ Config: no token found")
			fmt.Println("    Run: notion auth login --with-token")
			return nil
		}
		fmt.Println("  ✓ Config: token found")

		// Check 2: Token validity
		c := client.New(token)
		me, err := c.GetMe()
		if err != nil {
			fmt.Printf("  ✗ Auth: token is invalid (%v)\n", err)
			return nil
		}

		name, _ := me["name"].(string)
		botInfo, _ := me["bot"].(map[string]interface{})
		workspace, _ := botInfo["workspace_name"].(string)
		integrationType := detectIntegrationType(botInfo)
		fmt.Printf("  ✓ Auth: %s\n", name)
		fmt.Printf("  ✓ Workspace: %s\n", workspace)
		if integrationType != "" {
			fmt.Printf("  ✓ Integration: %s\n", integrationType)
			if integrationType == "internal" {
				fmt.Println("    note: internal integrations cannot create pages at the workspace root;")
				fmt.Println("          create a parent page in Notion and share it with this integration first.")
			}
		}

		// Check 3: Can search
		result, err := c.Search("", "", 1, "")
		if err != nil {
			fmt.Printf("  ✗ API: search failed (%v)\n", err)
			return nil
		}
		results, _ := result["results"].([]interface{})
		fmt.Printf("  ✓ API: search works (%d+ items accessible)\n", len(results))

		fmt.Println()
		fmt.Println("All checks passed ✓")
		return nil
	},
}

func init() {
	authLoginCmd.Flags().Bool("with-token", false, "Read token from standard input")
	authLoginCmd.Flags().StringP("profile", "p", "", "Profile name to save credentials under (default: \"default\")")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authDoctorCmd)
	authCmd.AddCommand(authSwitchCmd)
}

// detectIntegrationType inspects the bot object returned by
// GET /v1/users/me and classifies the integration as "internal",
// "public", or "" when the shape is unrecognizable.
//
// Notion's response includes `bot.owner.type`, which is:
//   - "workspace" for internal integrations (owned by the workspace)
//   - "user" for public integrations installed by an individual user
//
// Ref: https://developers.notion.com/reference/user#bot
func detectIntegrationType(botInfo map[string]interface{}) string {
	owner, ok := botInfo["owner"].(map[string]interface{})
	if !ok {
		return ""
	}
	ownerType, _ := owner["type"].(string)
	switch ownerType {
	case "workspace":
		return "internal"
	case "user":
		return "public"
	default:
		return ""
	}
}
