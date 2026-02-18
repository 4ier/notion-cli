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
			// Read file and parse markdown to blocks
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			children = parseMarkdownToBlocks(string(data))
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
			children = parseMarkdownToBlocks(string(data))
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
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parse response: %w", err)
			}
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
	blockListCmd.Flags().Int("depth", 1, "Depth of nested blocks to fetch (default 1)")
	blockListCmd.Flags().Bool("md", false, "Output as Markdown")
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
			lang := strings.TrimPrefix(line, "```")
			lang = strings.TrimSpace(lang)
			if lang == "" {
				lang = "plain text"
			}
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

		// Default: paragraph
		blocks = append(blocks, makeTextBlock("paragraph", line))
		i++
	}

	return blocks
}

func makeTextBlock(blockType, text string) map[string]interface{} {
	return map[string]interface{}{
		"object": "block",
		"type":   blockType,
		blockType: map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"text": map[string]interface{}{"content": strings.TrimSpace(text)}},
			},
		},
	}
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
