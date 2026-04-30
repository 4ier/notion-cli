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

var commentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Work with comments",
}

var commentListCmd = &cobra.Command{
	Use:   "list <page-id|url>",
	Short: "List comments on a page",
	Long: `List all comments on a Notion page or block.

Examples:
  notion comment list abc123
  notion comment list abc123 --format json
  notion comment list abc123 --all`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		blockID := util.ResolveID(args[0])
		all, _ := cmd.Flags().GetBool("all")
		cursor, _ := cmd.Flags().GetString("cursor")
		c := client.New(token)
		c.SetDebug(debugMode)

		var allResults []interface{}
		currentCursor := cursor

		for {
			result, err := c.ListComments(blockID, 100, currentCursor)
			if err != nil {
				return fmt.Errorf("list comments: %w", err)
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

		if len(allResults) == 0 {
			fmt.Println("No comments found.")
			return nil
		}

		for _, r := range allResults {
			comment, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := comment["id"].(string)
			createdTime, _ := comment["created_time"].(string)
			if len(createdTime) > 10 {
				createdTime = createdTime[:10]
			}

			var text string
			if richText, ok := comment["rich_text"].([]interface{}); ok {
				for _, t := range richText {
					if m, ok := t.(map[string]interface{}); ok {
						if pt, ok := m["plain_text"].(string); ok {
							text += pt
						}
					}
				}
			}

			render.Field("Comment", text)
			render.Subtitle(fmt.Sprintf("  ID: %s  Created: %s", id, createdTime))
			fmt.Println()
		}

		return nil
	},
}

var commentAddCmd = &cobra.Command{
	Use:   "add <page-id|url> [text]",
	Short: "Add a comment to a page",
	Long: `Add a comment to a Notion page.

Examples:
  notion comment add abc123 "This looks great!"
  notion comment add abc123 --mention-user user-123 --text "Please review this"
  notion comment add abc123 --mention-user user-123 --mention-user user-456 --text "Please review this"`,
	Args: validateCommentAddArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		pageID := util.ResolveID(args[0])
		text, mentionUserIDs, err := resolveCommentAddContent(cmd, args)
		if err != nil {
			return err
		}

		token, err := getToken()
		if err != nil {
			return err
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		data, err := c.AddComment(pageID, text, mentionUserIDs)
		if err != nil {
			return fmt.Errorf("add comment: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parse response: %w", err)
			}
			return render.JSON(result)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		id, _ := result["id"].(string)

		fmt.Fprintf(cmd.OutOrStdout(), "✓ Comment added\n")
		fmt.Fprintf(cmd.OutOrStdout(), "ID:             %s\n", id)
		return nil
	},
}

func validateCommentAddArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return cobra.RangeArgs(1, 2)(cmd, args)
	}

	_, _, err := resolveCommentAddContent(cmd, args)
	return err
}

func resolveCommentAddContent(cmd *cobra.Command, args []string) (string, []string, error) {
	flagText, _ := cmd.Flags().GetString("text")
	mentionUserIDs, _ := cmd.Flags().GetStringArray("mention-user")

	if len(args) == 2 && flagText != "" {
		return "", nil, fmt.Errorf("provide comment text either as an argument or with --text, not both")
	}

	text := flagText
	if len(args) == 2 {
		text = args[1]
	}

	if strings.TrimSpace(text) == "" && len(mentionUserIDs) == 0 {
		return "", nil, fmt.Errorf("comment content required: provide text or at least one --mention-user")
	}

	resolvedMentionUserIDs := make([]string, len(mentionUserIDs))
	for i, userID := range mentionUserIDs {
		resolvedMentionUserIDs[i] = util.ResolveID(userID)
	}

	return text, resolvedMentionUserIDs, nil
}

var commentGetCmd = &cobra.Command{
	Use:   "get <comment-id>",
	Short: "Get a comment by ID",
	Long: `Retrieve a single comment by its ID.

Examples:
  notion comment get abc123
  notion comment get abc123 --format json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		commentID := args[0]
		c := client.New(token)
		c.SetDebug(debugMode)

		data, err := c.Get("/v1/comments/" + commentID)
		if err != nil {
			return fmt.Errorf("get comment: %w", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return err
		}

		if outputFormat == "json" {
			return render.JSON(result)
		}

		id, _ := result["id"].(string)
		createdTime, _ := result["created_time"].(string)

		var text string
		if richText, ok := result["rich_text"].([]interface{}); ok {
			for _, t := range richText {
				if m, ok := t.(map[string]interface{}); ok {
					if pt, ok := m["plain_text"].(string); ok {
						text += pt
					}
				}
			}
		}

		render.Field("Comment", text)
		render.Field("ID", id)
		render.Field("Created", createdTime)
		return nil
	},
}

var commentReplyCmd = &cobra.Command{
	Use:   "reply <comment-id> <text>",
	Short: "Reply to a comment in the same thread",
	Long: `Reply to an existing comment in the same discussion thread.

The reply will appear in the same thread as the original comment.

Examples:
  notion comment reply abc123 "Thanks for the feedback!"
  notion comment reply abc123 "I'll look into this."`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		commentID := args[0]
		text := args[1]

		c := client.New(token)
		c.SetDebug(debugMode)

		// Get the parent comment to find its discussion_id
		data, err := c.Get("/v1/comments/" + commentID)
		if err != nil {
			return fmt.Errorf("get comment: %w", err)
		}

		var parentComment map[string]interface{}
		if err := json.Unmarshal(data, &parentComment); err != nil {
			return fmt.Errorf("parse comment: %w", err)
		}

		discussionID, _ := parentComment["discussion_id"].(string)
		if discussionID == "" {
			return fmt.Errorf("could not find discussion_id on comment %s", commentID)
		}

		// Post reply to the same discussion thread
		reqBody := map[string]interface{}{
			"discussion_id": discussionID,
			"rich_text": []map[string]interface{}{
				{"text": map[string]interface{}{"content": text}},
			},
		}

		respData, err := c.Post("/v1/comments", reqBody)
		if err != nil {
			return fmt.Errorf("post reply: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			if err := json.Unmarshal(respData, &result); err != nil {
				return fmt.Errorf("parse response: %w", err)
			}
			return render.JSON(result)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(respData, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		id, _ := result["id"].(string)

		render.Title("✓", "Reply added")
		render.Field("ID", id)
		render.Field("Discussion", discussionID)
		return nil
	},
}

func init() {
	commentListCmd.Flags().String("cursor", "", "Pagination cursor")
	commentListCmd.Flags().Bool("all", false, "Fetch all pages of results")
	commentAddCmd.Flags().String("text", "", "Comment text")
	commentAddCmd.Flags().StringArray("mention-user", nil, "Mention a Notion user by ID (repeatable)")
	commentUpdateCmd.Flags().String("text", "", "New comment text (required)")
	commentUpdateCmd.Flags().StringArray("mention-user", nil, "Mention a Notion user by ID (repeatable)")

	commentCmd.AddCommand(commentListCmd)
	commentCmd.AddCommand(commentAddCmd)
	commentCmd.AddCommand(commentGetCmd)
	commentCmd.AddCommand(commentReplyCmd)
	commentCmd.AddCommand(commentUpdateCmd)
	commentCmd.AddCommand(commentDeleteCmd)
}

var commentUpdateCmd = &cobra.Command{
	Use:   "update <comment-id>",
	Short: "Edit an existing comment's text",
	Long: `Edit the text of an existing comment.

Wraps PATCH /v1/comments/:id (added in Notion's 2025 API). The new
rich_text is built the same way as 'comment add' — --mention-user works
if you need to keep or add @mentions.

Examples:
  notion comment update abc123 --text "Fixed typo in previous comment"
  notion comment update abc123 --text "with mention" --mention-user <user-id>`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		commentID := strings.TrimSpace(args[0])
		text, _ := cmd.Flags().GetString("text")
		mentionUserIDs, _ := cmd.Flags().GetStringArray("mention-user")

		if text == "" && len(mentionUserIDs) == 0 {
			return fmt.Errorf("--text or --mention-user is required")
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		data, err := c.UpdateComment(commentID, text, mentionUserIDs)
		if err != nil {
			return fmt.Errorf("update comment: %w", err)
		}

		if outputFormat == "json" {
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parse response: %w", err)
			}
			return render.JSON(result)
		}

		fmt.Println("✓ Comment updated")
		return nil
	},
}

var commentDeleteCmd = &cobra.Command{
	Use:   "delete <comment-id ...>",
	Short: "Delete one or more comments",
	Long: `Delete comments by id. Accepts multiple ids for bulk removal,
mirroring 'block delete' — per-id errors are printed but do not stop
the batch.

Note: deleting the anchor (first) comment of a discussion removes the
whole thread. Deleting a reply removes just that one reply.

Wraps DELETE /v1/comments/:id (added in Notion's 2025 API).

Examples:
  notion comment delete abc123
  notion comment delete abc123 def456 ghi789`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		deleted := 0
		for _, id := range args {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, err := c.DeleteComment(id); err != nil {
				fmt.Fprintf(os.Stderr, "✗ Failed to delete %s: %v\n", id, err)
				continue
			}
			deleted++
		}

		if outputFormat != "json" {
			fmt.Printf("✓ %d comment(s) deleted\n", deleted)
		}
		return nil
	},
}
