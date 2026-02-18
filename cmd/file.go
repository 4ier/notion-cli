package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/4ier/notion-cli/internal/client"
	"github.com/4ier/notion-cli/internal/render"
	"github.com/spf13/cobra"
)

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

		// Verify file exists
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("file not found: %w", err)
		}

		fileName := filepath.Base(filePath)
		fileSize := fileInfo.Size()

		// Detect content type
		contentType := mime.TypeByExtension(filepath.Ext(filePath))
		if contentType == "" {
			// Read first 512 bytes for detection
			f, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("open file: %w", err)
			}
			buf := make([]byte, 512)
			n, _ := f.Read(buf)
			f.Close()
			contentType = http.DetectContentType(buf[:n])
		}

		c := client.New(token)
		c.SetDebug(debugMode)

		// Step 1: Create file upload
		createBody := map[string]interface{}{
			"file_name":    fileName,
			"content_type": contentType,
			"content_length": fileSize,
			"mode":         "single_part",
		}

		createData, err := c.Post("/v1/file_uploads", createBody)
		if err != nil {
			return fmt.Errorf("create file upload: %w", err)
		}

		var createResult map[string]interface{}
		if err := json.Unmarshal(createData, &createResult); err != nil {
			return err
		}

		uploadID, _ := createResult["id"].(string)
		if uploadID == "" {
			return fmt.Errorf("no upload ID returned")
		}

		// Step 2: Send file content
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		err = c.UploadFileContent(uploadID, fileName, contentType, fileBytes)
		if err != nil {
			return fmt.Errorf("send file content: %w", err)
		}

		if outputFormat == "json" {
			return render.JSON(createResult)
		}

		render.Title("✓", fmt.Sprintf("Uploaded: %s", fileName))
		render.Field("ID", uploadID)
		render.Field("Size", fmt.Sprintf("%d bytes", fileSize))

		return nil
	},
}

func init() {
	fileUploadCmd.Flags().String("to", "", "Target page ID to attach file to")
	fileCmd.AddCommand(fileListCmd)
	fileCmd.AddCommand(fileUploadCmd)
}
