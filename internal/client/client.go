package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"time"

	"github.com/4ier/notion-cli/internal/util"
)

const (
	BaseURL        = "https://api.notion.com"
	NotionVersion  = "2022-06-28"
	DefaultTimeout = 30 * time.Second
	UploadTimeout  = 5 * time.Minute
)

type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
	debug      bool
}

func New(token string) *Client {
	base := BaseURL
	if envBase := os.Getenv("NOTION_BASE_URL"); envBase != "" {
		base = envBase
	}
	return &Client{
		token:   token,
		baseURL: base,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

// NewWithBaseURL creates a client pointing at a custom API base (for testing).
func NewWithBaseURL(token, baseURL string) *Client {
	c := New(token)
	c.baseURL = baseURL
	return c
}

func (c *Client) SetDebug(debug bool) {
	c.debug = debug
}

func (c *Client) do(method, path string, body interface{}) ([]byte, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Notion-Version", NotionVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.debug {
		fmt.Printf("→ %s %s\n", method, url)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if c.debug {
		fmt.Printf("← %d %s (%d bytes)\n", resp.StatusCode, resp.Status, len(respBody))
	}

	if resp.StatusCode >= 400 {
		var apiErr struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Message != "" {
			hint := errorHint(apiErr.Code, apiErr.Message)
			if hint != "" {
				return nil, fmt.Errorf("%s: %s\n  → %s", apiErr.Code, apiErr.Message, hint)
			}
			return nil, fmt.Errorf("%s: %s", apiErr.Code, apiErr.Message)
		}
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	return respBody, nil
}

func (c *Client) Get(path string) ([]byte, error) {
	return c.do("GET", path, nil)
}

func (c *Client) Post(path string, body interface{}) ([]byte, error) {
	return c.do("POST", path, body)
}

func (c *Client) Patch(path string, body interface{}) ([]byte, error) {
	return c.do("PATCH", path, body)
}

func (c *Client) Delete(path string) ([]byte, error) {
	return c.do("DELETE", path, nil)
}

// GetMe returns the bot user info for the current token.
func (c *Client) GetMe() (map[string]interface{}, error) {
	data, err := c.Get("/v1/users/me")
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetUser retrieves a user by ID.
func (c *Client) GetUser(userID string) (map[string]interface{}, error) {
	data, err := c.Get("/v1/users/" + userID)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Search performs a search across the workspace.
func (c *Client) Search(query string, filter string, pageSize int, startCursor string) (map[string]interface{}, error) {
	body := map[string]interface{}{}
	if query != "" {
		body["query"] = query
	}
	if filter != "" {
		body["filter"] = map[string]interface{}{
			"value":    filter,
			"property": "object",
		}
	}
	if pageSize > 0 {
		body["page_size"] = pageSize
	}
	if startCursor != "" {
		body["start_cursor"] = startCursor
	}

	data, err := c.Post("/v1/search", body)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetPage retrieves a page by ID.
func (c *Client) GetPage(pageID string) (map[string]interface{}, error) {
	data, err := c.Get("/v1/pages/" + pageID)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetBlock retrieves a single block by ID.
func (c *Client) GetBlock(blockID string) (map[string]interface{}, error) {
	data, err := c.Get("/v1/blocks/" + blockID)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetBlockChildren retrieves children of a block.
func (c *Client) GetBlockChildren(blockID string, pageSize int, startCursor string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/v1/blocks/%s/children?page_size=%d", blockID, pageSize)
	if startCursor != "" {
		path += "&start_cursor=" + startCursor
	}
	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetDatabase retrieves a database by ID.
func (c *Client) GetDatabase(dbID string) (map[string]interface{}, error) {
	data, err := c.Get("/v1/databases/" + dbID)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// QueryDatabase queries a database with filters and sorts.
func (c *Client) QueryDatabase(dbID string, body map[string]interface{}) (map[string]interface{}, error) {
	data, err := c.Post("/v1/databases/"+dbID+"/query", body)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetUsers lists all users.
func (c *Client) GetUsers(pageSize int, startCursor string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/v1/users?page_size=%d", pageSize)
	if startCursor != "" {
		path += "&start_cursor=" + startCursor
	}
	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListComments lists comments on a block/page.
func (c *Client) ListComments(blockID string, pageSize int, startCursor string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/v1/comments?block_id=%s&page_size=%d", blockID, pageSize)
	if startCursor != "" {
		path += "&start_cursor=" + startCursor
	}
	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// AddComment adds a comment to a page.
func (c *Client) AddComment(pageID, text string, mentionUserIDs []string) ([]byte, error) {
	body := map[string]interface{}{
		"parent": map[string]interface{}{
			"page_id": util.ResolveID(pageID),
		},
		"rich_text": buildCommentRichText(text, mentionUserIDs),
	}
	return c.Post("/v1/comments", body)
}

// UpdateComment edits the rich_text body of an existing comment.
// Wraps PATCH /v1/comments/:id (added in Notion API 2025).
func (c *Client) UpdateComment(commentID, text string, mentionUserIDs []string) ([]byte, error) {
	body := map[string]interface{}{
		"rich_text": buildCommentRichText(text, mentionUserIDs),
	}
	return c.Patch("/v1/comments/"+commentID, body)
}

// DeleteComment removes a comment by id.
// Wraps DELETE /v1/comments/:id (added in Notion API 2025). When the
// target is the anchor comment of a discussion, Notion removes the whole
// thread; otherwise it removes just that one reply.
func (c *Client) DeleteComment(commentID string) ([]byte, error) {
	return c.Delete("/v1/comments/" + commentID)
}

func buildCommentRichText(text string, mentionUserIDs []string) []map[string]interface{} {
	var richText []map[string]interface{}

	for i, userID := range mentionUserIDs {
		richText = append(richText, map[string]interface{}{
			"type": "mention",
			"mention": map[string]interface{}{
				"type": "user",
				"user": map[string]interface{}{
					"id": userID,
				},
			},
		})
		if i < len(mentionUserIDs)-1 || text != "" {
			richText = append(richText, commentTextRichText(" "))
		}
	}

	if text != "" {
		richText = append(richText, commentTextRichText(text))
	}

	return richText
}

func commentTextRichText(text string) map[string]interface{} {
	return map[string]interface{}{
		"type": "text",
		"text": map[string]interface{}{
			"content": text,
		},
	}
}

// UploadFileContent sends file content to an existing file upload via multipart form.
func (c *Client) UploadFileContent(uploadID, fileName, contentType string, fileBytes []byte) ([]byte, error) {
	url := c.baseURL + fmt.Sprintf("/v1/file_uploads/%s/send", uploadID)

	// Build multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(fileName)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	partHeader.Set("Content-Type", contentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(fileBytes); err != nil {
		return nil, fmt.Errorf("write file data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("finalize multipart body: %w", err)
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Notion-Version", NotionVersion)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	if c.debug {
		fmt.Printf("→ POST %s (multipart, %d bytes)\n", url, body.Len())
	}

	ctx, cancel := context.WithTimeout(context.Background(), UploadTimeout)
	defer cancel()
	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if c.debug {
		fmt.Printf("← %d %s (%d bytes)\n", resp.StatusCode, resp.Status, len(respBody))
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upload failed (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// errorHint provides actionable suggestions for common API errors.
func errorHint(code, message string) string {
	switch code {
	case "object_not_found":
		return "Check the ID is correct and the page/database is shared with your integration"
	case "unauthorized":
		return "Run 'notion auth login' to authenticate, or check your token"
	case "restricted_resource":
		return "Your integration doesn't have access. Share the page/database with your integration in Notion"
	case "rate_limited":
		return "Too many requests. Wait a moment and try again"
	case "validation_error":
		if strings.Contains(message, "Internal integrations aren't owned") ||
			strings.Contains(message, "insert_content") {
			return internalIntegrationRootPageHint
		}
		if strings.Contains(message, "is not a property") {
			return "Check property names with 'notion db view <id>' or 'notion page props <id>'"
		}
		if strings.Contains(message, "body failed validation") {
			return "Check your input format. Use --debug for request details"
		}
	case "conflict_error":
		return "The resource was modified by another process. Retry the operation"
	case "internal_server_error", "service_unavailable":
		return "Notion's servers are having issues. Try again in a few minutes"
	}
	return ""
}

// internalIntegrationRootPageHint is the one-paragraph explanation the CLI
// prints when an internal integration tries to create a workspace-root page.
// It's a multi-line string because that's the shape most users need: the
// API error is accurate but not directly actionable.
const internalIntegrationRootPageHint = "Internal integrations can't create pages at the workspace root.\n" +
	"     Workaround: create (or pick) a parent page in the Notion UI, share\n" +
	"     it with this integration, then pass its ID as the parent:\n" +
	"         notion page create <shared-page-id> --title \"...\"\n" +
	"     To list pages shared with your integration: notion page list"
