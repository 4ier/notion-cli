package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestFilenameFromURL(t *testing.T) {
	cases := []struct {
		url  string
		disp string
		want string
	}{
		{"https://x.com/a/b/chart.png", "", "chart.png"},
		{"https://x.com/a/b/chart.png?foo=bar", "", "chart.png"},
		{"https://x.com/a/b/chart.png#frag", "", "chart.png"},
		{"https://x.com/download?id=123", `attachment; filename="report.pdf"`, "report.pdf"},
		{"https://x.com/", "", "download"},
	}
	for _, tc := range cases {
		got := filenameFromURL(tc.url, tc.disp)
		if got != tc.want {
			t.Errorf("filenameFromURL(%q, %q) = %q, want %q", tc.url, tc.disp, got, tc.want)
		}
	}
}

func TestSniffContentType_ByExtension(t *testing.T) {
	got := sniffContentType("chart.png", []byte{})
	if !strings.HasPrefix(got, "image/png") {
		t.Errorf("png by ext: %q", got)
	}
}

func TestSniffContentType_ByBytes(t *testing.T) {
	// PNG magic header.
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	got := sniffContentType("mystery", png)
	if !strings.HasPrefix(got, "image/png") {
		t.Errorf("png by bytes: %q", got)
	}
}

func TestLoadSourceFromStdin_RequiresName(t *testing.T) {
	_, err := loadSourceFromStdin("")
	if err == nil || !strings.Contains(err.Error(), "--name") {
		t.Errorf("expected --name required error, got %v", err)
	}
}

func TestLoadSourceFromStdin_ReadsBytes(t *testing.T) {
	// Redirect os.Stdin to a pipe with known content.
	r, w, _ := os.Pipe()
	w.Write([]byte("hello"))
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	src, err := loadSourceFromStdin("note.txt")
	if err != nil {
		t.Fatal(err)
	}
	if src.Name != "note.txt" {
		t.Errorf("name = %q", src.Name)
	}
	if string(src.Data) != "hello" {
		t.Errorf("data = %q", src.Data)
	}
	if src.Size != 5 {
		t.Errorf("size = %d", src.Size)
	}
	if !strings.HasPrefix(src.ContentType, "text/plain") {
		t.Errorf("content-type = %q", src.ContentType)
	}
}

func TestLoadSourceFromStdin_EmptyRejected(t *testing.T) {
	r, w, _ := os.Pipe()
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	_, err := loadSourceFromStdin("x.txt")
	if err == nil || !strings.Contains(err.Error(), "no data") {
		t.Errorf("expected no-data error, got %v", err)
	}
}

func TestLoadSourceFromURL_BasicFlow(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(png)
	}))
	defer srv.Close()

	src, err := loadSourceFromURL(srv.URL+"/a/b/chart.png?token=abc", "")
	if err != nil {
		t.Fatal(err)
	}
	if src.Name != "chart.png" {
		t.Errorf("name = %q, want chart.png", src.Name)
	}
	if src.ContentType != "image/png" {
		t.Errorf("content-type = %q", src.ContentType)
	}
	if string(src.Data) != string(png) {
		t.Errorf("data mismatch")
	}
}

func TestLoadSourceFromURL_NameOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("body"))
	}))
	defer srv.Close()

	src, err := loadSourceFromURL(srv.URL+"/opaque", "override.txt")
	if err != nil {
		t.Fatal(err)
	}
	if src.Name != "override.txt" {
		t.Errorf("name = %q", src.Name)
	}
}

func TestLoadSourceFromURL_ContentDispositionFilename(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="report.pdf"`)
		w.Write([]byte("%PDF-1.4\n"))
	}))
	defer srv.Close()

	src, err := loadSourceFromURL(srv.URL+"/download?id=123", "")
	if err != nil {
		t.Fatal(err)
	}
	if src.Name != "report.pdf" {
		t.Errorf("name = %q, want report.pdf", src.Name)
	}
}

func TestLoadSourceFromURL_HTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := loadSourceFromURL(srv.URL+"/missing.png", "")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got %v", err)
	}
}

// Integration: uploadFromAny with a URL source should hit the upload
// pipeline exactly the same way as a local-file source.
func TestUploadFromAny_URLUsesSamePipeline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("pngbytes"))
	}))
	defer srv.Close()

	mock := &mockFileUploadClient{}
	outcome, err := uploadFromAny(mock, srv.URL+"/chart.png", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if mock.postPath != "/v1/file_uploads" {
		t.Errorf("expected create POST, got %q", mock.postPath)
	}
	body := mock.postBody.(map[string]interface{})
	if body["filename"] != "chart.png" {
		t.Errorf("filename = %v", body["filename"])
	}
	if body["content_type"] != "image/png" {
		t.Errorf("content_type = %v", body["content_type"])
	}
	if outcome.FileName != "chart.png" {
		t.Errorf("outcome name = %q", outcome.FileName)
	}
	if string(mock.sendFileContents) != "pngbytes" {
		t.Errorf("body mismatch")
	}
}

// Integration: stdin source.
func TestUploadFromAny_StdinUsesSamePipeline(t *testing.T) {
	r, w, _ := os.Pipe()
	w.Write([]byte("hello from stdin"))
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	mock := &mockFileUploadClient{}
	outcome, err := uploadFromAny(mock, "-", "note.txt", "")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.FileName != "note.txt" {
		t.Errorf("name = %q", outcome.FileName)
	}
	if outcome.FileSize != 16 {
		t.Errorf("size = %d", outcome.FileSize)
	}
	if string(mock.sendFileContents) != "hello from stdin" {
		t.Errorf("body = %q", mock.sendFileContents)
	}
}
