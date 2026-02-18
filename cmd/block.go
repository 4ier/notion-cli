package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

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
	Short: "Append blocks to a page",
	Long: `Append content to a Notion page or block.

Supports plain text, block types, and markdown files.

Examples:
  notion block append <page-id> "Hello world"
  notion block append <page-id> --type heading1 "Section Title"
  notion block append <page-id> --type code --lang go "fmt.Println()"
  notion block append <page-id> --file notes.md`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		parentID := util.ResolveID(args[0])
		blockType, _ := cmd.Flags().GetString("type")
		filePath, _ := cmd.Flags().GetString("file")

		if blockType == "" {
			blockType = "paragraph"
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		var children []map[string]interface{}

		if filePath != "" {
			// Read file and split into paragraph blocks
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			paragraphs := strings.Split(strings.TrimSpace(string(data)), "\n\n")
			for _, p := range paragraphs {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				children = append(children, map[string]interface{}{
					"object": "block",
					"type":   "paragraph",
					"paragraph": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{"text": map[string]interface{}{"content": p}},
						},
					},
				})
			}
		} else {
			text := ""
			if len(args) > 1 {
				text = args[1]
			}
			if text == "" {
				return fmt.Errorf("text content or --file is required")
			}

			notionType := mapBlockType(blockType)
			blockContent := map[string]interface{}{
				"rich_text": []map[string]interface{}{
					{"text": map[string]interface{}{"content": text}},
				},
			}
			if notionType == "code" {
				lang, _ := cmd.Flags().GetString("lang")
				if lang == "" {
					lang = "plain text"
				}
				blockContent["language"] = lang
			}
			children = append(children, map[string]interface{}{
				"object":   "block",
				"type":     notionType,
				notionType: blockContent,
			})
		}

		if len(children) == 0 {
			return fmt.Errorf("no content to append")
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

		fmt.Printf("✓ %d block(s) appended\n", len(children))
		return nil
	},
}

var blockDeleteCmd = &cobra.Command{
	Use:   "delete <block-id ...>",
	Short: "Delete one or more blocks",
	Long: `Delete blocks by ID. Supports multiple IDs.

Examples:
  notion block delete abc123
  notion block delete abc123 def456 ghi789`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		deleted := 0
		for _, arg := range args {
			blockID := util.ResolveID(arg)
			_, err = c.Delete("/v1/blocks/" + blockID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "✗ Failed to delete %s: %v\n", blockID, err)
				continue
			}
			deleted++
		}

		if outputFormat != "json" {
			fmt.Printf("✓ %d block(s) deleted\n", deleted)
		}
		return nil
	},
}

var blockInsertCmd = &cobra.Command{
	Use:   "insert <parent-id|url> [text]",
	Short: "Insert a block after a specific block",
	Long: `Insert content after a specific child block within a parent.

Examples:
  notion block insert <page-id> "New paragraph" --after <block-id>
  notion block insert <page-id> "Section" --after <block-id> --type h2
  notion block insert <page-id> --file notes.md --after <block-id>`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		parentID := util.ResolveID(args[0])
		afterID, _ := cmd.Flags().GetString("after")
		blockType, _ := cmd.Flags().GetString("type")
		filePath, _ := cmd.Flags().GetString("file")

		if afterID == "" {
			return fmt.Errorf("--after <block-id> is required (use 'block append' to add to end)")
		}
		afterID = util.ResolveID(afterID)

		if blockType == "" {
			blockType = "paragraph"
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		var children []map[string]interface{}

		if filePath != "" {
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			paragraphs := strings.Split(strings.TrimSpace(string(data)), "\n\n")
			for _, p := range paragraphs {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				children = append(children, map[string]interface{}{
					"object": "block",
					"type":   "paragraph",
					"paragraph": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{"text": map[string]interface{}{"content": p}},
						},
					},
				})
			}
		} else {
			text := ""
			if len(args) > 1 {
				text = args[1]
			}
			if text == "" {
				return fmt.Errorf("text content or --file is required")
			}

			notionType := mapBlockType(blockType)
			blockContent := map[string]interface{}{
				"rich_text": []map[string]interface{}{
					{"text": map[string]interface{}{"content": text}},
				},
			}
			if notionType == "code" {
				lang, _ := cmd.Flags().GetString("lang")
				if lang == "" {
					lang = "plain text"
				}
				blockContent["language"] = lang
			}
			children = append(children, map[string]interface{}{
				"object":   "block",
				"type":     notionType,
				notionType: blockContent,
			})
		}

		reqBody := map[string]interface{}{
			"children": children,
			"after":    afterID,
		}

		data, err := c.Patch(fmt.Sprintf("/v1/blocks/%s/children", parentID), reqBody)
		if err != nil {
			return fmt.Errorf("insert block: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			json.Unmarshal(data, &result)
			return render.JSON(result)
		}

		fmt.Printf("✓ %d block(s) inserted\n", len(children))
		return nil
	},
}

func init() {
	blockAppendCmd.Flags().StringP("type", "t", "paragraph", "Block type: paragraph, h1, h2, h3, todo, bullet, numbered, quote, code, callout, divider")
	blockAppendCmd.Flags().String("lang", "plain text", "Language for code blocks (e.g. go, python, bash)")
	blockAppendCmd.Flags().String("file", "", "Read content from a file (each double-newline-separated section becomes a block)")
	blockInsertCmd.Flags().String("after", "", "Block ID to insert after (required)")
	blockInsertCmd.Flags().StringP("type", "t", "paragraph", "Block type")
	blockInsertCmd.Flags().String("lang", "plain text", "Language for code blocks")
	blockInsertCmd.Flags().String("file", "", "Read content from a file")
	blockListCmd.Flags().String("cursor", "", "Pagination cursor")
	blockListCmd.Flags().Bool("all", false, "Fetch all pages of results")
	blockUpdateCmd.Flags().String("text", "", "New text content (required)")
	blockUpdateCmd.Flags().StringP("type", "t", "", "Block type (auto-detected if not specified)")

	blockCmd.AddCommand(blockListCmd)
	blockCmd.AddCommand(blockGetCmd)
	blockCmd.AddCommand(blockAppendCmd)
	blockCmd.AddCommand(blockInsertCmd)
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
