package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/4ier/notion-cli/internal/client"
	"github.com/4ier/notion-cli/internal/render"
	"github.com/4ier/notion-cli/internal/util"
	"github.com/spf13/cobra"
)

// pageMarkdownCmd wraps GET /v1/pages/:id/markdown — Notion's server-side
// markdown rendering of a page. Strictly better than 'block list --md' for
// most page targets because the server handles toggles, columns, synced
// blocks, callouts, and databases-as-pages uniformly.
//
// For non-page targets (a single block) 'block list --md' is still the right
// tool — this command only accepts a page id / url.
var pageMarkdownCmd = &cobra.Command{
	Use:   "markdown <page-id|url>",
	Short: "Render a page as markdown (server-side, complete)",
	Long: `Render a full Notion page as markdown, returning the server-rendered
text. This is the preferred way to dump a page to markdown: the server
handles nested layouts (toggles, columns, synced blocks) correctly and
is always consistent with how the Notion UI presents the page.

Compared to 'block list <id> --md':
  - Works on the entire page tree including nested databases.
  - Cannot address a single sub-block (use 'block list' for that).
  - Reports whether the content was 'truncated' and which blocks (if any)
    the server could not render, via the 'unknown_block_ids' field in
    --format json.

Examples:
  notion page markdown <page-id>
  notion page markdown <page-id> > page.md
  notion page markdown <page-id> --format json     # full response incl. truncated flag
  notion page markdown <page-id> --out page.md     # write directly to file`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		pageID := util.ResolveID(args[0])
		outPath, _ := cmd.Flags().GetString("out")

		c := client.New(token)
		c.SetDebug(debugMode)

		data, err := c.Get(fmt.Sprintf("/v1/pages/%s/markdown", pageID))
		if err != nil {
			return fmt.Errorf("get page markdown: %w", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		if outputFormat == "json" {
			return render.JSON(result)
		}

		markdown, _ := result["markdown"].(string)
		truncated, _ := result["truncated"].(bool)
		if unknowns, ok := result["unknown_block_ids"].([]interface{}); ok && len(unknowns) > 0 {
			fmt.Fprintf(os.Stderr, "note: %d block(s) could not be rendered as markdown (see --format json for ids)\n", len(unknowns))
		}
		if truncated {
			fmt.Fprintln(os.Stderr, "note: response was truncated server-side — very long pages may be incomplete")
		}

		if outPath != "" {
			if err := os.WriteFile(outPath, []byte(markdown), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", outPath, err)
			}
			fmt.Fprintf(os.Stderr, "✓ wrote %d bytes to %s\n", len(markdown), outPath)
			return nil
		}
		fmt.Print(markdown)
		if !strings.HasSuffix(markdown, "\n") {
			fmt.Println()
		}
		return nil
	},
}

// pageSetMarkdownCmd wraps PATCH /v1/pages/:id/markdown with the four
// mutation modes the Notion API supports:
//
//	--replace              replace_content: overwrite whole page (default)
//	--append               insert_content: append at the end of the page
//	--after <ellipsis>     insert_content: insert after a text anchor
//	--range <ellipsis>     replace_content_range: replace a range
//
// Ellipsis anchors look like "start text...end text" per the Notion API
// convention.
var pageSetMarkdownCmd = &cobra.Command{
	Use:   "set-markdown <page-id|url>",
	Short: "Replace / append / edit page content using markdown",
	Long: `Update a page's content by sending markdown to Notion's server-side
renderer. One call replaces or inserts as many blocks as the markdown
parses to — no need to batch, and no 100-children limit applies.

Modes (pick one, default is --replace):

  --replace         Overwrite the entire page content.
  --append          Insert content at the end of the page.
  --after <anchor>  Insert content after an ellipsis anchor
                    (format: "start text...end text").
  --range <anchor>  Replace a range bounded by an ellipsis anchor.

Source (pick one):

  --file <path>     Read markdown from a file. Use '-' for stdin.
  --text <str>      Inline markdown string.

Examples:
  notion page set-markdown <id> --file new.md
  cat new.md | notion page set-markdown <id> --file -
  notion page set-markdown <id> --append --text "\n\n> Update: done."
  notion page set-markdown <id> --after "Status...done" --text "More detail below."
  notion page set-markdown <id> --replace --file new.md --allow-deleting-content`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		pageID := util.ResolveID(args[0])
		filePath, _ := cmd.Flags().GetString("file")
		text, _ := cmd.Flags().GetString("text")
		replace, _ := cmd.Flags().GetBool("replace")
		appendMode, _ := cmd.Flags().GetBool("append")
		after, _ := cmd.Flags().GetString("after")
		rangeAnchor, _ := cmd.Flags().GetString("range")
		allowDelete, _ := cmd.Flags().GetBool("allow-deleting-content")

		content, err := readMarkdownSource(filePath, text)
		if err != nil {
			return err
		}

		body, err := buildSetMarkdownBody(content, replace, appendMode, after, rangeAnchor, allowDelete)
		if err != nil {
			return err
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		data, err := c.Patch(fmt.Sprintf("/v1/pages/%s/markdown", pageID), body)
		if err != nil {
			return fmt.Errorf("set page markdown: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parse response: %w", err)
			}
			return render.JSON(result)
		}

		mode := "replaced"
		switch {
		case appendMode:
			mode = "appended to"
		case after != "":
			mode = "inserted after anchor in"
		case rangeAnchor != "":
			mode = "replaced range in"
		}
		fmt.Printf("✓ %s page\n", mode)
		return nil
	},
}

// readMarkdownSource picks the markdown source from --file / --text / stdin.
// Exactly one must be set; --file "-" also reads from stdin.
func readMarkdownSource(filePath, text string) (string, error) {
	if filePath == "" && text == "" {
		return "", fmt.Errorf("one of --file or --text is required")
	}
	if filePath != "" && text != "" {
		return "", fmt.Errorf("--file and --text are mutually exclusive")
	}
	if text != "" {
		return text, nil
	}
	if filePath == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", filePath, err)
	}
	return string(data), nil
}

// buildSetMarkdownBody assembles the PATCH body for the four mutation modes
// Notion's page-markdown endpoint accepts. At most one mode flag must be
// truthy; if none are set we default to replace_content.
func buildSetMarkdownBody(content string, replace, appendMode bool, after, rangeAnchor string, allowDelete bool) (map[string]interface{}, error) {
	// Count how many mode flags are set so we can reject ambiguous combinations.
	modes := 0
	if replace {
		modes++
	}
	if appendMode {
		modes++
	}
	if after != "" {
		modes++
	}
	if rangeAnchor != "" {
		modes++
	}
	if modes > 1 {
		return nil, fmt.Errorf("pick at most one of --replace, --append, --after, --range")
	}

	switch {
	case appendMode:
		return map[string]interface{}{
			"type": "insert_content",
			"insert_content": map[string]interface{}{
				"content": content,
			},
		}, nil
	case after != "":
		return map[string]interface{}{
			"type": "insert_content",
			"insert_content": map[string]interface{}{
				"content": content,
				"after":   after,
			},
		}, nil
	case rangeAnchor != "":
		rr := map[string]interface{}{
			"content":       content,
			"content_range": rangeAnchor,
		}
		if allowDelete {
			rr["allow_deleting_content"] = true
		}
		return map[string]interface{}{
			"type":                   "replace_content_range",
			"replace_content_range": rr,
		}, nil
	default:
		// Default is replace_content.
		rc := map[string]interface{}{
			"new_str": content,
		}
		if allowDelete {
			rc["allow_deleting_content"] = true
		}
		return map[string]interface{}{
			"type":            "replace_content",
			"replace_content": rc,
		}, nil
	}
}
