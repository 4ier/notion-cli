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

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Work with databases",
}

var dbListCmd = &cobra.Command{
	Use:   "list",
	Short: "List accessible databases",
	Long: `List all databases you have access to.

Examples:
  notion db list
  notion db list --limit 20
  notion db list --format json`,
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
			result, err := c.Search("", "database", limit, currentCursor)
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

var dbViewCmd = &cobra.Command{
	Use:   "view <db-id|url>",
	Short: "Show database schema",
	Long: `Display the schema (columns/fields) of a database.

Examples:
  notion db view abc123
  notion db view https://notion.so/abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		dbID := util.ResolveID(args[0])
		c := client.New(token)
		c.SetDebug(debugMode)

		db, err := c.GetDatabase(dbID)
		if err != nil {
			return fmt.Errorf("get database: %w", err)
		}

		if outputFormat == "json" {
			return render.JSON(db)
		}

		title := render.ExtractTitle(db)
		render.Title("🗃️", title)
		render.Separator()

		id, _ := db["id"].(string)
		render.Field("ID", id)
		url, _ := db["url"].(string)
		if url != "" {
			render.Field("URL", url)
		}
		fmt.Println()

		// Show schema
		props, _ := db["properties"].(map[string]interface{})
		if len(props) > 0 {
			headers := []string{"PROPERTY", "TYPE", "OPTIONS"}
			var rows [][]string

			for name, v := range props {
				prop, ok := v.(map[string]interface{})
				if !ok {
					continue
				}
				propType, _ := prop["type"].(string)
				options := extractSchemaOptions(prop, propType)
				rows = append(rows, []string{name, propType, options})
			}
			render.Table(headers, rows)
		}

		return nil
	},
}

var dbCreateCmd = &cobra.Command{
	Use:   "create <parent-id|url>",
	Short: "Create a new database",
	Long: `Create a database under a parent page.

Examples:
  notion db create <parent-id> --title "Task Tracker"
  notion db create <parent-id> --title "Tasks" --props "Status:select,Priority:select,Date:date"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		parentID := util.ResolveID(args[0])
		title, _ := cmd.Flags().GetString("title")
		propsFlag, _ := cmd.Flags().GetString("props")

		if title == "" {
			return fmt.Errorf("--title is required")
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		// Build properties
		properties := map[string]interface{}{
			"Name": map[string]interface{}{
				"title": map[string]interface{}{},
			},
		}

		// Parse additional properties from --props flag
		if propsFlag != "" {
			for _, p := range strings.Split(propsFlag, ",") {
				parts := strings.SplitN(strings.TrimSpace(p), ":", 2)
				if len(parts) != 2 {
					continue
				}
				name, pType := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
				properties[name] = map[string]interface{}{
					pType: map[string]interface{}{},
				}
			}
		}

		body := map[string]interface{}{
			"parent": map[string]interface{}{
				"page_id": parentID,
			},
			"title": []map[string]interface{}{
				{"text": map[string]interface{}{"content": title}},
			},
			"properties": properties,
		}

		data, err := c.Post("/v1/databases", body)
		if err != nil {
			return fmt.Errorf("create database: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			json.Unmarshal(data, &result)
			return render.JSON(result)
		}

		var result map[string]interface{}
		json.Unmarshal(data, &result)
		id, _ := result["id"].(string)
		url, _ := result["url"].(string)

		render.Title("✓", fmt.Sprintf("Created database: %s", title))
		render.Field("ID", id)
		if url != "" {
			render.Field("URL", url)
		}

		return nil
	},
}

var dbUpdateCmd = &cobra.Command{
	Use:   "update <db-id|url>",
	Short: "Update a database",
	Long: `Update a database title or add/modify properties.

Examples:
  notion db update abc123 --title "New Title"
  notion db update abc123 --add-prop "Priority:select"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		dbID := util.ResolveID(args[0])
		title, _ := cmd.Flags().GetString("title")
		addProp, _ := cmd.Flags().GetString("add-prop")

		c := client.New(token)
		c.SetDebug(debugMode)

		body := map[string]interface{}{}

		if title != "" {
			body["title"] = []map[string]interface{}{
				{"text": map[string]interface{}{"content": title}},
			}
		}

		if addProp != "" {
			properties := map[string]interface{}{}
			for _, p := range strings.Split(addProp, ",") {
				parts := strings.SplitN(strings.TrimSpace(p), ":", 2)
				if len(parts) != 2 {
					continue
				}
				name, pType := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
				properties[name] = map[string]interface{}{
					pType: map[string]interface{}{},
				}
			}
			body["properties"] = properties
		}

		if len(body) == 0 {
			return fmt.Errorf("nothing to update. Specify --title or --add-prop")
		}

		data, err := c.Patch("/v1/databases/"+dbID, body)
		if err != nil {
			return fmt.Errorf("update database: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			json.Unmarshal(data, &result)
			return render.JSON(result)
		}

		fmt.Println("✓ Database updated")
		return nil
	},
}

var dbAddCmd = &cobra.Command{
	Use:   "add <db-id|url> <prop=value ...>",
	Short: "Add a row to a database",
	Long: `Add a new row (page) to a database with property values.

The CLI fetches the database schema to determine property types automatically.

Examples:
  notion db add abc123 "Name=My Task" "Status=Todo"
  notion db add abc123 "Name=Meeting" "Date=2026-03-01" "Priority=High"`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		dbID := util.ResolveID(args[0])
		c := client.New(token)
		c.SetDebug(debugMode)

		// Get database schema to determine property types
		db, err := c.GetDatabase(dbID)
		if err != nil {
			return fmt.Errorf("get database schema: %w", err)
		}

		dbProps, _ := db["properties"].(map[string]interface{})

		// Parse key=value pairs
		properties := map[string]interface{}{}
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

		body := map[string]interface{}{
			"parent": map[string]interface{}{
				"database_id": dbID,
			},
			"properties": properties,
		}

		data, err := c.Post("/v1/pages", body)
		if err != nil {
			return fmt.Errorf("add row: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			json.Unmarshal(data, &result)
			return render.JSON(result)
		}

		var result map[string]interface{}
		json.Unmarshal(data, &result)
		id, _ := result["id"].(string)
		url, _ := result["url"].(string)

		render.Title("✓", "Row added")
		render.Field("ID", id)
		if url != "" {
			render.Field("URL", url)
		}

		return nil
	},
}

var dbQueryCmd = &cobra.Command{
	Use:   "query <db-id|url>",
	Short: "Query a database with filters and sorts",
	Long: `Query a database with optional filters and sorting.

Filter syntax: property operator value
Operators: = != > >= < <= ~= (contains)

Sort syntax: property:direction (asc or desc)

Examples:
  notion db query abc123
  notion db query abc123 --filter 'Status=Done'
  notion db query abc123 --filter 'Date>=2026-01-01' --sort 'Date:desc'
  notion db query abc123 --filter 'Status=Done' --filter 'Priority=High'
  notion db query abc123 --limit 5`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		dbID := util.ResolveID(args[0])
		filters, _ := cmd.Flags().GetStringArray("filter")
		sorts, _ := cmd.Flags().GetStringArray("sort")
		limit, _ := cmd.Flags().GetInt("limit")

		c := client.New(token)
		c.SetDebug(debugMode)

		// Get database schema to determine property types
		db, err := c.GetDatabase(dbID)
		if err != nil {
			return fmt.Errorf("get database schema: %w", err)
		}
		dbProps, _ := db["properties"].(map[string]interface{})

		body := map[string]interface{}{}

		// Parse filters
		if len(filters) > 0 {
			filterConditions := []interface{}{}
			for _, f := range filters {
				condition, err := parseFilter(f, dbProps)
				if err != nil {
					return fmt.Errorf("invalid filter %q: %w", f, err)
				}
				filterConditions = append(filterConditions, condition)
			}

			if len(filterConditions) == 1 {
				body["filter"] = filterConditions[0]
			} else {
				body["filter"] = map[string]interface{}{
					"and": filterConditions,
				}
			}
		}

		// Parse sorts
		if len(sorts) > 0 {
			sortList := []interface{}{}
			for _, s := range sorts {
				sort := parseSort(s)
				sortList = append(sortList, sort)
			}
			body["sorts"] = sortList
		}

		if limit > 0 {
			body["page_size"] = limit
		}

		result, err := c.QueryDatabase(dbID, body)
		if err != nil {
			return fmt.Errorf("query database: %w", err)
		}

		if outputFormat == "json" {
			return render.JSON(result)
		}

		// Build table from results
		results, _ := result["results"].([]interface{})
		if len(results) == 0 {
			fmt.Println("No results found.")
			return nil
		}

		// Collect all property names from schema for column headers
		propNames := []string{}
		propTypes := map[string]string{}
		for name, v := range dbProps {
			prop, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			propType, _ := prop["type"].(string)
			propNames = append(propNames, name)
			propTypes[name] = propType
		}

		// Sort: put title first
		sortedNames := []string{}
		for _, n := range propNames {
			if propTypes[n] == "title" {
				sortedNames = append([]string{n}, sortedNames...)
			} else {
				sortedNames = append(sortedNames, n)
			}
		}

		headers := make([]string, len(sortedNames))
		for i, n := range sortedNames {
			headers[i] = n
		}

		var rows [][]string
		for _, r := range results {
			page, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			pageProps, _ := page["properties"].(map[string]interface{})

			row := make([]string, len(sortedNames))
			for i, name := range sortedNames {
				if prop, ok := pageProps[name].(map[string]interface{}); ok {
					row[i] = extractPropertyValue(prop)
				}
			}
			rows = append(rows, row)
		}

		render.Table(headers, rows)

		// Show pagination info
		hasMore, _ := result["has_more"].(bool)
		if hasMore {
			nextCursor, _ := result["next_cursor"].(string)
			render.Subtitle(fmt.Sprintf("\nMore results available. Use --cursor %s", nextCursor))
		}

		return nil
	},
}

var dbOpenCmd = &cobra.Command{
	Use:   "open <db-id|url>",
	Short: "Open a database in the browser",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input := args[0]
		var url string
		if strings.Contains(input, "notion.so") || strings.Contains(input, "notion.site") {
			url = input
		} else {
			dbID := util.ResolveID(input)
			url = "https://www.notion.so/" + strings.ReplaceAll(dbID, "-", "")
		}
		return openURL(url)
	},
}

var dbAddBulkCmd = &cobra.Command{
	Use:   "add-bulk <db-id|url>",
	Short: "Bulk add rows from a JSON file",
	Long: `Add multiple rows to a database from a JSON file.

File format: JSON array of objects with property key-value pairs.

Examples:
  notion db add-bulk abc123 --file items.json

  # items.json:
  # [
  #   {"Name": "Task A", "Status": "Todo"},
  #   {"Name": "Task B", "Status": "Done", "Priority": "High"}
  # ]`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		dbID := util.ResolveID(args[0])
		filePath, _ := cmd.Flags().GetString("file")

		if filePath == "" {
			return fmt.Errorf("--file is required")
		}

		// Read and parse JSON file
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		var items []map[string]string
		if err := json.Unmarshal(data, &items); err != nil {
			return fmt.Errorf("parse JSON: %w (expected array of {\"Key\": \"Value\"} objects)", err)
		}

		if len(items) == 0 {
			return fmt.Errorf("no items in file")
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		// Get database schema once
		db, err := c.GetDatabase(dbID)
		if err != nil {
			return fmt.Errorf("get database schema: %w", err)
		}
		dbProps, _ := db["properties"].(map[string]interface{})

		created := 0
		var errors []string

		for i, item := range items {
			properties := map[string]interface{}{}
			for key, value := range item {
				propDef, ok := dbProps[key].(map[string]interface{})
				if !ok {
					errors = append(errors, fmt.Sprintf("row %d: property %q not found", i+1, key))
					continue
				}
				propType, _ := propDef["type"].(string)
				properties[key] = buildPropertyValue(propType, value)
			}

			body := map[string]interface{}{
				"parent": map[string]interface{}{
					"database_id": dbID,
				},
				"properties": properties,
			}

			_, err := c.Post("/v1/pages", body)
			if err != nil {
				errors = append(errors, fmt.Sprintf("row %d: %v", i+1, err))
				continue
			}
			created++

			if outputFormat != "json" {
				fmt.Printf("\r  %d/%d rows created", created, len(items))
			}
		}

		if outputFormat == "json" {
			return render.JSON(map[string]interface{}{
				"created": created,
				"total":   len(items),
				"errors":  errors,
			})
		}

		fmt.Println() // newline after progress
		fmt.Printf("✓ %d/%d rows created\n", created, len(items))
		if len(errors) > 0 {
			for _, e := range errors {
				fmt.Printf("  ✗ %s\n", e)
			}
		}
		return nil
	},
}

func init() {
	dbListCmd.Flags().IntP("limit", "l", 10, "Maximum results")
	dbListCmd.Flags().String("cursor", "", "Pagination cursor")
	dbListCmd.Flags().Bool("all", false, "Fetch all pages of results")
	dbCreateCmd.Flags().String("title", "", "Database title (required)")
	dbCreateCmd.Flags().String("props", "", "Additional properties as name:type,... (e.g. Status:select,Date:date)")
	dbUpdateCmd.Flags().String("title", "", "New database title")
	dbUpdateCmd.Flags().String("add-prop", "", "Add properties as name:type,... (e.g. Priority:select)")
	dbQueryCmd.Flags().StringArrayP("filter", "F", nil, "Filter expression (e.g. 'Status=Done')")
	dbQueryCmd.Flags().StringArrayP("sort", "s", nil, "Sort expression (e.g. 'Date:desc')")
	dbQueryCmd.Flags().IntP("limit", "l", 0, "Maximum results")
	dbQueryCmd.Flags().String("cursor", "", "Pagination cursor")
	dbAddBulkCmd.Flags().String("file", "", "JSON file with rows to create (required)")

	dbCmd.AddCommand(dbListCmd)
	dbCmd.AddCommand(dbViewCmd)
	dbCmd.AddCommand(dbCreateCmd)
	dbCmd.AddCommand(dbUpdateCmd)
	dbCmd.AddCommand(dbAddCmd)
	dbCmd.AddCommand(dbAddBulkCmd)
	dbCmd.AddCommand(dbQueryCmd)
	dbCmd.AddCommand(dbOpenCmd)
}

// parseFilter parses a filter expression like "Status=Done" into a Notion filter object.
func parseFilter(expr string, dbProps map[string]interface{}) (map[string]interface{}, error) {
	// Try operators in order of specificity (longest first)
	operators := []struct {
		op     string
		notion string
	}{
		{">=", "gte"},
		{"<=", "lte"},
		{"!=", "neq"},
		{"~=", "contains"},
		{"!~=", "not_contains"},
		{">", "gt"},
		{"<", "lt"},
		{"=", "eq"},
	}

	for _, op := range operators {
		idx := strings.Index(expr, op.op)
		if idx < 0 {
			continue
		}

		propName := strings.TrimSpace(expr[:idx])
		value := strings.TrimSpace(expr[idx+len(op.op):])

		// Look up property type
		propDef, ok := dbProps[propName].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("property %q not found in database", propName)
		}
		propType, _ := propDef["type"].(string)

		return buildFilter(propName, propType, op.notion, value), nil
	}

	return nil, fmt.Errorf("no valid operator found in expression")
}

// buildFilter creates a Notion API filter based on property type and operator.
func buildFilter(propName, propType, op, value string) map[string]interface{} {
	filter := map[string]interface{}{
		"property": propName,
	}

	// Map operator to Notion API field name based on property type
	switch propType {
	case "title", "rich_text", "url", "email", "phone_number":
		textOp := mapTextOp(op)
		filter[propType] = map[string]interface{}{textOp: value}
	case "number":
		numOp := mapNumberOp(op)
		// Try to parse as float
		var numVal interface{} = value
		if n := json.Number(value); true {
			if f, err := n.Float64(); err == nil {
				numVal = f
			}
		}
		filter["number"] = map[string]interface{}{numOp: numVal}
	case "select":
		selectOp := "equals"
		if op == "neq" {
			selectOp = "does_not_equal"
		}
		filter["select"] = map[string]interface{}{selectOp: value}
	case "multi_select":
		msOp := "contains"
		if op == "not_contains" || op == "neq" {
			msOp = "does_not_contain"
		}
		filter["multi_select"] = map[string]interface{}{msOp: value}
	case "status":
		statusOp := "equals"
		if op == "neq" {
			statusOp = "does_not_equal"
		}
		filter["status"] = map[string]interface{}{statusOp: value}
	case "date", "created_time", "last_edited_time":
		dateOp := mapDateOp(op)
		dateType := propType
		if dateType == "date" {
			dateType = "date"
		}
		filter[dateType] = map[string]interface{}{dateOp: value}
	case "checkbox":
		boolVal := value == "true" || value == "1" || value == "yes"
		filter["checkbox"] = map[string]interface{}{"equals": boolVal}
	default:
		// Fallback: try as rich_text
		textOp := mapTextOp(op)
		filter["rich_text"] = map[string]interface{}{textOp: value}
	}

	return filter
}

func mapTextOp(op string) string {
	switch op {
	case "eq":
		return "equals"
	case "neq":
		return "does_not_equal"
	case "contains":
		return "contains"
	case "not_contains":
		return "does_not_contain"
	default:
		return "equals"
	}
}

func mapNumberOp(op string) string {
	switch op {
	case "eq":
		return "equals"
	case "neq":
		return "does_not_equal"
	case "gt":
		return "greater_than"
	case "gte":
		return "greater_than_or_equal_to"
	case "lt":
		return "less_than"
	case "lte":
		return "less_than_or_equal_to"
	default:
		return "equals"
	}
}

func mapDateOp(op string) string {
	switch op {
	case "eq":
		return "equals"
	case "gt", "gte":
		return "on_or_after"
	case "lt", "lte":
		return "on_or_before"
	case "neq":
		return "does_not_equal"
	default:
		return "equals"
	}
}

// parseSort parses a sort expression like "Date:desc" into a Notion sort object.
func parseSort(expr string) map[string]interface{} {
	parts := strings.SplitN(expr, ":", 2)
	propName := strings.TrimSpace(parts[0])
	direction := "ascending"
	if len(parts) == 2 {
		d := strings.TrimSpace(strings.ToLower(parts[1]))
		if d == "desc" || d == "descending" {
			direction = "descending"
		}
	}
	return map[string]interface{}{
		"property":  propName,
		"direction": direction,
	}
}

// extractSchemaOptions returns a summary of options for select/multi_select/status properties.
func extractSchemaOptions(prop map[string]interface{}, propType string) string {
	var getData func() []interface{}

	switch propType {
	case "select":
		getData = func() []interface{} {
			if sel, ok := prop["select"].(map[string]interface{}); ok {
				if opts, ok := sel["options"].([]interface{}); ok {
					return opts
				}
			}
			return nil
		}
	case "multi_select":
		getData = func() []interface{} {
			if ms, ok := prop["multi_select"].(map[string]interface{}); ok {
				if opts, ok := ms["options"].([]interface{}); ok {
					return opts
				}
			}
			return nil
		}
	case "status":
		getData = func() []interface{} {
			if s, ok := prop["status"].(map[string]interface{}); ok {
				if opts, ok := s["options"].([]interface{}); ok {
					return opts
				}
			}
			return nil
		}
	default:
		return ""
	}

	opts := getData()
	if len(opts) == 0 {
		return ""
	}

	var names []string
	for _, o := range opts {
		if m, ok := o.(map[string]interface{}); ok {
			name, _ := m["name"].(string)
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}
