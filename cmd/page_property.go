package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/4ier/notion-cli/internal/client"
	"github.com/4ier/notion-cli/internal/render"
	"github.com/4ier/notion-cli/internal/util"
	"github.com/spf13/cobra"
)

// pagePropertyCmd wraps GET /v1/pages/:id/properties/:property_id and
// fully paginates the results, which the page-level endpoint silently
// truncates at 25 items. This is the correctness fix for #38.
//
// This is distinct from `page props` which prints a one-line summary of
// every property (and inherits the same 25-item truncation from
// GET /v1/pages/:id). `page property` dives into a single property and
// returns the full value.
var pagePropertyCmd = &cobra.Command{
	Use:   "property <page-id|url> [property-id]",
	Short: "Retrieve a single property value (auto-paginates relation / rollup / rich_text)",
	Long: `Retrieve a single page property, following pagination cursors until
every item has been fetched.

This is the correct way to read properties that may have >25 items:
  - relation lists
  - rollup arrays
  - long rich_text / title values

The property id is usually visible in 'notion db view <database-id>'
(schema) or 'notion page props <page-id>' (per-property JSON). Prefer
--name when you only know the human-readable property name.

Examples:
  notion page property <page-id> <property-id>
  notion page property <page-id> --name "References"
  notion page property <page-id> <property-id> --format json
  notion page property <page-id> <property-id> --page-size 50`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		pageID := util.ResolveID(args[0])
		name, _ := cmd.Flags().GetString("name")
		pageSize, _ := cmd.Flags().GetInt("page-size")
		if pageSize < 1 || pageSize > 100 {
			return fmt.Errorf("--page-size must be between 1 and 100")
		}

		var propID string
		if len(args) == 2 {
			propID = args[1]
		}
		if propID == "" && name == "" {
			return fmt.Errorf("provide a property-id positional arg or use --name <property-name>")
		}
		if propID != "" && name != "" {
			return fmt.Errorf("pass either a property-id positional arg OR --name, not both")
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		// Resolve --name to an id by looking at the page's property map.
		if name != "" {
			page, err := c.GetPage(pageID)
			if err != nil {
				return fmt.Errorf("get page: %w", err)
			}
			propID, err = findPropertyIDByName(page, name)
			if err != nil {
				return err
			}
		}

		result, err := fetchPagePropertyAllPages(c, pageID, propID, pageSize)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			return render.JSON(result)
		}

		renderPageProperty(result)
		return nil
	},
}

// fetchPagePropertyAllPages walks every page of a page-property response
// and returns a single merged object. For non-paginated property types
// (title / number / select / ...) a single request is enough.
func fetchPagePropertyAllPages(c *client.Client, pageID, propID string, pageSize int) (map[string]interface{}, error) {
	basePath := fmt.Sprintf("/v1/pages/%s/properties/%s", pageID, propID)
	var merged map[string]interface{}
	var allResults []interface{}
	cursor := ""

	for {
		path := fmt.Sprintf("%s?page_size=%d", basePath, pageSize)
		if cursor != "" {
			path += "&start_cursor=" + url.QueryEscape(cursor)
		}
		data, err := c.Get(path)
		if err != nil {
			return nil, fmt.Errorf("get property: %w", err)
		}
		var page map[string]interface{}
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, fmt.Errorf("parse property response: %w", err)
		}

		// Non-paginated property types have `object: "property_item"` at the
		// top level — no results[] / has_more. Return as-is.
		if page["object"] == "property_item" {
			return page, nil
		}

		// Paginated: object is "list" with results + has_more + next_cursor.
		if merged == nil {
			merged = page
		}
		results, _ := page["results"].([]interface{})
		allResults = append(allResults, results...)

		hasMore, _ := page["has_more"].(bool)
		if !hasMore {
			break
		}
		nextCursor, _ := page["next_cursor"].(string)
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	if merged != nil {
		merged["results"] = allResults
		merged["has_more"] = false
		merged["next_cursor"] = nil
	}
	return merged, nil
}

// findPropertyIDByName scans page.properties[] for a key matching name and
// returns its id.
func findPropertyIDByName(page map[string]interface{}, name string) (string, error) {
	props, ok := page["properties"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("page has no properties map (is it a database row?)")
	}
	if prop, ok := props[name].(map[string]interface{}); ok {
		if id, _ := prop["id"].(string); id != "" {
			return id, nil
		}
	}
	// Build a sorted list for the error message.
	var names []string
	for k := range props {
		names = append(names, k)
	}
	return "", fmt.Errorf("no property named %q on this page; available: %v", name, names)
}

// renderPageProperty prints a friendly summary of a property fetch result.
// Both shapes (single property_item, paginated list) land here.
func renderPageProperty(result map[string]interface{}) {
	obj, _ := result["object"].(string)
	propType, _ := result["type"].(string)
	if propType == "" {
		// Paginated list: grab type from first result.
		if results, ok := result["results"].([]interface{}); ok && len(results) > 0 {
			if first, ok := results[0].(map[string]interface{}); ok {
				propType, _ = first["type"].(string)
			}
		}
	}

	render.Title("📋", fmt.Sprintf("Property (%s)", propType))

	if obj == "property_item" {
		// Single-value: print the extracted value directly.
		value := extractPropertyValue(result)
		render.Field("Value", value)
		return
	}

	// Paginated list.
	results, _ := result["results"].([]interface{})
	render.Field("Total items", fmt.Sprintf("%d", len(results)))
	for i, r := range results {
		item, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		summary := summarizePropertyItem(item)
		render.Field(fmt.Sprintf("  [%d]", i), summary)
	}
}

// summarizePropertyItem produces a single-line description of one element
// of a paginated property list (a relation ref, rollup item, rich_text chunk).
func summarizePropertyItem(item map[string]interface{}) string {
	t, _ := item["type"].(string)
	switch t {
	case "relation":
		rel, _ := item["relation"].(map[string]interface{})
		if id, _ := rel["id"].(string); id != "" {
			return id
		}
	case "rich_text", "title":
		if inner, ok := item[t].(map[string]interface{}); ok {
			if pt, _ := inner["plain_text"].(string); pt != "" {
				return pt
			}
		}
	case "people":
		if inner, ok := item["people"].(map[string]interface{}); ok {
			if id, _ := inner["id"].(string); id != "" {
				return id
			}
		}
	case "number":
		if n, ok := item["number"].(float64); ok {
			return fmt.Sprintf("%v", n)
		}
	}
	// Fallback: compact JSON.
	data, _ := json.Marshal(item)
	return string(data)
}
