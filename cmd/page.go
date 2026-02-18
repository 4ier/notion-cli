package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/4ier/notion-cli/internal/client"
	"github.com/4ier/notion-cli/internal/render"
	"github.com/4ier/notion-cli/internal/util"
	"github.com/spf13/cobra"
)

// openURL opens a URL in the default browser across platforms.
func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

var pageCmd = &cobra.Command{
	Use:   "page",
	Short: "Work with Notion pages",
}

var pageViewCmd = &cobra.Command{
	Use:   "view <page-id|url>",
	Short: "View a page's content",
	Long: `Display a Notion page's content as readable text.

Examples:
  notion page view abc123
  notion page view https://notion.so/My-Page-abc123
  notion page view abc123 --format json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		pageID := util.ResolveID(args[0])
		c := client.New(token)
		c.SetDebug(debugMode)

		// Get page metadata
		page, err := c.GetPage(pageID)
		if err != nil {
			return fmt.Errorf("get page: %w", err)
		}

		// Get page blocks (content)
		blocks, err := c.GetBlockChildren(pageID, 100, "")
		if err != nil {
			return fmt.Errorf("get blocks: %w", err)
		}

		if outputFormat == "json" {
			combined := map[string]interface{}{
				"page":   page,
				"blocks": blocks,
			}
			return render.JSON(combined)
		}

		// Render blocks
		results, _ := blocks["results"].([]interface{})

		if outputFormat == "md" || outputFormat == "markdown" {
			// Pure markdown output
			title := render.ExtractTitle(page)
			fmt.Printf("# %s\n\n", title)
			for _, b := range results {
				block, ok := b.(map[string]interface{})
				if !ok {
					continue
				}
				renderBlockMarkdown(block, 0)
			}
			return nil
		}

		// Pretty print
		title := render.ExtractTitle(page)
		lastEdited, _ := page["last_edited_time"].(string)

		render.Title("📄", title)
		render.Separator()
		render.Subtitle(fmt.Sprintf("Last edited: %s", lastEdited))
		fmt.Println()

		for _, b := range results {
			block, ok := b.(map[string]interface{})
			if !ok {
				continue
			}
			renderBlock(block, 0)
		}

		return nil
	},
}

var pageListCmd = &cobra.Command{
	Use:   "list [parent-id]",
	Short: "List pages",
	Long: `List pages in the workspace or under a parent.

Examples:
  notion page list
  notion page list --limit 20`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")
		all, _ := cmd.Flags().GetBool("all")
		c := client.New(token)
		c.SetDebug(debugMode)

		var allResults []interface{}
		currentCursor := cursor

		for {
			result, err := c.Search("", "page", limit, currentCursor)
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

		headers := []string{"TITLE", "ID", "LAST EDITED"}
		var rows [][]string

		for _, r := range allResults {
			obj, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			title := render.ExtractTitle(obj)
			id, _ := obj["id"].(string)
			lastEdited, _ := obj["last_edited_time"].(string)
			if len(lastEdited) > 10 {
				lastEdited = lastEdited[:10]
			}
			rows = append(rows, []string{title, id, lastEdited})
		}

		render.Table(headers, rows)
		return nil
	},
}

var pageCreateCmd = &cobra.Command{
	Use:   "create <parent-id|url> [prop=value ...]",
	Short: "Create a new page",
	Long: `Create a new page under a parent page or database.

When creating under a database, provide properties as key=value arguments.
Property types are auto-detected from the database schema.

Examples:
  notion page create <page-id> --title "My New Page"
  notion page create <page-id> --title "Meeting Notes" --body "Agenda items..."
  notion page create <db-id> --db "Name=Sprint Review" "Status=Todo" "Date=2026-03-01"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		parentID := util.ResolveID(args[0])
		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")
		isDB, _ := cmd.Flags().GetBool("db")

		c := client.New(token)
		c.SetDebug(debugMode)

		var reqBody map[string]interface{}

		if isDB {
			// Database parent: auto-detect property types from schema
			db, err := c.GetDatabase(parentID)
			if err != nil {
				return fmt.Errorf("get database schema: %w", err)
			}
			dbProps, _ := db["properties"].(map[string]interface{})

			properties := map[string]interface{}{}

			// Parse key=value pairs from remaining args
			for _, kv := range args[1:] {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid property format %q, expected key=value", kv)
				}
				key, value := parts[0], parts[1]
				propDef, ok := dbProps[key].(map[string]interface{})
				if !ok {
					return fmt.Errorf("property %q not found in database schema", key)
				}
				propType, _ := propDef["type"].(string)
				properties[key] = buildPropertyValue(propType, value)
			}

			// If --title provided and there's a title property, set it
			if title != "" {
				for name, v := range dbProps {
					if prop, ok := v.(map[string]interface{}); ok {
						if pt, _ := prop["type"].(string); pt == "title" {
							properties[name] = buildPropertyValue("title", title)
							break
						}
					}
				}
			}

			reqBody = map[string]interface{}{
				"parent": map[string]interface{}{
					"database_id": parentID,
				},
				"properties": properties,
			}
		} else {
			// Page parent
			if title == "" {
				return fmt.Errorf("--title is required")
			}

			reqBody = map[string]interface{}{
				"parent": map[string]interface{}{
					"page_id": parentID,
				},
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"title": []map[string]interface{}{
							{"text": map[string]interface{}{"content": title}},
						},
					},
				},
			}
		}

		// Add body content if provided
		if body != "" {
			reqBody["children"] = []map[string]interface{}{
				{
					"object": "block",
					"type":   "paragraph",
					"paragraph": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{"text": map[string]interface{}{"content": body}},
						},
					},
				},
			}
		}

		data, err := c.Post("/v1/pages", reqBody)
		if err != nil {
			return fmt.Errorf("create page: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				return err
			}
			return render.JSON(result)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return err
		}

		id, _ := result["id"].(string)
		url, _ := result["url"].(string)

		displayTitle := title
		if displayTitle == "" {
			displayTitle = "New row"
		}
		render.Title("✓", fmt.Sprintf("Created: %s", displayTitle))
		render.Field("ID", id)
		if url != "" {
			render.Field("URL", url)
		}

		return nil
	},
}

var pageDeleteCmd = &cobra.Command{
	Use:   "delete <page-id|url>",
	Short: "Delete (archive) a page",
	Long: `Archive a Notion page (soft delete).

Examples:
  notion page delete abc123
  notion page delete https://notion.so/My-Page-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		pageID := util.ResolveID(args[0])
		c := client.New(token)
		c.SetDebug(debugMode)

		body := map[string]interface{}{
			"archived": true,
		}

		data, err := c.Patch("/v1/pages/"+pageID, body)
		if err != nil {
			return fmt.Errorf("delete page: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
			return render.JSON(result)
		}

		fmt.Println("✓ Page archived")
		return nil
	},
}

var pageMoveCmd = &cobra.Command{
	Use:   "move <page-id|url>",
	Short: "Move a page to a new parent",
	Long: `Move a page under a different parent.

Examples:
  notion page move abc123 --to def456`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		pageID := util.ResolveID(args[0])
		to, _ := cmd.Flags().GetString("to")
		if to == "" {
			return fmt.Errorf("--to flag is required")
		}
		toID := util.ResolveID(to)

		c := client.New(token)
		c.SetDebug(debugMode)

		body := map[string]interface{}{
			"parent": map[string]interface{}{
				"page_id": toID,
			},
		}

		data, err := c.Post(fmt.Sprintf("/v1/pages/%s/move", pageID), body)
		if err != nil {
			return fmt.Errorf("move page: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
			return render.JSON(result)
		}

		fmt.Printf("✓ Page moved to %s\n", toID)
		return nil
	},
}

var pageOpenCmd = &cobra.Command{
	Use:   "open <page-id|url>",
	Short: "Open a page in the browser",
	Long: `Open a Notion page in your default browser.

Examples:
  notion page open abc123
  notion page open https://notion.so/My-Page-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input := args[0]

		// If it's already a URL, use it directly
		var url string
		if strings.Contains(input, "notion.so") || strings.Contains(input, "notion.site") {
			url = input
		} else {
			pageID := util.ResolveID(input)
			url = "https://www.notion.so/" + strings.ReplaceAll(pageID, "-", "")
		}

		return openBrowser(url)
	},
}

var pageSetCmd = &cobra.Command{
	Use:   "set <page-id|url> <key=value ...>",
	Short: "Set page properties",
	Long: `Set one or more properties on a page using key=value syntax.

The CLI will fetch the page schema to determine property types automatically.

Examples:
  notion page set abc123 Status=Done
  notion page set abc123 Status=Done Priority=High
  notion page set abc123 "Name=My New Title"`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		pageID := util.ResolveID(args[0])
		c := client.New(token)
		c.SetDebug(debugMode)

		// Get the page to determine property types
		page, err := c.GetPage(pageID)
		if err != nil {
			return fmt.Errorf("get page: %w", err)
		}

		existingProps, _ := page["properties"].(map[string]interface{})

		// Parse key=value pairs
		properties := map[string]interface{}{}
		for _, kv := range args[1:] {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid property format %q, expected key=value", kv)
			}
			key, value := parts[0], parts[1]

			// Look up property type from existing properties
			propDef, ok := existingProps[key].(map[string]interface{})
			if !ok {
				return fmt.Errorf("property %q not found on page", key)
			}
			propType, _ := propDef["type"].(string)
			properties[key] = buildPropertyValue(propType, value)
		}

		body := map[string]interface{}{
			"properties": properties,
		}

		data, err := c.Patch("/v1/pages/"+pageID, body)
		if err != nil {
			return fmt.Errorf("set properties: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
			return render.JSON(result)
		}

		fmt.Println("✓ Properties updated")
		return nil
	},
}

var pagePropsCmd = &cobra.Command{
	Use:   "props <page-id|url> [property-id]",
	Short: "Show page properties",
	Long: `Show all properties of a page, or retrieve a specific property value.

Examples:
  notion page props abc123
  notion page props abc123 title`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		pageID := util.ResolveID(args[0])
		c := client.New(token)
		c.SetDebug(debugMode)

		if len(args) == 2 {
			// Get specific property
			propID := args[1]
			data, err := c.Get(fmt.Sprintf("/v1/pages/%s/properties/%s", pageID, propID))
			if err != nil {
				return fmt.Errorf("get property: %w", err)
			}
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
			return render.JSON(result)
		}

		// Get all properties from page
		page, err := c.GetPage(pageID)
		if err != nil {
			return fmt.Errorf("get page: %w", err)
		}

		if outputFormat == "json" {
			props := page["properties"]
			return render.JSON(props)
		}

		title := render.ExtractTitle(page)
		render.Title("📄", title)
		render.Separator()

		props, _ := page["properties"].(map[string]interface{})
		for name, v := range props {
			prop, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			propType, _ := prop["type"].(string)
			value := extractPropertyValue(prop)
			render.Field(name, fmt.Sprintf("%s (%s)", value, propType))
		}

		return nil
	},
}

var pageRestoreCmd = &cobra.Command{
	Use:   "restore <page-id|url>",
	Short: "Restore an archived page",
	Long: `Unarchive a Notion page (reverse of delete).

Examples:
  notion page restore abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		pageID := util.ResolveID(args[0])
		c := client.New(token)
		c.SetDebug(debugMode)

		body := map[string]interface{}{
			"archived": false,
		}

		data, err := c.Patch("/v1/pages/"+pageID, body)
		if err != nil {
			return fmt.Errorf("restore page: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
			return render.JSON(result)
		}

		fmt.Println("✓ Page restored")
		return nil
	},
}

var pageLinkCmd = &cobra.Command{
	Use:   "link <page-id|url>",
	Short: "Link a page via a relation property",
	Long: `Add a relation link between two pages.

Examples:
  notion page link abc123 --prop "Project" --to def456`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		pageID := util.ResolveID(args[0])
		propName, _ := cmd.Flags().GetString("prop")
		toID, _ := cmd.Flags().GetString("to")

		if propName == "" {
			return fmt.Errorf("--prop is required")
		}
		if toID == "" {
			return fmt.Errorf("--to is required")
		}
		toID = util.ResolveID(toID)

		c := client.New(token)
		c.SetDebug(debugMode)

		// Get current page to read existing relations
		page, err := c.GetPage(pageID)
		if err != nil {
			return fmt.Errorf("get page: %w", err)
		}

		props, _ := page["properties"].(map[string]interface{})
		propData, ok := props[propName].(map[string]interface{})
		if !ok {
			return fmt.Errorf("property %q not found", propName)
		}

		// Build new relation list (existing + new)
		var relations []map[string]interface{}
		if existing, ok := propData["relation"].([]interface{}); ok {
			for _, r := range existing {
				if m, ok := r.(map[string]interface{}); ok {
					relations = append(relations, m)
				}
			}
		}
		relations = append(relations, map[string]interface{}{"id": toID})

		body := map[string]interface{}{
			"properties": map[string]interface{}{
				propName: map[string]interface{}{
					"relation": relations,
				},
			},
		}

		data, err := c.Patch("/v1/pages/"+pageID, body)
		if err != nil {
			return fmt.Errorf("link page: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
			return render.JSON(result)
		}

		fmt.Println("✓ Relation added")
		return nil
	},
}

var pageUnlinkCmd = &cobra.Command{
	Use:   "unlink <page-id|url>",
	Short: "Remove a relation link from a page",
	Long: `Remove a relation link between two pages.

Examples:
  notion page unlink abc123 --prop "Project" --from def456`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		pageID := util.ResolveID(args[0])
		propName, _ := cmd.Flags().GetString("prop")
		fromID, _ := cmd.Flags().GetString("from")

		if propName == "" {
			return fmt.Errorf("--prop is required")
		}
		if fromID == "" {
			return fmt.Errorf("--from is required")
		}
		fromID = util.ResolveID(fromID)

		c := client.New(token)
		c.SetDebug(debugMode)

		// Get current page to read existing relations
		page, err := c.GetPage(pageID)
		if err != nil {
			return fmt.Errorf("get page: %w", err)
		}

		props, _ := page["properties"].(map[string]interface{})
		propData, ok := props[propName].(map[string]interface{})
		if !ok {
			return fmt.Errorf("property %q not found", propName)
		}

		// Build new relation list without the target
		var relations []map[string]interface{}
		if existing, ok := propData["relation"].([]interface{}); ok {
			for _, r := range existing {
				if m, ok := r.(map[string]interface{}); ok {
					id, _ := m["id"].(string)
					if id != fromID {
						relations = append(relations, m)
					}
				}
			}
		}

		body := map[string]interface{}{
			"properties": map[string]interface{}{
				propName: map[string]interface{}{
					"relation": relations,
				},
			},
		}

		data, err := c.Patch("/v1/pages/"+pageID, body)
		if err != nil {
			return fmt.Errorf("unlink page: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
			return render.JSON(result)
		}

		fmt.Println("✓ Relation removed")
		return nil
	},
}

func init() {
	pageListCmd.Flags().IntP("limit", "l", 10, "Maximum results")
	pageListCmd.Flags().String("cursor", "", "Pagination cursor")
	pageListCmd.Flags().Bool("all", false, "Fetch all pages of results")
	pageCreateCmd.Flags().String("title", "", "Page title (required for page parent)")
	pageCreateCmd.Flags().String("body", "", "Page body text")
	pageCreateCmd.Flags().Bool("db", false, "Create under a database (properties as key=value args)")
	pageMoveCmd.Flags().String("to", "", "Target parent page/database ID or URL (required)")
	pageLinkCmd.Flags().String("prop", "", "Relation property name (required)")
	pageLinkCmd.Flags().String("to", "", "Target page ID or URL to link (required)")
	pageUnlinkCmd.Flags().String("prop", "", "Relation property name (required)")
	pageUnlinkCmd.Flags().String("from", "", "Target page ID or URL to unlink (required)")

	pageCmd.AddCommand(pageViewCmd)
	pageCmd.AddCommand(pageListCmd)
	pageCmd.AddCommand(pageCreateCmd)
	pageCmd.AddCommand(pageDeleteCmd)
	pageCmd.AddCommand(pageRestoreCmd)
	pageCmd.AddCommand(pageMoveCmd)
	pageCmd.AddCommand(pageOpenCmd)
	pageCmd.AddCommand(pageSetCmd)
	pageCmd.AddCommand(pagePropsCmd)
	pageCmd.AddCommand(pageLinkCmd)
	pageCmd.AddCommand(pageUnlinkCmd)
}

// openBrowser opens a URL in the default browser.
func openBrowser(url string) error {
	return openURL(url)
}

// buildPropertyValue converts a string value to a Notion property value based on type.
func buildPropertyValue(propType, value string) interface{} {
	switch propType {
	case "title":
		return map[string]interface{}{
			"title": []map[string]interface{}{
				{"text": map[string]interface{}{"content": value}},
			},
		}
	case "rich_text":
		return map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"text": map[string]interface{}{"content": value}},
			},
		}
	case "number":
		// Try to parse as number
		var n json.Number = json.Number(value)
		if f, err := n.Float64(); err == nil {
			return map[string]interface{}{"number": f}
		}
		return map[string]interface{}{"number": value}
	case "select":
		return map[string]interface{}{
			"select": map[string]interface{}{"name": value},
		}
	case "multi_select":
		names := strings.Split(value, ",")
		options := []map[string]interface{}{}
		for _, n := range names {
			options = append(options, map[string]interface{}{"name": strings.TrimSpace(n)})
		}
		return map[string]interface{}{"multi_select": options}
	case "status":
		return map[string]interface{}{
			"status": map[string]interface{}{"name": value},
		}
	case "date":
		return map[string]interface{}{
			"date": map[string]interface{}{"start": value},
		}
	case "checkbox":
		return map[string]interface{}{
			"checkbox": value == "true" || value == "1" || value == "yes",
		}
	case "url":
		return map[string]interface{}{"url": value}
	case "email":
		return map[string]interface{}{"email": value}
	case "phone_number":
		return map[string]interface{}{"phone_number": value}
	default:
		// Fallback: try as rich_text
		return map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"text": map[string]interface{}{"content": value}},
			},
		}
	}
}

// extractPropertyValue extracts a human-readable value from a Notion property.
func extractPropertyValue(prop map[string]interface{}) string {
	propType, _ := prop["type"].(string)

	switch propType {
	case "title":
		if arr, ok := prop["title"].([]interface{}); ok {
			return extractPlainTextFromRichText(arr)
		}
	case "rich_text":
		if arr, ok := prop["rich_text"].([]interface{}); ok {
			return extractPlainTextFromRichText(arr)
		}
	case "number":
		if n, ok := prop["number"]; ok && n != nil {
			return fmt.Sprintf("%v", n)
		}
	case "select":
		if sel, ok := prop["select"].(map[string]interface{}); ok {
			name, _ := sel["name"].(string)
			return name
		}
	case "multi_select":
		if arr, ok := prop["multi_select"].([]interface{}); ok {
			var names []string
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					n, _ := m["name"].(string)
					names = append(names, n)
				}
			}
			return strings.Join(names, ", ")
		}
	case "status":
		if s, ok := prop["status"].(map[string]interface{}); ok {
			name, _ := s["name"].(string)
			return name
		}
	case "date":
		if d, ok := prop["date"].(map[string]interface{}); ok {
			start, _ := d["start"].(string)
			end, _ := d["end"].(string)
			if end != "" {
				return start + " → " + end
			}
			return start
		}
	case "checkbox":
		if b, ok := prop["checkbox"].(bool); ok {
			if b {
				return "✓"
			}
			return "✗"
		}
	case "url":
		if u, ok := prop["url"].(string); ok {
			return u
		}
	case "email":
		if e, ok := prop["email"].(string); ok {
			return e
		}
	case "phone_number":
		if p, ok := prop["phone_number"].(string); ok {
			return p
		}
	case "people":
		if arr, ok := prop["people"].([]interface{}); ok {
			var names []string
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					n, _ := m["name"].(string)
					names = append(names, n)
				}
			}
			return strings.Join(names, ", ")
		}
	case "relation":
		if arr, ok := prop["relation"].([]interface{}); ok {
			var ids []string
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					id, _ := m["id"].(string)
					ids = append(ids, id)
				}
			}
			return strings.Join(ids, ", ")
		}
	case "formula":
		if f, ok := prop["formula"].(map[string]interface{}); ok {
			fType, _ := f["type"].(string)
			if v, ok := f[fType]; ok {
				return fmt.Sprintf("%v", v)
			}
		}
	case "rollup":
		if r, ok := prop["rollup"].(map[string]interface{}); ok {
			rType, _ := r["type"].(string)
			if v, ok := r[rType]; ok {
				return fmt.Sprintf("%v", v)
			}
		}
	case "created_time":
		if t, ok := prop["created_time"].(string); ok {
			return t
		}
	case "last_edited_time":
		if t, ok := prop["last_edited_time"].(string); ok {
			return t
		}
	case "created_by", "last_edited_by":
		if u, ok := prop[propType].(map[string]interface{}); ok {
			name, _ := u["name"].(string)
			return name
		}
	}
	return ""
}

func extractPlainTextFromRichText(arr []interface{}) string {
	var parts []string
	for _, t := range arr {
		if m, ok := t.(map[string]interface{}); ok {
			if pt, ok := m["plain_text"].(string); ok {
				parts = append(parts, pt)
			}
		}
	}
	return strings.Join(parts, "")
}

// renderBlock renders a single Notion block to stdout.
func renderBlock(block map[string]interface{}, indent int) {
	blockType, _ := block["type"].(string)
	prefix := strings.Repeat("  ", indent)

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
			fmt.Printf("%s%s\n", prefix, text)
		} else {
			fmt.Println()
		}
	case "heading_1":
		text := getText("heading_1")
		fmt.Printf("%s# %s\n", prefix, text)
	case "heading_2":
		text := getText("heading_2")
		fmt.Printf("%s## %s\n", prefix, text)
	case "heading_3":
		text := getText("heading_3")
		fmt.Printf("%s### %s\n", prefix, text)
	case "bulleted_list_item":
		text := getText("bulleted_list_item")
		fmt.Printf("%s• %s\n", prefix, text)
	case "numbered_list_item":
		text := getText("numbered_list_item")
		fmt.Printf("%s  %s\n", prefix, text)
	case "to_do":
		text := getText("to_do")
		data, _ := block["to_do"].(map[string]interface{})
		checked, _ := data["checked"].(bool)
		mark := "☐"
		if checked {
			mark = "☑"
		}
		fmt.Printf("%s%s %s\n", prefix, mark, text)
	case "toggle":
		text := getText("toggle")
		fmt.Printf("%s▸ %s\n", prefix, text)
	case "code":
		data, _ := block["code"].(map[string]interface{})
		lang, _ := data["language"].(string)
		text := getText("code")
		fmt.Printf("%s```%s\n%s%s\n%s```\n", prefix, lang, prefix, text, prefix)
	case "quote":
		text := getText("quote")
		fmt.Printf("%s│ %s\n", prefix, text)
	case "callout":
		text := getText("callout")
		fmt.Printf("%s💡 %s\n", prefix, text)
	case "divider":
		fmt.Printf("%s───\n", prefix)
	case "bookmark":
		if data, ok := block["bookmark"].(map[string]interface{}); ok {
			url, _ := data["url"].(string)
			fmt.Printf("%s🔗 %s\n", prefix, url)
		}
	case "image":
		fmt.Printf("%s🖼  [image]\n", prefix)
	default:
		text := getText(blockType)
		if text != "" {
			fmt.Printf("%s%s\n", prefix, text)
		}
	}
}
