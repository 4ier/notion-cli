package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type mockFileUploadClient struct {
	postPath         string
	postBody         interface{}
	patchPath        string
	patchBody        interface{}
	sendUploadID     string
	sendFileName     string
	sendContentType  string
	sendFileContents []byte
}

func (m *mockFileUploadClient) Post(path string, body interface{}) ([]byte, error) {
	m.postPath = path
	m.postBody = body
	return []byte(`{"id":"upload-123","status":"pending"}`), nil
}

func (m *mockFileUploadClient) Patch(path string, body interface{}) ([]byte, error) {
	m.patchPath = path
	m.patchBody = body
	return []byte(`{"results":[]}`), nil
}

func (m *mockFileUploadClient) UploadFileContent(uploadID, fileName, contentType string, fileBytes []byte) ([]byte, error) {
	m.sendUploadID = uploadID
	m.sendFileName = fileName
	m.sendContentType = contentType
	m.sendFileContents = append([]byte(nil), fileBytes...)
	return []byte(`{"id":"upload-123","status":"uploaded"}`), nil
}

func TestUploadFileCreatesUploadWithExpectedPayload(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "notes.txt")
	if err := os.WriteFile(filePath, []byte("hello world"), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	mock := &mockFileUploadClient{}
	outcome, err := uploadFile(mock, filePath, "")
	if err != nil {
		t.Fatalf("uploadFile returned error: %v", err)
	}

	if mock.postPath != "/v1/file_uploads" {
		t.Fatalf("postPath = %q, want %q", mock.postPath, "/v1/file_uploads")
	}

	body, ok := mock.postBody.(map[string]interface{})
	if !ok {
		t.Fatalf("postBody type = %T, want map[string]interface{}", mock.postBody)
	}
	if body["filename"] != "notes.txt" {
		t.Fatalf("filename = %v, want %q", body["filename"], "notes.txt")
	}
	if body["content_type"] != "text/plain; charset=utf-8" {
		t.Fatalf("content_type = %v, want %q", body["content_type"], "text/plain; charset=utf-8")
	}
	if body["content_length"] != int64(11) {
		t.Fatalf("content_length = %v, want %d", body["content_length"], 11)
	}
	if body["mode"] != "single_part" {
		t.Fatalf("mode = %v, want %q", body["mode"], "single_part")
	}

	if mock.sendUploadID != "upload-123" {
		t.Fatalf("sendUploadID = %q, want %q", mock.sendUploadID, "upload-123")
	}
	if mock.sendFileName != "notes.txt" {
		t.Fatalf("sendFileName = %q, want %q", mock.sendFileName, "notes.txt")
	}
	if mock.sendContentType != "text/plain; charset=utf-8" {
		t.Fatalf("sendContentType = %q, want %q", mock.sendContentType, "text/plain; charset=utf-8")
	}
	if string(mock.sendFileContents) != "hello world" {
		t.Fatalf("sendFileContents = %q, want %q", string(mock.sendFileContents), "hello world")
	}
	if mock.patchPath != "" {
		t.Fatalf("patchPath = %q, want empty", mock.patchPath)
	}
	if outcome.UploadID != "upload-123" {
		t.Fatalf("UploadID = %q, want %q", outcome.UploadID, "upload-123")
	}
	if outcome.Result["status"] != "uploaded" {
		t.Fatalf("status = %v, want %q", outcome.Result["status"], "uploaded")
	}
}

func TestUploadFileAttachesUploadedFileWhenToIsSet(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "diagram.png")
	if err := os.WriteFile(filePath, []byte("not-a-real-png"), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	mock := &mockFileUploadClient{}
	pageURL := "https://www.notion.so/skill-test-31f4d69381a180629761e1f7c6dd6e7c"

	outcome, err := uploadFile(mock, filePath, pageURL)
	if err != nil {
		t.Fatalf("uploadFile returned error: %v", err)
	}

	wantPath := "/v1/blocks/31f4d693-81a1-8062-9761-e1f7c6dd6e7c/children"
	if mock.patchPath != wantPath {
		t.Fatalf("patchPath = %q, want %q", mock.patchPath, wantPath)
	}

	reqBody, ok := mock.patchBody.(map[string]interface{})
	if !ok {
		t.Fatalf("patchBody type = %T, want map[string]interface{}", mock.patchBody)
	}
	children, ok := reqBody["children"].([]map[string]interface{})
	if !ok || len(children) != 1 {
		t.Fatalf("children = %#v, want 1 child block", reqBody["children"])
	}
	block := children[0]
	if block["type"] != "image" {
		t.Fatalf("block type = %v, want %q", block["type"], "image")
	}

	imageData, ok := block["image"].(map[string]interface{})
	if !ok {
		t.Fatalf("image block = %T, want map[string]interface{}", block["image"])
	}
	fileUploadData, ok := imageData["file_upload"].(map[string]interface{})
	if !ok {
		t.Fatalf("file_upload = %T, want map[string]interface{}", imageData["file_upload"])
	}
	if fileUploadData["id"] != "upload-123" {
		t.Fatalf("file_upload.id = %v, want %q", fileUploadData["id"], "upload-123")
	}

	if outcome.AttachedTo != "31f4d693-81a1-8062-9761-e1f7c6dd6e7c" {
		t.Fatalf("AttachedTo = %q", outcome.AttachedTo)
	}
	if outcome.BlockType != "image" {
		t.Fatalf("BlockType = %q, want %q", outcome.BlockType, "image")
	}

	raw, err := json.Marshal(outcome.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatal("result should marshal to valid JSON")
	}
}
