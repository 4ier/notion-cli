package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

const (
	BaseURL        = "https://api.notion.com"
	NotionVersion  = "2022-06-28"
	DefaultTimeout = 30 * time.Second
)

type Client struct {
	token      string
	httpClient *http.Client
	debug      bool
}

func New(token string) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

func (c *Client) SetDebug(debug bool) {
	c.debug = debug
}

func (c *Client) do(method, path string, body interface{}) ([]byte, error) {
	url := BaseURL + path

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
func (c *Client) AddComment(pageID, text string) ([]byte, error) {
	body := map[string]interface{}{
		"parent": map[string]interface{}{
			"page_id": pageID,
		},
		"rich_text": []map[string]interface{}{
			{"text": map[string]interface{}{"content": text}},
		},
	}
	return c.Post("/v1/comments", body)
}

// UploadFileContent sends file content to an existing file upload via multipart form.
func (c *Client) UploadFileContent(uploadID, fileName, contentType string, fileBytes []byte) error {
	url := BaseURL + fmt.Sprintf("/v1/file_uploads/%s/send", uploadID)

	// Build multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(fileBytes); err != nil {
		return fmt.Errorf("write file data: %w", err)
	}
	writer.Close()

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Notion-Version", NotionVersion)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	if c.debug {
		fmt.Printf("→ POST %s (multipart, %d bytes)\n", url, body.Len())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed (%d): %s", resp.StatusCode, string(respBody))
	}

	return nil
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
