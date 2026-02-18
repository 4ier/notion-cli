package cmd

import (
	"bufio"
	"fmt"
	"os"
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
	Long:  `Authenticate with Notion using an integration token.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		withToken, _ := cmd.Flags().GetBool("with-token")

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

		// Save config
		cfg := &config.Config{
			Token:         token,
			WorkspaceName: workspaceName,
			WorkspaceID:   workspaceID,
			BotID:         botID,
		}
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		render.Title("✓", fmt.Sprintf("Logged in to %s", workspaceName))
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			fmt.Println("✗ Not authenticated")
			return nil
		}

		c := client.New(token)
		me, err := c.GetMe()
		if err != nil {
			return fmt.Errorf("token is invalid: %w", err)
		}

		botInfo, _ := me["bot"].(map[string]interface{})
		workspaceName, _ := botInfo["workspace_name"].(string)
		name, _ := me["name"].(string)

		render.Title("✓", "Authenticated")
		render.Field("Workspace", workspaceName)
		render.Field("Bot", name)
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of Notion",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := &config.Config{}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Println("✓ Logged out")
		return nil
	},
}

func init() {
	authLoginCmd.Flags().Bool("with-token", false, "Read token from standard input")
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)
}
