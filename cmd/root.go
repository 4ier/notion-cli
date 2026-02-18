package cmd

import (
	"fmt"
	"os"

	"github.com/4ier/notion-cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	outputFormat string
	debugMode    bool
	// Version is set by goreleaser ldflags
	Version = "dev"
)

var rootCmd = &cobra.Command{
	Use:     "notion",
	Short:   "Work seamlessly with Notion from the command line",
	Long: `Work seamlessly with Notion from the command line.

Notion CLI lets you manage pages, databases, blocks, and more
without leaving your terminal. Built for developers and AI agents.`,
	Version:       Version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "format", "f", "", "Output format: json, table, text (default: auto)")
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Show HTTP request/response details")

	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(pageCmd)
	rootCmd.AddCommand(dbCmd)
	rootCmd.AddCommand(blockCmd)
	rootCmd.AddCommand(userCmd)
	rootCmd.AddCommand(apiCmd)
	rootCmd.AddCommand(commentCmd)
	rootCmd.AddCommand(fileCmd)
}

// getToken returns the Notion API token from flag, env, or config file.
func getToken() (string, error) {
	// 1. Environment variable
	if token := os.Getenv("NOTION_TOKEN"); token != "" {
		return token, nil
	}

	// 2. Config file
	cfg, err := config.Load()
	if err == nil && cfg.Token != "" {
		return cfg.Token, nil
	}

	return "", fmt.Errorf("not authenticated. Run 'notion auth login --token' or set NOTION_TOKEN")
}
