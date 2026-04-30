package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
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
  notion block list <page-id> --all
  notion block list <page-id> --depth 2`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		parentID := util.ResolveID(args[0])
		all, _ := cmd.Flags().GetBool("all")
		cursor, _ := cmd.Flags().GetString("cursor")
		depth, _ := cmd.Flags().GetInt("depth")
		if depth < 1 {
			depth = 1
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		allResults, err := fetchBlockChildren(c, parentID, cursor, all)
		if err != nil {
			return err
		}

		// Recursively fetch nested children
		if depth > 1 {
			allResults = fetchNestedBlocks(c, allResults, depth-1)
		}

		if outputFormat == "json" {
			return render.JSON(map[string]interface{}{"results": allResults})
		}

		mdMode, _ := cmd.Flags().GetBool("md")
		if outputFormat == "md" || outputFormat == "markdown" {
			mdMode = true
		}
		for _, b := range allResults {
			block, ok := b.(map[string]interface{})
			if !ok {
				continue
			}
			if mdMode {
				renderBlockMarkdown(block, 0)
			} else {
				renderBlockRecursive(block, 0)
			}
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
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parse response: %w", err)
			}
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

Supports plain text, block types, and markdown files. Large markdown files
are handled transparently:
  - >100 children are auto-batched into sequential PATCHes.
  - code / rich_text exceeding Notion's 2000-char limit are split by
    default (override with --on-oversize=truncate|fail).

Examples:
  notion block append <page-id> "Hello world"
  notion block append <page-id> --type heading1 "Section Title"
  notion block append <page-id> --type code --lang go "fmt.Println()"
  notion block append <page-id> --file notes.md
  notion block append <page-id> --file big.md --on-oversize=truncate
  notion block append <page-id> --image-url https://example.com/a.png --caption "图 1-1"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		parentID := util.ResolveID(args[0])
		blockType, _ := cmd.Flags().GetString("type")
		filePath, _ := cmd.Flags().GetString("file")
		imageURL, _ := cmd.Flags().GetString("image-url")
		caption, _ := cmd.Flags().GetString("caption")
		onOversizeRaw, _ := cmd.Flags().GetString("on-oversize")
		mode, err := parseOversizeMode(onOversizeRaw)
		if err != nil {
			return err
		}

		text := ""
		if len(args) > 1 {
			text = args[1]
		}

		if err := validateMediaFlags(imageURL, filePath, text); err != nil {
			return err
		}

		if blockType == "" {
			blockType = "paragraph"
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		var children []map[string]interface{}

		if imageURL != "" {
			children = append(children, buildExternalImageBlock(imageURL, caption))
		} else if filePath != "" {
			// Read file and parse markdown to blocks
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			children = parseMarkdownToBlocks(string(data))
		} else {
			if text == "" {
				return fmt.Errorf("text content, --file, or --image-url is required")
			}

			notionType := mapBlockType(blockType)
			blockContent := map[string]interface{}{
				"rich_text": []map[string]interface{}{
					{"text": map[string]interface{}{"content": text}},
				},
			}
			if notionType == "code" {
				lang, _ := cmd.Flags().GetString("lang")
				blockContent["language"] = normalizeCodeLanguage(lang)
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

		children, err = handleOversizedBlocks(children, mode)
		if err != nil {
			return err
		}

		data, err := appendChildrenBatched(c, parentID, "", children)
		if err != nil {
			return fmt.Errorf("append block: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parse response: %w", err)
			}
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
  notion block insert <page-id> --file notes.md --after <block-id>
  notion block insert <page-id> --after <block-id> --image-url https://example.com/a.png --caption "图 1-1"`,
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
		imageURL, _ := cmd.Flags().GetString("image-url")
		caption, _ := cmd.Flags().GetString("caption")

		text := ""
		if len(args) > 1 {
			text = args[1]
		}

		if err := validateMediaFlags(imageURL, filePath, text); err != nil {
			return err
		}

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

		onOversizeRaw, _ := cmd.Flags().GetString("on-oversize")
		mode, err := parseOversizeMode(onOversizeRaw)
		if err != nil {
			return err
		}

		if imageURL != "" {
			children = append(children, buildExternalImageBlock(imageURL, caption))
		} else if filePath != "" {
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			children = parseMarkdownToBlocks(string(data))
		} else {
			if text == "" {
				return fmt.Errorf("text content, --file, or --image-url is required")
			}

			notionType := mapBlockType(blockType)
			blockContent := map[string]interface{}{
				"rich_text": []map[string]interface{}{
					{"text": map[string]interface{}{"content": text}},
				},
			}
			if notionType == "code" {
				lang, _ := cmd.Flags().GetString("lang")
				blockContent["language"] = normalizeCodeLanguage(lang)
			}
			children = append(children, map[string]interface{}{
				"object":   "block",
				"type":     notionType,
				notionType: blockContent,
			})
		}

		children, err = handleOversizedBlocks(children, mode)
		if err != nil {
			return err
		}

		data, err := appendChildrenBatched(c, parentID, afterID, children)
		if err != nil {
			return fmt.Errorf("insert block: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parse response: %w", err)
			}
			return render.JSON(result)
		}

		fmt.Printf("✓ %d block(s) inserted\n", len(children))
		return nil
	},
}

var blockMoveCmd = &cobra.Command{
	Use:   "move <block-id|url>",
	Short: "Move a block to a new position",
	Long: `Move a block within its parent or to a different parent.

Use --after to position after a specific block.
Use --before to position before a specific block.
Use --parent to move to a different parent block/page.

Examples:
  notion block move abc123 --after def456
  notion block move abc123 --before ghi789
  notion block move abc123 --parent xyz000
  notion block move abc123 --parent xyz000 --after def456`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		blockID := util.ResolveID(args[0])
		afterID, _ := cmd.Flags().GetString("after")
		beforeID, _ := cmd.Flags().GetString("before")
		parentID, _ := cmd.Flags().GetString("parent")

		if afterID == "" && beforeID == "" && parentID == "" {
			return fmt.Errorf("at least one of --after, --before, or --parent is required")
		}

		if afterID != "" && beforeID != "" {
			return fmt.Errorf("cannot specify both --after and --before")
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		// Get the current block to find its parent if not specified
		currentBlock, err := c.GetBlock(blockID)
		if err != nil {
			return fmt.Errorf("get block: %w", err)
		}

		// Determine the target parent
		targetParentID := parentID
		if targetParentID == "" {
			// Use the current parent
			parent, _ := currentBlock["parent"].(map[string]interface{})
			if pid, ok := parent["page_id"].(string); ok {
				targetParentID = pid
			} else if pid, ok := parent["block_id"].(string); ok {
				targetParentID = pid
			}
		} else {
			targetParentID = util.ResolveID(targetParentID)
		}

		if targetParentID == "" {
			return fmt.Errorf("could not determine parent block/page")
		}

		// Handle --before by finding the block that comes before the target
		var afterBlockID string
		if beforeID != "" {
			beforeID = util.ResolveID(beforeID)
			// Get all children of the parent
			children, err := fetchBlockChildren(c, targetParentID, "", true)
			if err != nil {
				return fmt.Errorf("get parent children: %w", err)
			}

			// Find the block that comes before the target
			for i, child := range children {
				childBlock, ok := child.(map[string]interface{})
				if !ok {
					continue
				}
				childID, _ := childBlock["id"].(string)
				if childID == beforeID {
					if i > 0 {
						// Get the ID of the previous block
						prevBlock, _ := children[i-1].(map[string]interface{})
						afterBlockID, _ = prevBlock["id"].(string)
					}
					// If i == 0, afterBlockID stays empty (insert at beginning)
					break
				}
			}
		} else if afterID != "" {
			afterBlockID = util.ResolveID(afterID)
		}

		// Build the request body for moving the block
		// Note: Notion API uses PATCH /v1/blocks/{id} with parent and after fields
		body := map[string]interface{}{}

		// Set the parent (try page_id first, then block_id)
		// We need to check if the parent is a page or a block
		parentBlock, err := c.GetBlock(targetParentID)
		if err == nil {
			parentType, _ := parentBlock["type"].(string)
			if parentType == "child_page" || parentType == "" {
				// It's a page
				body["parent"] = map[string]interface{}{
					"page_id": targetParentID,
				}
			} else {
				// It's a block
				body["parent"] = map[string]interface{}{
					"block_id": targetParentID,
				}
			}
		} else {
			// Assume it's a page if we can't get the block
			body["parent"] = map[string]interface{}{
				"page_id": targetParentID,
			}
		}

		if afterBlockID != "" {
			body["after"] = afterBlockID
		}

		data, err := c.Patch("/v1/blocks/"+blockID, body)
		if err != nil {
			return fmt.Errorf("move block: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parse response: %w", err)
			}
			return render.JSON(result)
		}

		if afterBlockID != "" {
			fmt.Printf("✓ Block moved after %s\n", afterBlockID)
		} else if beforeID != "" {
			fmt.Printf("✓ Block moved before %s\n", beforeID)
		} else {
			fmt.Printf("✓ Block moved to %s\n", targetParentID)
		}
		return nil
	},
}

func init() {
	blockAppendCmd.Flags().StringP("type", "t", "paragraph", "Block type: paragraph, h1, h2, h3, todo, bullet, numbered, quote, code, callout, divider")
	blockAppendCmd.Flags().String("lang", "plain text", "Language for code blocks (e.g. go, python, bash)")
	blockAppendCmd.Flags().String("file", "", "Read content from a file (each double-newline-separated section becomes a block)")
	blockAppendCmd.Flags().String("image-url", "", "Append an external image block by URL (http/https)")
	blockAppendCmd.Flags().String("caption", "", "Caption for --image-url (optional)")
	blockAppendCmd.Flags().String("on-oversize", "split", "Behavior for rich_text >2000 chars: split|truncate|fail")
	blockInsertCmd.Flags().String("after", "", "Block ID to insert after (required)")
	blockInsertCmd.Flags().StringP("type", "t", "paragraph", "Block type")
	blockInsertCmd.Flags().String("lang", "plain text", "Language for code blocks")
	blockInsertCmd.Flags().String("file", "", "Read content from a file")
	blockInsertCmd.Flags().String("image-url", "", "Insert an external image block by URL (http/https)")
	blockInsertCmd.Flags().String("caption", "", "Caption for --image-url (optional)")
	blockInsertCmd.Flags().String("on-oversize", "split", "Behavior for rich_text >2000 chars: split|truncate|fail")
	blockListCmd.Flags().String("cursor", "", "Pagination cursor")
	blockListCmd.Flags().Bool("all", false, "Fetch all pages of results")
	blockListCmd.Flags().Int("depth", 1, "Depth of nested blocks to fetch (default 1)")
	blockListCmd.Flags().Bool("md", false, "Output as Markdown")
	blockUpdateCmd.Flags().String("text", "", "New text content (required)")
	blockUpdateCmd.Flags().StringP("type", "t", "", "Block type (auto-detected if not specified)")
	blockMoveCmd.Flags().String("after", "", "Block ID to position after")
	blockMoveCmd.Flags().String("before", "", "Block ID to position before")
	blockMoveCmd.Flags().String("parent", "", "New parent block/page ID to move to")

	blockCmd.AddCommand(blockListCmd)
	blockCmd.AddCommand(blockGetCmd)
	blockCmd.AddCommand(blockAppendCmd)
	blockCmd.AddCommand(blockInsertCmd)
	blockCmd.AddCommand(blockUpdateCmd)
	blockCmd.AddCommand(blockDeleteCmd)
	blockCmd.AddCommand(blockMoveCmd)
}

func buildExternalImageBlock(url, caption string) map[string]interface{} {
	image := map[string]interface{}{
		"type":     "external",
		"external": map[string]interface{}{"url": url},
	}
	if caption != "" {
		image["caption"] = []map[string]interface{}{
			{"type": "text", "text": map[string]interface{}{"content": caption}},
		}
	}
	return map[string]interface{}{
		"object": "block",
		"type":   "image",
		"image":  image,
	}
}

func validateMediaFlags(imageURL, filePath, text string) error {
	if imageURL == "" {
		return nil
	}
	if filePath != "" {
		return fmt.Errorf("--image-url cannot be combined with --file")
	}
	if text != "" {
		return fmt.Errorf("--image-url cannot be combined with a positional text argument")
	}
	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		return fmt.Errorf("--image-url must be an http:// or https:// URL")
	}
	return nil
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

// fetchBlockChildren fetches all children of a block with optional pagination.
func fetchBlockChildren(c *client.Client, parentID, cursor string, all bool) ([]interface{}, error) {
	var allResults []interface{}
	currentCursor := cursor

	for {
		result, err := c.GetBlockChildren(parentID, 100, currentCursor)
		if err != nil {
			return nil, err
		}

		results, _ := result["results"].([]interface{})
		allResults = append(allResults, results...)

		hasMore, _ := result["has_more"].(bool)
		if !all || !hasMore {
			break
		}
		nextCursor, _ := result["next_cursor"].(string)
		currentCursor = nextCursor
	}

	return allResults, nil
}

// fetchNestedBlocks recursively fetches children for blocks that have them.
func fetchNestedBlocks(c *client.Client, blocks []interface{}, remainingDepth int) []interface{} {
	if remainingDepth <= 0 {
		return blocks
	}
	for _, b := range blocks {
		block, ok := b.(map[string]interface{})
		if !ok {
			continue
		}
		hasChildren, _ := block["has_children"].(bool)
		if !hasChildren {
			continue
		}
		id, _ := block["id"].(string)
		if id == "" {
			continue
		}
		children, err := fetchBlockChildren(c, id, "", true)
		if err != nil {
			continue
		}
		if remainingDepth > 1 {
			children = fetchNestedBlocks(c, children, remainingDepth-1)
		}
		block["_children"] = children
	}
	return blocks
}

// renderBlockRecursive renders a block and its nested children.
func renderBlockRecursive(block map[string]interface{}, indent int) {
	renderBlock(block, indent)
	if children, ok := block["_children"].([]interface{}); ok {
		for _, child := range children {
			if childBlock, ok := child.(map[string]interface{}); ok {
				renderBlockRecursive(childBlock, indent+1)
			}
		}
	}
}

// parseMarkdownToBlocks converts markdown text to Notion block objects.
func parseMarkdownToBlocks(content string) []map[string]interface{} {
	var blocks []map[string]interface{}
	lines := strings.Split(content, "\n")

	i := 0
	for i < len(lines) {
		line := lines[i]

		// Code fence
		if strings.HasPrefix(line, "```") {
			lang := normalizeCodeLanguage(strings.TrimPrefix(line, "```"))
			var codeLines []string
			i++
			for i < len(lines) && !strings.HasPrefix(lines[i], "```") {
				codeLines = append(codeLines, lines[i])
				i++
			}
			i++ // skip closing ```
			blocks = append(blocks, map[string]interface{}{
				"object": "block",
				"type":   "code",
				"code": map[string]interface{}{
					"rich_text": []map[string]interface{}{
						{"text": map[string]interface{}{"content": strings.Join(codeLines, "\n")}},
					},
					"language": lang,
				},
			})
			continue
		}

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}

		// Headings
		if strings.HasPrefix(line, "### ") {
			blocks = append(blocks, makeTextBlock("heading_3", strings.TrimPrefix(line, "### ")))
			i++
			continue
		}
		if strings.HasPrefix(line, "## ") {
			blocks = append(blocks, makeTextBlock("heading_2", strings.TrimPrefix(line, "## ")))
			i++
			continue
		}
		if strings.HasPrefix(line, "# ") {
			blocks = append(blocks, makeTextBlock("heading_1", strings.TrimPrefix(line, "# ")))
			i++
			continue
		}

		// Todo (must check before bullet — "- [ ]" starts with "- ")
		if strings.HasPrefix(line, "- [ ] ") {
			block := makeTextBlock("to_do", line[6:])
			block["to_do"].(map[string]interface{})["checked"] = false
			blocks = append(blocks, block)
			i++
			continue
		}
		if strings.HasPrefix(line, "- [x] ") || strings.HasPrefix(line, "- [X] ") {
			block := makeTextBlock("to_do", line[6:])
			block["to_do"].(map[string]interface{})["checked"] = true
			blocks = append(blocks, block)
			i++
			continue
		}

		// Bullet list
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			blocks = append(blocks, makeTextBlock("bulleted_list_item", line[2:]))
			i++
			continue
		}

		// Numbered list
		if len(line) > 2 && line[0] >= '0' && line[0] <= '9' && strings.Contains(line[:5], ". ") {
			idx := strings.Index(line, ". ")
			blocks = append(blocks, makeTextBlock("numbered_list_item", line[idx+2:]))
			i++
			continue
		}

		// Quote
		if strings.HasPrefix(line, "> ") {
			blocks = append(blocks, makeTextBlock("quote", strings.TrimPrefix(line, "> ")))
			i++
			continue
		}

		// Divider
		if line == "---" || line == "***" || line == "___" {
			blocks = append(blocks, map[string]interface{}{
				"object":  "block",
				"type":    "divider",
				"divider": map[string]interface{}{},
			})
			i++
			continue
		}

		// GFM Table: starts with '|'
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			// Collect all consecutive pipe-starting lines
			var tableLines []string
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|") {
				tableLines = append(tableLines, lines[i])
				i++
			}
			// Need at least header + separator + 1 data row to be a valid GFM table
			if len(tableLines) >= 2 && isTableSeparator(tableLines[1]) {
				tableBlock := buildTableBlock(tableLines)
				if tableBlock != nil {
					blocks = append(blocks, tableBlock)
					continue
				}
			}
			// Not a valid table — treat each line as a paragraph
			for _, tl := range tableLines {
				blocks = append(blocks, makeTextBlock("paragraph", tl))
			}
			continue
		}

		// Default: paragraph
		blocks = append(blocks, makeTextBlock("paragraph", line))
		i++
	}

	return blocks
}

// isTableSeparator returns true when a line looks like a GFM table separator (|---|---|).
func isTableSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return false
	}
	// Strip leading/trailing '|', split cells, check each cell is only -/:/space
	inner := strings.Trim(trimmed, "|")
	cells := strings.Split(inner, "|")
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			continue
		}
		// Must consist of dashes and optional colons (alignment markers)
		for _, ch := range cell {
			if ch != '-' && ch != ':' {
				return false
			}
		}
	}
	return true
}

// splitTableRow splits a pipe-delimited table row into trimmed cell strings.
func splitTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	// Strip leading/trailing '|'
	trimmed = strings.Trim(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

// buildTableBlock converts collected GFM table lines into a Notion table block.
// tableLines[0] = header row, tableLines[1] = separator, tableLines[2:] = data rows.
func buildTableBlock(tableLines []string) map[string]interface{} {
	headerCells := splitTableRow(tableLines[0])
	tableWidth := len(headerCells)
	if tableWidth == 0 {
		return nil
	}

	var rows []map[string]interface{}

	// Header row (index 0), skip separator (index 1), then data rows
	for idx, line := range tableLines {
		if idx == 1 {
			continue // separator — skip
		}
		cells := splitTableRow(line)
		// Pad or trim to tableWidth
		for len(cells) < tableWidth {
			cells = append(cells, "")
		}
		cells = cells[:tableWidth]

		notionCells := make([]interface{}, tableWidth)
		for j, cellText := range cells {
			notionCells[j] = parseInlineFormatting(cellText)
		}
		rows = append(rows, map[string]interface{}{
			"object": "block",
			"type":   "table_row",
			"table_row": map[string]interface{}{
				"cells": notionCells,
			},
		})
	}

	if len(rows) == 0 {
		return nil
	}

	// Notion API requires table_row children INSIDE table{}, not at block top-level.
	return map[string]interface{}{
		"object": "block",
		"type":   "table",
		"table": map[string]interface{}{
			"table_width":       tableWidth,
			"has_column_header": true,
			"has_row_header":    false,
			"children":          rows,
		},
	}
}

func makeTextBlock(blockType, text string) map[string]interface{} {
	return map[string]interface{}{
		"object": "block",
		"type":   blockType,
		blockType: map[string]interface{}{
			"rich_text": parseInlineFormatting(strings.TrimSpace(text)),
		},
	}
}

// parseInlineFormatting converts inline markdown (bold, italic, code, link, strikethrough)
// into a Notion rich_text array.
func parseInlineFormatting(text string) []map[string]interface{} {
	// token pattern: **bold**, *italic*, _italic_, `code`, ~~strike~~, [text](url)
	tokenRe := regexp.MustCompile(`\*\*(.+?)\*\*|\*(.+?)\*|_(.+?)_|` + "`" + `(.+?)` + "`" + `|~~(.+?)~~|\[([^\]]+)\]\(([^)]+)\)`)

	var result []map[string]interface{}
	remaining := text
	for len(remaining) > 0 {
		loc := tokenRe.FindStringIndex(remaining)
		if loc == nil {
			// No more tokens — append remaining as plain text
			if remaining != "" {
				result = append(result, plainRichText(remaining))
			}
			break
		}
		// Plain text before the match
		if loc[0] > 0 {
			result = append(result, plainRichText(remaining[:loc[0]]))
		}
		match := tokenRe.FindStringSubmatch(remaining[loc[0]:loc[1]])
		rt := buildAnnotatedRichText(match)
		result = append(result, rt)
		remaining = remaining[loc[1]:]
	}
	if len(result) == 0 {
		return []map[string]interface{}{plainRichText(text)}
	}
	return result
}

func plainRichText(text string) map[string]interface{} {
	return map[string]interface{}{
		"text": map[string]interface{}{"content": text},
	}
}

func buildAnnotatedRichText(match []string) map[string]interface{} {
	// match[0] = full match
	// match[1] = **bold**  content
	// match[2] = *italic*  content
	// match[3] = _italic_  content
	// match[4] = `code`    content
	// match[5] = ~~strike~~ content
	// match[6] = [text](url) text part
	// match[7] = [text](url) url part
	switch {
	case match[1] != "": // **bold**
		return map[string]interface{}{
			"text":        map[string]interface{}{"content": match[1]},
			"annotations": map[string]interface{}{"bold": true},
		}
	case match[2] != "": // *italic*
		return map[string]interface{}{
			"text":        map[string]interface{}{"content": match[2]},
			"annotations": map[string]interface{}{"italic": true},
		}
	case match[3] != "": // _italic_
		return map[string]interface{}{
			"text":        map[string]interface{}{"content": match[3]},
			"annotations": map[string]interface{}{"italic": true},
		}
	case match[4] != "": // `code`
		return map[string]interface{}{
			"text":        map[string]interface{}{"content": match[4]},
			"annotations": map[string]interface{}{"code": true},
		}
	case match[5] != "": // ~~strike~~
		return map[string]interface{}{
			"text":        map[string]interface{}{"content": match[5]},
			"annotations": map[string]interface{}{"strikethrough": true},
		}
	case match[6] != "": // [text](url)
		return map[string]interface{}{
			"text": map[string]interface{}{
				"content": match[6],
				"link":    map[string]interface{}{"url": match[7]},
			},
		}
	default:
		return plainRichText(match[0])
	}
}

// richTextToMarkdown converts a Notion rich_text cell ([]interface{} of rich_text objects)
// into a plain markdown string, applying inline annotations.
func richTextToMarkdown(cell interface{}) string {
	items, ok := cell.([]interface{})
	if !ok {
		// Could be []map[string]interface{} from our own buildTableBlock
		if maps, ok := cell.([]map[string]interface{}); ok {
			var sb strings.Builder
			for _, m := range maps {
				sb.WriteString(richTextItemToMarkdown(m))
			}
			return sb.String()
		}
		return ""
	}
	var sb strings.Builder
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		sb.WriteString(richTextItemToMarkdown(m))
	}
	return sb.String()
}

func richTextItemToMarkdown(m map[string]interface{}) string {
	textObj, _ := m["text"].(map[string]interface{})
	content, _ := textObj["content"].(string)
	link, hasLink := textObj["link"].(map[string]interface{})

	ann, _ := m["annotations"].(map[string]interface{})
	bold, _ := ann["bold"].(bool)
	italic, _ := ann["italic"].(bool)
	code, _ := ann["code"].(bool)
	strike, _ := ann["strikethrough"].(bool)

	// plain_text fallback (from Notion API responses)
	if content == "" {
		content, _ = m["plain_text"].(string)
		// For API responses, check href for links
		if href, ok := m["href"].(string); ok && href != "" {
			return fmt.Sprintf("[%s](%s)", content, href)
		}
	}

	result := content
	if code {
		return "`" + result + "`"
	}
	if strike {
		result = "~~" + result + "~~"
	}
	if bold {
		result = "**" + result + "**"
	}
	if italic {
		result = "*" + result + "*"
	}
	if hasLink {
		url, _ := link["url"].(string)
		result = fmt.Sprintf("[%s](%s)", result, url)
	}
	return result
}

// renderBlockMarkdown outputs a block as clean Markdown.
func renderBlockMarkdown(block map[string]interface{}, indent int) {
	blockType, _ := block["type"].(string)
	prefix := strings.Repeat("  ", indent) // 2-space indent for nested blocks

	getText := func(key string) string {
		if data, ok := block[key].(map[string]interface{}); ok {
			if richText, ok := data["rich_text"].([]interface{}); ok {
				var parts []string
				for _, t := range richText {
					if m, ok := t.(map[string]interface{}); ok {
						if pt, ok := m["plain_text"].(string); ok {
							parts = append(parts, pt)
						}
					}
				}
				return strings.Join(parts, "")
			}
		}
		return ""
	}

	switch blockType {
	case "paragraph":
		text := getText("paragraph")
		if text != "" {
			fmt.Printf("%s%s\n\n", prefix, text)
		} else {
			fmt.Println()
		}
	case "heading_1":
		fmt.Printf("%s# %s\n\n", prefix, getText("heading_1"))
	case "heading_2":
		fmt.Printf("%s## %s\n\n", prefix, getText("heading_2"))
	case "heading_3":
		fmt.Printf("%s### %s\n\n", prefix, getText("heading_3"))
	case "bulleted_list_item":
		fmt.Printf("%s- %s\n", prefix, getText("bulleted_list_item"))
	case "numbered_list_item":
		fmt.Printf("%s1. %s\n", prefix, getText("numbered_list_item"))
	case "to_do":
		text := getText("to_do")
		data, _ := block["to_do"].(map[string]interface{})
		checked, _ := data["checked"].(bool)
		if checked {
			fmt.Printf("%s- [x] %s\n", prefix, text)
		} else {
			fmt.Printf("%s- [ ] %s\n", prefix, text)
		}
	case "toggle":
		fmt.Printf("%s- %s\n", prefix, getText("toggle"))
	case "code":
		data, _ := block["code"].(map[string]interface{})
		lang, _ := data["language"].(string)
		if lang == "plain text" {
			lang = ""
		}
		fmt.Printf("%s```%s\n%s\n%s```\n\n", prefix, lang, getText("code"), prefix)
	case "quote":
		fmt.Printf("%s> %s\n\n", prefix, getText("quote"))
	case "callout":
		data, _ := block["callout"].(map[string]interface{})
		icon := "💡"
		if iconObj, ok := data["icon"].(map[string]interface{}); ok {
			if emoji, ok := iconObj["emoji"].(string); ok {
				icon = emoji
			}
		}
		fmt.Printf("%s> %s %s\n\n", prefix, icon, getText("callout"))
	case "divider":
		fmt.Printf("%s---\n\n", prefix)
	case "bookmark":
		if data, ok := block["bookmark"].(map[string]interface{}); ok {
			url, _ := data["url"].(string)
			caption := ""
			if captions, ok := data["caption"].([]interface{}); ok && len(captions) > 0 {
				if m, ok := captions[0].(map[string]interface{}); ok {
					caption, _ = m["plain_text"].(string)
				}
			}
			if caption != "" {
				fmt.Printf("%s[%s](%s)\n\n", prefix, caption, url)
			} else {
				fmt.Printf("%s[%s](%s)\n\n", prefix, url, url)
			}
		}
	case "image":
		imageURL := ""
		if data, ok := block["image"].(map[string]interface{}); ok {
			if f, ok := data["file"].(map[string]interface{}); ok {
				imageURL, _ = f["url"].(string)
			} else if e, ok := data["external"].(map[string]interface{}); ok {
				imageURL, _ = e["url"].(string)
			}
		}
		if imageURL != "" {
			fmt.Printf("%s![image](%s)\n\n", prefix, imageURL)
		}
	case "embed":
		if data, ok := block["embed"].(map[string]interface{}); ok {
			url, _ := data["url"].(string)
			fmt.Printf("%s[embed](%s)\n\n", prefix, url)
		}
	case "video":
		videoURL := ""
		if data, ok := block["video"].(map[string]interface{}); ok {
			if f, ok := data["file"].(map[string]interface{}); ok {
				videoURL, _ = f["url"].(string)
			} else if e, ok := data["external"].(map[string]interface{}); ok {
				videoURL, _ = e["url"].(string)
			}
		}
		if videoURL != "" {
			fmt.Printf("%s[video](%s)\n\n", prefix, videoURL)
		}
	case "table_of_contents":
		fmt.Printf("%s[TOC]\n\n", prefix)
	case "equation":
		if data, ok := block["equation"].(map[string]interface{}); ok {
			expr, _ := data["expression"].(string)
			fmt.Printf("%s$$\n%s%s\n%s$$\n\n", prefix, prefix, expr, prefix)
		}
	case "table":
		// Table is rendered by iterating its _children (table_row blocks)
		// We reconstruct the GFM table including the separator after header row.
		tableData, _ := block["table"].(map[string]interface{})
		hasColHeader, _ := tableData["has_column_header"].(bool)
		children, _ := block["_children"].([]interface{})
		for rowIdx, child := range children {
			rowBlock, ok := child.(map[string]interface{})
			if !ok {
				continue
			}
			renderBlockMarkdown(rowBlock, indent)
			// Insert GFM separator after header row
			if rowIdx == 0 && hasColHeader {
				width := 0
				if rowData, ok := rowBlock["table_row"].(map[string]interface{}); ok {
					if cells, ok := rowData["cells"].([]interface{}); ok {
						width = len(cells)
					}
				}
				if width > 0 {
					sep := prefix + "|" + strings.Repeat("---|", width)
					fmt.Println(sep)
				}
			}
		}
		fmt.Println()
		return // children already handled above
	case "table_row":
		rowData, _ := block["table_row"].(map[string]interface{})
		cells, _ := rowData["cells"].([]interface{})
		var parts []string
		for _, cell := range cells {
			cellText := richTextToMarkdown(cell)
			parts = append(parts, cellText)
		}
		fmt.Printf("%s| %s |\n", prefix, strings.Join(parts, " | "))
		return
	case "column_list", "synced_block":
		// Container blocks — just render children
	default:
		text := getText(blockType)
		if text != "" {
			fmt.Printf("%s%s\n\n", prefix, text)
		}
	}

	// Recurse into children
	if children, ok := block["_children"].([]interface{}); ok {
		for _, child := range children {
			if childBlock, ok := child.(map[string]interface{}); ok {
				renderBlockMarkdown(childBlock, indent+1)
			}
		}
	}
}
