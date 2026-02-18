package cmd

import (
	"fmt"

	"github.com/4ier/notion-cli/internal/client"
	"github.com/4ier/notion-cli/internal/render"
	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "User information",
}

var userMeCmd = &cobra.Command{
	Use:   "me",
	Short: "Show current bot user",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		me, err := c.GetMe()
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			return render.JSON(me)
		}

		name, _ := me["name"].(string)
		id, _ := me["id"].(string)
		botInfo, _ := me["bot"].(map[string]interface{})
		workspaceName, _ := botInfo["workspace_name"].(string)

		render.Title("🤖", name)
		render.Field("ID", id)
		render.Field("Workspace", workspaceName)
		return nil
	},
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspace users",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		all, _ := cmd.Flags().GetBool("all")
		cursor, _ := cmd.Flags().GetString("cursor")
		c := client.New(token)
		c.SetDebug(debugMode)

		var allResults []interface{}
		currentCursor := cursor

		for {
			result, err := c.GetUsers(100, currentCursor)
			if err != nil {
				return err
			}

			if outputFormat == "json" && !all {
				return render.JSON(result)
			}

			results, _ := result["results"].([]interface{})
			allResults = append(allResults, results...)

			hasMore, _ := result["has_more"].(bool)
			if !all || !hasMore {
				if all && outputFormat == "json" {
					return render.JSON(map[string]interface{}{"results": allResults})
				}
				break
			}
			nextCursor, _ := result["next_cursor"].(string)
			currentCursor = nextCursor
		}

		headers := []string{"NAME", "TYPE", "ID"}
		var rows [][]string

		for _, r := range allResults {
			user, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := user["name"].(string)
			userType, _ := user["type"].(string)
			id, _ := user["id"].(string)
			rows = append(rows, []string{name, userType, id})
		}

		if len(rows) == 0 {
			fmt.Println("No users found.")
			return nil
		}

		render.Table(headers, rows)
		return nil
	},
}

var userGetCmd = &cobra.Command{
	Use:   "get <user-id>",
	Short: "Get user details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		user, err := c.GetUser(args[0])
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}

		if outputFormat == "json" {
			return render.JSON(user)
		}

		name, _ := user["name"].(string)
		id, _ := user["id"].(string)
		userType, _ := user["type"].(string)

		render.Title("👤", name)
		render.Field("ID", id)
		render.Field("Type", userType)

		if userType == "person" {
			if person, ok := user["person"].(map[string]interface{}); ok {
				email, _ := person["email"].(string)
				if email != "" {
					render.Field("Email", email)
				}
			}
		}
		return nil
	},
}

func init() {
	userListCmd.Flags().String("cursor", "", "Pagination cursor")
	userListCmd.Flags().Bool("all", false, "Fetch all pages of results")

	userCmd.AddCommand(userMeCmd)
	userCmd.AddCommand(userListCmd)
	userCmd.AddCommand(userGetCmd)
}
