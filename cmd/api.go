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

Examples:
  notion api GET /v1/users/me
  notion api POST /v1/search --body '{"query":"test"}'
  echo '{"query":"test"}' | notion api POST /v1/search`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		method := strings.ToUpper(args[0])
		path := args[1]

		// Ensure path starts with /
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		bodyStr, _ := cmd.Flags().GetString("body")

		// Read body from stdin if not provided via flag
		if bodyStr == "" && (method == "POST" || method == "PATCH" || method == "PUT") {
			stat, _ := os.Stdin.Stat()
			if stat.Mode()&os.ModeCharDevice == 0 {
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
			respData, err = c.Post(path, body)
			if method == "PATCH" {
				respData, err = c.Patch(path, body)
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

func init() {
	apiCmd.Flags().String("body", "", "JSON request body")
}
