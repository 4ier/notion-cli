package cmd

import (
	"fmt"
	"strings"

	"github.com/4ier/notion-cli/internal/client"
	"github.com/4ier/notion-cli/internal/render"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search pages and databases",
	Long: `Search across your Notion workspace by title.

Examples:
  notion search "meeting notes"
  notion search --type page "roadmap"
  notion search --type database
  notion search --limit 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		query := ""
		if len(args) > 0 {
			query = strings.Join(args, " ")
		}

		filterType, _ := cmd.Flags().GetString("type")
		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")
		all, _ := cmd.Flags().GetBool("all")

		c := client.New(token)
		c.SetDebug(debugMode)

		var allResults []interface{}
		currentCursor := cursor

		for {
			result, err := c.Search(query, filterType, limit, currentCursor)
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
					return render.JSON(map[string]interface{}{
						"results": allResults,
					})
				}
				break
			}
			nextCursor, _ := result["next_cursor"].(string)
			currentCursor = nextCursor
		}

		if len(allResults) == 0 {
			fmt.Println("No results found.")
			return nil
		}

		headers := []string{"TYPE", "TITLE", "ID", "LAST EDITED"}
		var rows [][]string

		for _, r := range allResults {
			obj, ok := r.(map[string]interface{})
			if !ok {
				continue
			}

			objType, _ := obj["object"].(string)
			title := render.ExtractTitle(obj)
			id, _ := obj["id"].(string)
			lastEdited, _ := obj["last_edited_time"].(string)
			if len(lastEdited) > 10 {
				lastEdited = lastEdited[:10]
			}

			icon := "📄"
			if objType == "database" {
				icon = "🗃️"
			}

			rows = append(rows, []string{icon + " " + objType, title, id, lastEdited})
		}

		render.Table(headers, rows)
		return nil
	},
}

func init() {
	searchCmd.Flags().StringP("type", "t", "", "Filter by type: page, database")
	searchCmd.Flags().IntP("limit", "l", 10, "Maximum results to return")
	searchCmd.Flags().String("cursor", "", "Pagination cursor from previous results")
	searchCmd.Flags().Bool("all", false, "Fetch all pages of results")
}
