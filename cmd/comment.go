package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/4ier/notion-cli/internal/client"
	"github.com/4ier/notion-cli/internal/render"
	"github.com/4ier/notion-cli/internal/util"
	"github.com/spf13/cobra"
)

var commentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Work with comments",
}

var commentListCmd = &cobra.Command{
	Use:   "list <page-id|url>",
	Short: "List comments on a page",
	Long: `List all comments on a Notion page or block.

Examples:
  notion comment list abc123
  notion comment list abc123 --format json
  notion comment list abc123 --all`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		blockID := util.ResolveID(args[0])
		all, _ := cmd.Flags().GetBool("all")
		cursor, _ := cmd.Flags().GetString("cursor")
		c := client.New(token)
		c.SetDebug(debugMode)

		var allResults []interface{}
		currentCursor := cursor

		for {
			result, err := c.ListComments(blockID, 100, currentCursor)
			if err != nil {
				return fmt.Errorf("list comments: %w", err)
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

		if len(allResults) == 0 {
			fmt.Println("No comments found.")
			return nil
		}

		for _, r := range allResults {
			comment, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := comment["id"].(string)
			createdTime, _ := comment["created_time"].(string)
			if len(createdTime) > 10 {
				createdTime = createdTime[:10]
			}

			var text string
			if richText, ok := comment["rich_text"].([]interface{}); ok {
				for _, t := range richText {
					if m, ok := t.(map[string]interface{}); ok {
						if pt, ok := m["plain_text"].(string); ok {
							text += pt
						}
					}
				}
			}

			render.Field("Comment", text)
			render.Subtitle(fmt.Sprintf("  ID: %s  Created: %s", id, createdTime))
			fmt.Println()
		}

		return nil
	},
}

var commentAddCmd = &cobra.Command{
	Use:   "add <page-id|url> <text>",
	Short: "Add a comment to a page",
	Long: `Add a comment to a Notion page.

Examples:
  notion comment add abc123 "This looks great!"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		pageID := util.ResolveID(args[0])
		text := args[1]

		c := client.New(token)
		c.SetDebug(debugMode)

		data, err := c.AddComment(pageID, text)
		if err != nil {
			return fmt.Errorf("add comment: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			json.Unmarshal(data, &result)
			return render.JSON(result)
		}

		var result map[string]interface{}
		json.Unmarshal(data, &result)
		id, _ := result["id"].(string)

		render.Title("✓", "Comment added")
		render.Field("ID", id)
		return nil
	},
}

var commentGetCmd = &cobra.Command{
	Use:   "get <comment-id>",
	Short: "Get a comment by ID",
	Long: `Retrieve a single comment by its ID.

Examples:
  notion comment get abc123
  notion comment get abc123 --format json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		commentID := args[0]
		c := client.New(token)
		c.SetDebug(debugMode)

		data, err := c.Get("/v1/comments/" + commentID)
		if err != nil {
			return fmt.Errorf("get comment: %w", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return err
		}

		if outputFormat == "json" {
			return render.JSON(result)
		}

		id, _ := result["id"].(string)
		createdTime, _ := result["created_time"].(string)

		var text string
		if richText, ok := result["rich_text"].([]interface{}); ok {
			for _, t := range richText {
				if m, ok := t.(map[string]interface{}); ok {
					if pt, ok := m["plain_text"].(string); ok {
						text += pt
					}
				}
			}
		}

		render.Field("Comment", text)
		render.Field("ID", id)
		render.Field("Created", createdTime)
		return nil
	},
}

func init() {
	commentListCmd.Flags().String("cursor", "", "Pagination cursor")
	commentListCmd.Flags().Bool("all", false, "Fetch all pages of results")

	commentCmd.AddCommand(commentListCmd)
	commentCmd.AddCommand(commentAddCmd)
	commentCmd.AddCommand(commentGetCmd)
}
