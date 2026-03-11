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
	Use:   "upload <file-path>",
	Short: "Upload a file to Notion",
	Long: `Upload a file using Notion's file upload API (multi-step).

Examples:
  notion file upload ./document.pdf
  notion file upload ./image.png --to <page-id>`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getToken()
		if err != nil {
			return err
		}

		filePath := args[0]
		targetID, _ := cmd.Flags().GetString("to")

		c := client.New(token)
		c.SetDebug(debugMode)

		outcome, err := uploadFile(c, filePath, targetID)
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

func uploadFile(api fileUploadAPI, filePath, targetID string) (*fileUploadOutcome, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	fileName := filepath.Base(filePath)
	fileSize := fileInfo.Size()

	contentType, err := detectFileContentType(filePath)
	if err != nil {
		return nil, err
	}

	createData, err := api.Post("/v1/file_uploads", buildCreateFileUploadBody(fileName, contentType, fileSize))
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

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	sendData, err := api.UploadFileContent(uploadID, fileName, contentType, fileBytes)
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
		FileName:    fileName,
		FileSize:    fileSize,
		ContentType: contentType,
	}

	if strings.TrimSpace(targetID) == "" {
		return outcome, nil
	}

	resolvedTargetID := util.ResolveID(targetID)
	blockType := mediaBlockTypeForContentType(contentType)
	if _, err := api.Patch(fmt.Sprintf("/v1/blocks/%s/children", resolvedTargetID), buildFileUploadAppendRequest(uploadID, contentType)); err != nil {
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
	fileCmd.AddCommand(fileListCmd)
	fileCmd.AddCommand(fileUploadCmd)
}
