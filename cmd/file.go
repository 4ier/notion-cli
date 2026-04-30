package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/4ier/notion-cli/internal/client"
	"github.com/4ier/notion-cli/internal/render"
	"github.com/4ier/notion-cli/internal/util"
	"github.com/spf13/cobra"
)

type fileUploadAPI interface {
	Post(path string, body interface{}) ([]byte, error)
	Patch(path string, body interface{}) ([]byte, error)
	UploadFileContent(uploadID, fileName, contentType string, fileBytes []byte) ([]byte, error)
}

type fileUploadOutcome struct {
	Result      map[string]interface{}
	UploadID    string
	FileName    string
	FileSize    int64
	ContentType string
	AttachedTo  string
	BlockType   string
}

// fileSource is everything uploadFile needs about the bytes it's about to
// send. It abstracts over local files, stdin, and http(s) URLs so the
// core upload flow stays single-path.
type fileSource struct {
	Name        string
	Size        int64
	ContentType string
	Data        []byte
}

var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "Work with file uploads",
}

var fileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List file uploads",
	Long: `List file uploads in the workspace.

Examples:
  notion file list
  notion file list --format json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		data, err := c.Get("/v1/file_uploads")
		if err != nil {
			return fmt.Errorf("list files: %w", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return err
		}

		if outputFormat == "json" {
			return render.JSON(result)
		}

		results, _ := result["results"].([]interface{})
		if len(results) == 0 {
			fmt.Println("No file uploads found.")
			return nil
		}

		headers := []string{"NAME", "ID", "STATUS", "CREATED"}
		var rows [][]string

		for _, r := range results {
			f, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := f["name"].(string)
			id, _ := f["id"].(string)
			status, _ := f["status"].(string)
			created, _ := f["created_time"].(string)
			if len(created) > 10 {
				created = created[:10]
			}
			rows = append(rows, []string{name, id, status, created})
		}

		render.Table(headers, rows)
		return nil
	},
}

var fileUploadCmd = &cobra.Command{
	Use:   "upload <file-path|url|->",
	Short: "Upload a file to Notion",
	Long: `Upload a file using Notion's file upload API (multi-step).

The source can be:
  <path>          a local file on disk
  <http(s) url>   a remote resource (streamed, not saved to disk)
  -               stdin (commonly used with a pipeline like curl | upload)

For non-path sources --name is recommended because we can't always
derive a sensible filename.

Examples:
  notion file upload ./document.pdf
  notion file upload ./image.png --to <page-id>
  notion file upload https://example.com/chart.png --name chart.png
  curl -sSL https://example.com/chart.png | notion file upload - --name chart.png`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		source := args[0]
		targetID, _ := cmd.Flags().GetString("to")
		nameOverride, _ := cmd.Flags().GetString("name")

		c := client.New(token)
		c.SetDebug(debugMode)

		outcome, err := uploadFromAny(c, source, nameOverride, targetID)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			return render.JSON(outcome.Result)
		}

		render.Title("✓", fmt.Sprintf("Uploaded: %s", outcome.FileName))
		render.Field("ID", outcome.UploadID)
		render.Field("Size", fmt.Sprintf("%d bytes", outcome.FileSize))
		if outcome.AttachedTo != "" {
			render.Field("Attached To", outcome.AttachedTo)
			render.Field("Block Type", outcome.BlockType)
		}

		return nil
	},
}

// uploadFromAny dispatches the source string to one of the three loaders
// (file / stdin / http) and then funnels into the existing upload path.
func uploadFromAny(api fileUploadAPI, source, nameOverride, targetID string) (*fileUploadOutcome, error) {
	var src *fileSource
	var err error

	switch {
	case source == "-":
		src, err = loadSourceFromStdin(nameOverride)
	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		src, err = loadSourceFromURL(source, nameOverride)
	default:
		src, err = loadSourceFromPath(source, nameOverride)
	}
	if err != nil {
		return nil, err
	}
	return uploadFromSource(api, src, targetID)
}

func loadSourceFromPath(filePath, nameOverride string) (*fileSource, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}
	name := filepath.Base(filePath)
	if nameOverride != "" {
		name = nameOverride
	}
	ct, err := detectFileContentType(filePath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return &fileSource{Name: name, Size: fi.Size(), ContentType: ct, Data: data}, nil
}

func loadSourceFromStdin(nameOverride string) (*fileSource, error) {
	if nameOverride == "" {
		return nil, fmt.Errorf("--name is required when reading from stdin (notion file upload - --name <filename>)")
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("stdin contained no data")
	}
	return &fileSource{
		Name:        nameOverride,
		Size:        int64(len(data)),
		ContentType: sniffContentType(nameOverride, data),
		Data:        data,
	}, nil
}

// loadSourceFromURL fetches a remote resource and returns it as a
// fileSource. Redirects follow curl-style defaults. No external auth
// mechanism is added here — callers who need auth can still do:
//
//	curl -H 'Authorization: Bearer X' ... | notion file upload - --name foo
//
// which covers the advanced case without bloating this command.
func loadSourceFromURL(url, nameOverride string) (*fileSource, error) {
	resp, err := http.Get(url) //nolint:gosec // URL comes from the user's own CLI arg
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download %s: HTTP %d %s", url, resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}

	name := nameOverride
	if name == "" {
		name = filenameFromURL(url, resp.Header.Get("Content-Disposition"))
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = sniffContentType(name, data)
	}

	return &fileSource{
		Name:        name,
		Size:        int64(len(data)),
		ContentType: ct,
		Data:        data,
	}, nil
}

// filenameFromURL derives a best-effort filename. Prefers the
// Content-Disposition attachment hint, then the last path segment.
func filenameFromURL(rawURL, contentDisposition string) string {
	if contentDisposition != "" {
		if _, params, err := mime.ParseMediaType(contentDisposition); err == nil {
			if name := params["filename"]; name != "" {
				return filepath.Base(name)
			}
		}
	}
	// Strip query string and hash.
	base := rawURL
	if i := strings.IndexAny(base, "?#"); i >= 0 {
		base = base[:i]
	}
	// Cut scheme and authority so filepath.Base doesn't return the host.
	if i := strings.Index(base, "://"); i >= 0 {
		rest := base[i+3:]
		if j := strings.Index(rest, "/"); j >= 0 {
			base = rest[j:]
		} else {
			base = ""
		}
	}
	base = strings.TrimRight(base, "/")
	base = filepath.Base(base)
	if base == "" || base == "/" || base == "." {
		return "download"
	}
	return base
}

// sniffContentType combines extension-based and byte-signature detection,
// falling back to application/octet-stream.
func sniffContentType(name string, data []byte) string {
	if ext := filepath.Ext(name); ext != "" {
		if ct := mime.TypeByExtension(ext); ct != "" {
			return ct
		}
	}
	if len(data) == 0 {
		return "application/octet-stream"
	}
	if len(data) > 512 {
		return http.DetectContentType(data[:512])
	}
	return http.DetectContentType(data)
}

// uploadFile is the legacy entry point kept for tests and the --to flow.
// New code should call uploadFromAny / uploadFromSource.
func uploadFile(api fileUploadAPI, filePath, targetID string) (*fileUploadOutcome, error) {
	src, err := loadSourceFromPath(filePath, "")
	if err != nil {
		return nil, err
	}
	return uploadFromSource(api, src, targetID)
}

// uploadFromSource runs the two-step create + send dance against Notion,
// then optionally attaches the new upload to a target page.
func uploadFromSource(api fileUploadAPI, src *fileSource, targetID string) (*fileUploadOutcome, error) {
	createData, err := api.Post("/v1/file_uploads", buildCreateFileUploadBody(src.Name, src.ContentType, src.Size))
	if err != nil {
		return nil, fmt.Errorf("create file upload: %w", err)
	}

	var createResult map[string]interface{}
	if err := json.Unmarshal(createData, &createResult); err != nil {
		return nil, fmt.Errorf("parse create response: %w", err)
	}

	uploadID, _ := createResult["id"].(string)
	if uploadID == "" {
		return nil, fmt.Errorf("no upload ID returned")
	}

	sendData, err := api.UploadFileContent(uploadID, src.Name, src.ContentType, src.Data)
	if err != nil {
		return nil, fmt.Errorf("send file content: %w", err)
	}

	result := createResult
	if len(sendData) > 0 {
		var sendResult map[string]interface{}
		if err := json.Unmarshal(sendData, &sendResult); err != nil {
			return nil, fmt.Errorf("parse send response: %w", err)
		}
		result = sendResult
	}

	outcome := &fileUploadOutcome{
		Result:      result,
		UploadID:    uploadID,
		FileName:    src.Name,
		FileSize:    src.Size,
		ContentType: src.ContentType,
	}

	if strings.TrimSpace(targetID) == "" {
		return outcome, nil
	}

	resolvedTargetID := util.ResolveID(targetID)
	blockType := mediaBlockTypeForContentType(src.ContentType)
	if _, err := api.Patch(fmt.Sprintf("/v1/blocks/%s/children", resolvedTargetID), buildFileUploadAppendRequest(uploadID, src.ContentType)); err != nil {
		return nil, fmt.Errorf("attach file to page: %w", err)
	}

	outcome.AttachedTo = resolvedTargetID
	outcome.BlockType = blockType
	outcome.Result["attached_to"] = resolvedTargetID
	outcome.Result["attached_block_type"] = blockType

	return outcome, nil
}

func detectFileContentType(filePath string) (string, error) {
	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType != "" {
		return contentType, nil
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read file header: %w", err)
	}

	return http.DetectContentType(buf[:n]), nil
}

func buildCreateFileUploadBody(fileName, contentType string, fileSize int64) map[string]interface{} {
	return map[string]interface{}{
		"filename":       fileName,
		"content_type":   contentType,
		"content_length": fileSize,
		"mode":           "single_part",
	}
}

func buildFileUploadAppendRequest(uploadID, contentType string) map[string]interface{} {
	return map[string]interface{}{
		"children": []map[string]interface{}{
			buildUploadedFileBlock(uploadID, contentType),
		},
	}
}

func buildUploadedFileBlock(uploadID, contentType string) map[string]interface{} {
	blockType := mediaBlockTypeForContentType(contentType)
	return map[string]interface{}{
		"object": "block",
		"type":   blockType,
		blockType: map[string]interface{}{
			"type":        "file_upload",
			"file_upload": map[string]interface{}{"id": uploadID},
		},
	}
}

func mediaBlockTypeForContentType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil && mediaType != "" {
		contentType = mediaType
	}

	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	case strings.HasPrefix(contentType, "audio/"):
		return "audio"
	case contentType == "application/pdf":
		return "pdf"
	default:
		return "file"
	}
}

func init() {
	fileUploadCmd.Flags().String("to", "", "Target page ID to attach file to")
	fileUploadCmd.Flags().String("name", "", "Override filename (required for stdin source, optional for URL)")
	fileCmd.AddCommand(fileListCmd)
	fileCmd.AddCommand(fileUploadCmd)
}
