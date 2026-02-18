package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/4ier/notion-cli/internal/client"
	"github.com/4ier/notion-cli/internal/render"
	"github.com/4ier/notion-cli/internal/util"
	"github.com/spf13/cobra"
)

var blockCmd = &cobra.Command{
	Use:   "block",
	Short: "Work with content blocks",
}

var blockListCmd = &cobra.Command{
	Use:   "list <parent-id|url>",
	Short: "List child blocks",
	Long: `List all child blocks of a page or block.

Examples:
  notion block list <page-id>
  notion block list <page-id> --format json
  notion block list <page-id> --all`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		parentID := util.ResolveID(args[0])
		all, _ := cmd.Flags().GetBool("all")
		cursor, _ := cmd.Flags().GetString("cursor")
		c := client.New(token)
		c.SetDebug(debugMode)

		var allResults []interface{}
		currentCursor := cursor

		for {
			result, err := c.GetBlockChildren(parentID, 100, currentCursor)
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

		for _, b := range allResults {
			block, ok := b.(map[string]interface{})
			if !ok {
				continue
			}
			renderBlock(block, 0)
		}

		return nil
	},
}

var blockGetCmd = &cobra.Command{
	Use:   "get <block-id|url>",
	Short: "Get a specific block",
	Long: `Retrieve a single block by ID.

Examples:
  notion block get abc123
  notion block get abc123 --format json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		blockID := util.ResolveID(args[0])
		c := client.New(token)
		c.SetDebug(debugMode)

		block, err := c.GetBlock(blockID)
		if err != nil {
			return fmt.Errorf("get block: %w", err)
		}

		if outputFormat == "json" {
			return render.JSON(block)
		}

		blockType, _ := block["type"].(string)
		id, _ := block["id"].(string)
		hasChildren, _ := block["has_children"].(bool)

		render.Title("🧱", fmt.Sprintf("Block: %s", blockType))
		render.Field("ID", id)
		render.Field("Type", blockType)
		render.Field("Has Children", fmt.Sprintf("%v", hasChildren))
		fmt.Println()
		renderBlock(block, 0)

		return nil
	},
}

var blockUpdateCmd = &cobra.Command{
	Use:   "update <block-id|url>",
	Short: "Update a block",
	Long: `Update a block's content.

Examples:
  notion block update abc123 --text "Updated content"
  notion block update abc123 --type paragraph --text "New text"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		blockID := util.ResolveID(args[0])
		text, _ := cmd.Flags().GetString("text")
		blockType, _ := cmd.Flags().GetString("type")

		c := client.New(token)
		c.SetDebug(debugMode)

		// If no type specified, get the block first to determine its type
		if blockType == "" {
			block, err := c.GetBlock(blockID)
			if err != nil {
				return fmt.Errorf("get block: %w", err)
			}
			blockType, _ = block["type"].(string)
		} else {
			blockType = mapBlockType(blockType)
		}

		if text == "" {
			return fmt.Errorf("--text is required")
		}

		body := map[string]interface{}{
			blockType: map[string]interface{}{
				"rich_text": []map[string]interface{}{
					{"text": map[string]interface{}{"content": text}},
				},
			},
		}

		data, err := c.Patch("/v1/blocks/"+blockID, body)
		if err != nil {
			return fmt.Errorf("update block: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			json.Unmarshal(data, &result)
			return render.JSON(result)
		}

		fmt.Println("✓ Block updated")
		return nil
	},
}

var blockAppendCmd = &cobra.Command{
	Use:   "append <parent-id|url> [text]",
	Short: "Append a block to a page",
	Long: `Append content to a Notion page or block.

Examples:
  notion block append <page-id> "Hello world"
  notion block append <page-id> --type heading1 "Section Title"
  echo "piped content" | notion block append <page-id>`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		parentID := util.ResolveID(args[0])
		blockType, _ := cmd.Flags().GetString("type")
		if blockType == "" {
			blockType = "paragraph"
		}

		text := ""
		if len(args) > 1 {
			text = args[1]
		}

		if text == "" {
			return fmt.Errorf("text content is required")
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		// Map friendly type names to Notion block types
		notionType := mapBlockType(blockType)

		children := []map[string]interface{}{
			{
				"object": "block",
				"type":   notionType,
				notionType: map[string]interface{}{
					"rich_text": []map[string]interface{}{
						{"text": map[string]interface{}{"content": text}},
					},
				},
			},
		}

		reqBody := map[string]interface{}{
			"children": children,
		}

		data, err := c.Patch(fmt.Sprintf("/v1/blocks/%s/children", parentID), reqBody)
		if err != nil {
			return fmt.Errorf("append block: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			json.Unmarshal(data, &result)
			return render.JSON(result)
		}

		fmt.Println("✓ Block appended")
		return nil
	},
}

var blockDeleteCmd = &cobra.Command{
	Use:   "delete <block-id>",
	Short: "Delete a block",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		blockID := util.ResolveID(args[0])
		c := client.New(token)
		c.SetDebug(debugMode)

		_, err = c.Delete("/v1/blocks/" + blockID)
		if err != nil {
			return fmt.Errorf("delete block: %w", err)
		}

		fmt.Println("✓ Block deleted")
		return nil
	},
}

func init() {
	blockAppendCmd.Flags().StringP("type", "t", "paragraph", "Block type: paragraph, heading1, heading2, heading3, todo, bullet, numbered, quote, code, callout, divider")
	blockUpdateCmd.Flags().String("text", "", "New text content (required)")
	blockUpdateCmd.Flags().StringP("type", "t", "", "Block type (auto-detected if not specified)")

	blockCmd.AddCommand(blockListCmd)
	blockCmd.AddCommand(blockGetCmd)
	blockCmd.AddCommand(blockAppendCmd)
	blockCmd.AddCommand(blockUpdateCmd)
	blockCmd.AddCommand(blockDeleteCmd)
}

func mapBlockType(t string) string {
	switch t {
	case "heading1", "h1":
		return "heading_1"
	case "heading2", "h2":
		return "heading_2"
	case "heading3", "h3":
		return "heading_3"
	case "bullet":
		return "bulleted_list_item"
	case "numbered":
		return "numbered_list_item"
	case "todo":
		return "to_do"
	case "paragraph", "p":
		return "paragraph"
	case "quote":
		return "quote"
	case "code":
		return "code"
	case "callout":
		return "callout"
	case "divider":
		return "divider"
	default:
		return t
	}
}
