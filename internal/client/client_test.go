package client

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestErrorHint(t *testing.T) {
	tests := []struct {
		code    string
		message string
		wantHas string // substring that should be in the hint
	}{
		{"object_not_found", "Could not find page", "shared with your integration"},
		{"unauthorized", "API token is invalid", "notion auth login"},
		{"restricted_resource", "Not allowed", "Share the page"},
		{"rate_limited", "Rate limited", "Wait"},
		{"validation_error", "is not a property that exists", "notion db view"},
		{"validation_error", "body failed validation", "--debug"},
		{"conflict_error", "conflict", "Retry"},
		{"internal_server_error", "error", "Notion's servers"},
		{"service_unavailable", "unavailable", "Try again"},
		{"unknown_code", "unknown", ""},
	}

	for i, tt := range tests {
		name := fmt.Sprintf("%d_%s", i, tt.code)
		t.Run(name, func(t *testing.T) {
			hint := errorHint(tt.code, tt.message)
			if tt.wantHas == "" {
				if hint != "" {
					t.Errorf("expected empty hint, got %q", hint)
				}
				return
			}
			if !strings.Contains(hint, tt.wantHas) {
				t.Errorf("hint = %q, want substring %q", hint, tt.wantHas)
			}
		})
	}
}

func TestUploadFileContentSetsMultipartPartContentType(t *testing.T) {
	var gotPath string
	var gotPartContentType string
	var gotFileName string
	var gotBody string

	c := &Client{
		token: "test-token",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				gotPath = req.URL.Path

				mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
				if err != nil {
					t.Fatalf("parse Content-Type: %v", err)
				}
				if mediaType != "multipart/form-data" {
					t.Fatalf("Content-Type = %q, want multipart/form-data", mediaType)
				}

				reader := multipart.NewReader(req.Body, params["boundary"])
				part, err := reader.NextPart()
				if err != nil {
					t.Fatalf("read multipart part: %v", err)
				}

				gotPartContentType = part.Header.Get("Content-Type")
				gotFileName = part.FileName()
				body, err := io.ReadAll(part)
				if err != nil {
					t.Fatalf("read part body: %v", err)
				}
				gotBody = string(body)

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"id":"upload-123","status":"uploaded"}`)),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}

	data, err := c.UploadFileContent("upload-123", "notes.txt", "text/plain; charset=utf-8", []byte("hello world"))
	if err != nil {
		t.Fatalf("UploadFileContent returned error: %v", err)
	}

	if gotPath != "/v1/file_uploads/upload-123/send" {
		t.Fatalf("path = %q, want %q", gotPath, "/v1/file_uploads/upload-123/send")
	}
	if gotPartContentType != "text/plain; charset=utf-8" {
		t.Fatalf("part Content-Type = %q, want %q", gotPartContentType, "text/plain; charset=utf-8")
	}
	if gotFileName != "notes.txt" {
		t.Fatalf("filename = %q, want %q", gotFileName, "notes.txt")
	}
	if gotBody != "hello world" {
		t.Fatalf("body = %q, want %q", gotBody, "hello world")
	}
	if string(data) != `{"id":"upload-123","status":"uploaded"}` {
		t.Fatalf("response = %q", string(data))
	}
}

func TestUploadFileContentEscapesQuotedFilename(t *testing.T) {
	var gotFileName string

	c := &Client{
		token: "test-token",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
				if err != nil {
					t.Fatalf("parse Content-Type: %v", err)
				}
				if mediaType != "multipart/form-data" {
					t.Fatalf("Content-Type = %q, want multipart/form-data", mediaType)
				}

				reader := multipart.NewReader(req.Body, params["boundary"])
				part, err := reader.NextPart()
				if err != nil {
					t.Fatalf("read multipart part: %v", err)
				}

				gotFileName = part.FileName()

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"id":"upload-123","status":"uploaded"}`)),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}

	if _, err := c.UploadFileContent("upload-123", `report "final".pdf`, "application/pdf", []byte("pdf-bytes")); err != nil {
		t.Fatalf("UploadFileContent returned error: %v", err)
	}

	if gotFileName != `report "final".pdf` {
		t.Fatalf("filename = %q, want %q", gotFileName, `report "final".pdf`)
	}
}
