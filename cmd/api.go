package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/4ier/notion-cli/internal/client"
	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api <method> <path> [--body <json>]",
	Short: "Make a raw API request",
	Long: `Make an authenticated request to the Notion API.

This is an escape hatch for any operation not yet covered by the CLI.

The <path> must target the Notion API (starts with /v1/). For convenience,
paths that start with "/" but not "/v1/" are auto-prefixed with /v1/.

Body can be provided three ways:
  --body '<json>'         inline JSON string
  --body @<path>          read JSON from a file
  (stdin)                 pipe JSON on stdin for POST/PATCH/PUT

Examples:
  notion api GET /v1/users/me
  notion api POST /v1/search --body '{"query":"test"}'
  notion api PATCH /v1/blocks/<id>/children --body @children.json
  echo '{"query":"test"}' | notion api POST /v1/search`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		method := strings.ToUpper(args[0])
		path := normalizeAPIPath(args[1])

		bodyStr, _ := cmd.Flags().GetString("body")

		// Support --body @file.json to read body from file.
		if strings.HasPrefix(bodyStr, "@") {
			filePath := strings.TrimPrefix(bodyStr, "@")
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read body file: %w", err)
			}
			bodyStr = string(data)
		}

		// Read body from stdin if not provided via flag (or --body - explicit stdin).
		if bodyStr == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read body from stdin: %w", err)
			}
			bodyStr = string(data)
		}
		if bodyStr == "" && (method == "POST" || method == "PATCH" || method == "PUT") {
			stat, _ := os.Stdin.Stat()
			if stat != nil && stat.Mode()&os.ModeCharDevice == 0 {
				data, err := io.ReadAll(os.Stdin)
				if err == nil && len(data) > 0 {
					bodyStr = string(data)
				}
			}
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		var respData []byte
		if bodyStr != "" {
			var body interface{}
			if err := json.Unmarshal([]byte(bodyStr), &body); err != nil {
				return fmt.Errorf("invalid JSON body: %w", err)
			}
			switch method {
			case "PATCH":
				respData, err = c.Patch(path, body)
			case "POST", "PUT":
				respData, err = c.Post(path, body)
			case "DELETE":
				respData, err = c.Delete(path)
			case "GET":
				return fmt.Errorf("GET requests do not accept a body")
			default:
				respData, err = c.Post(path, body)
			}
		} else {
			switch method {
			case "GET":
				respData, err = c.Get(path)
			case "DELETE":
				respData, err = c.Delete(path)
			default:
				respData, err = c.Post(path, nil)
			}
		}

		if err != nil {
			return err
		}

		// Pretty-print JSON response
		var formatted interface{}
		if json.Unmarshal(respData, &formatted) == nil {
			out, _ := json.MarshalIndent(formatted, "", "  ")
			fmt.Println(string(out))
		} else {
			fmt.Println(string(respData))
		}

		return nil
	},
}

// normalizeAPIPath ensures the path targets the Notion API.
//   - Prefixes with "/" if missing.
//   - Auto-prefixes "/v1" when the path starts with "/" but not "/v1/" (and
//     isn't already "/v1"). A note is written to stderr so power users are
//     aware of the rewrite.
func normalizeAPIPath(path string) string {
	if path == "" {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path == "/v1" || strings.HasPrefix(path, "/v1/") {
		return path
	}
	normalized := "/v1" + path
	fmt.Fprintf(os.Stderr, "note: prepending /v1 to path → %s\n", normalized)
	return normalized
}

func init() {
	apiCmd.Flags().String("body", "", "JSON request body. Use @<file> to read from file, or - for stdin")
}
