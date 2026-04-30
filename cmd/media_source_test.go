package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newMediaCmd() *cobra.Command {
	c := &cobra.Command{Use: "test"}
	registerMediaFlags(c)
	return c
}

type fakeMediaClient struct {
	mockFileUploadClient
}

func (f *fakeMediaClient) Post(path string, body interface{}) ([]byte, error) {
	return f.mockFileUploadClient.Post(path, body)
}
func (f *fakeMediaClient) Patch(path string, body interface{}) ([]byte, error) {
	return f.mockFileUploadClient.Patch(path, body)
}
func (f *fakeMediaClient) UploadFileContent(uploadID, fileName, contentType string, fileBytes []byte) ([]byte, error) {
	return f.mockFileUploadClient.UploadFileContent(uploadID, fileName, contentType, fileBytes)
}

func TestResolveMediaSource_NoneSet(t *testing.T) {
	cmd := newMediaCmd()
	got, err := resolveMediaSource(cmd, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.IsActive() {
		t.Error("no flags set should produce inactive source")
	}
}

func TestResolveMediaSource_EveryKindAndMode(t *testing.T) {
	cases := []struct {
		flag     string
		value    string
		wantKind string
		wantMode string
	}{
		{"image-url", "https://x/y.png", "image", "external"},
		{"image-file", "/tmp/a.png", "image", "file"},
		{"image-upload", "abc-123", "image", "upload"},
		{"video-url", "https://x/y.mp4", "video", "external"},
		{"video-file", "/tmp/a.mp4", "video", "file"},
		{"video-upload", "abc-123", "video", "upload"},
		{"audio-url", "https://x/y.mp3", "audio", "external"},
		{"audio-file", "/tmp/a.mp3", "audio", "file"},
		{"audio-upload", "abc-123", "audio", "upload"},
		{"file-url", "https://x/y.zip", "file", "external"},
		{"file-file", "/tmp/a.zip", "file", "file"},
		{"file-upload", "abc-123", "file", "upload"},
		{"pdf-url", "https://x/y.pdf", "pdf", "external"},
		{"pdf-file", "/tmp/a.pdf", "pdf", "file"},
		{"pdf-upload", "abc-123", "pdf", "upload"},
	}
	for _, tc := range cases {
		t.Run(tc.flag, func(t *testing.T) {
			cmd := newMediaCmd()
			if err := cmd.Flags().Set(tc.flag, tc.value); err != nil {
				t.Fatal(err)
			}
			got, err := resolveMediaSource(cmd, "", "")
			if err != nil {
				t.Fatal(err)
			}
			if got.kind != tc.wantKind || got.mode != tc.wantMode || got.value != tc.value {
				t.Errorf("got %+v, want {kind:%s mode:%s value:%s}", got, tc.wantKind, tc.wantMode, tc.value)
			}
		})
	}
}

func TestResolveMediaSource_MutualExclusion(t *testing.T) {
	cmd := newMediaCmd()
	cmd.Flags().Set("image-url", "https://x/y.png")
	cmd.Flags().Set("pdf-upload", "abc-123")
	_, err := resolveMediaSource(cmd, "", "")
	if err == nil {
		t.Fatal("expected error for two active sources")
	}
	if !strings.Contains(err.Error(), "at most one media source") {
		t.Errorf("unexpected error text: %v", err)
	}
}

func TestResolveMediaSource_ConflictsWithTextAndFile(t *testing.T) {
	cmd := newMediaCmd()
	cmd.Flags().Set("image-file", "/tmp/x.png")
	if _, err := resolveMediaSource(cmd, "notes.md", ""); err == nil || !strings.Contains(err.Error(), "--file") {
		t.Errorf("should conflict with --file, got: %v", err)
	}
	cmd2 := newMediaCmd()
	cmd2.Flags().Set("image-upload", "abc")
	if _, err := resolveMediaSource(cmd2, "", "hello"); err == nil || !strings.Contains(err.Error(), "positional") {
		t.Errorf("should conflict with text, got: %v", err)
	}
}

func TestResolveMediaSource_RejectsNonHTTPExternal(t *testing.T) {
	cmd := newMediaCmd()
	cmd.Flags().Set("image-url", "ftp://x/y.png")
	_, err := resolveMediaSource(cmd, "", "")
	if err == nil || !strings.Contains(err.Error(), "http://") {
		t.Errorf("ftp url should be rejected, got: %v", err)
	}
}

func TestResolveMediaSource_CaptionWithoutSource(t *testing.T) {
	cmd := newMediaCmd()
	cmd.Flags().Set("caption", "fig.1")
	if _, err := resolveMediaSource(cmd, "", ""); err == nil {
		t.Error("standalone --caption should error")
	}
}

func TestBuildExternalMediaBlock_Types(t *testing.T) {
	for _, kind := range mediaKinds {
		b := buildExternalMediaBlock(kind, "https://x/y", "c")
		if b["type"] != kind {
			t.Errorf("type=%v, want %s", b["type"], kind)
		}
		m := b[kind].(map[string]interface{})
		if m["type"] != "external" {
			t.Errorf("%s.type=%v, want external", kind, m["type"])
		}
		if _, ok := m["caption"]; !ok {
			t.Errorf("%s caption missing", kind)
		}
	}
}

func TestBuildFileUploadMediaBlock_Types(t *testing.T) {
	for _, kind := range mediaKinds {
		b := buildFileUploadMediaBlock(kind, "u-1", "")
		if b["type"] != kind {
			t.Errorf("type=%v, want %s", b["type"], kind)
		}
		m := b[kind].(map[string]interface{})
		if m["type"] != "file_upload" {
			t.Errorf("%s.type=%v, want file_upload", kind, m["type"])
		}
		fu := m["file_upload"].(map[string]interface{})
		if fu["id"] != "u-1" {
			t.Errorf("%s file_upload.id=%v", kind, fu["id"])
		}
		if _, ok := m["caption"]; ok {
			t.Errorf("%s caption should be omitted when empty", kind)
		}
	}
}

func TestMediaSource_Build_Upload(t *testing.T) {
	src := &mediaSource{kind: "pdf", mode: "upload", value: "upload-xyz", caption: "spec"}
	block, err := src.Build(&fakeMediaClient{})
	if err != nil {
		t.Fatal(err)
	}
	if block["type"] != "pdf" {
		t.Errorf("type = %v", block["type"])
	}
	m := block["pdf"].(map[string]interface{})
	if m["file_upload"].(map[string]interface{})["id"] != "upload-xyz" {
		t.Errorf("wrong upload id: %v", m)
	}
}

func TestMediaSource_Build_External(t *testing.T) {
	src := &mediaSource{kind: "image", mode: "external", value: "https://x/y.png"}
	block, err := src.Build(&fakeMediaClient{})
	if err != nil {
		t.Fatal(err)
	}
	m := block["image"].(map[string]interface{})
	if m["external"].(map[string]interface{})["url"] != "https://x/y.png" {
		t.Errorf("wrong url: %v", m)
	}
}

// Regression: --image-file should upload the local file and then reference
// the returned file_upload id in a single block, without attaching
// anywhere else (attaching is the caller's job via appendChildrenBatched).
func TestMediaSource_Build_File_UploadsAndReferences(t *testing.T) {
	tmp := t.TempDir()
	localPath := fmt.Sprintf("%s/chart.png", tmp)
	if err := writeTempFile(localPath, "not-a-png"); err != nil {
		t.Fatal(err)
	}
	mock := &fakeMediaClient{}
	src := &mediaSource{kind: "image", mode: "file", value: localPath}
	block, err := src.Build(mock)
	if err != nil {
		t.Fatal(err)
	}
	if mock.postPath != "/v1/file_uploads" {
		t.Errorf("expected file_uploads POST, got %q", mock.postPath)
	}
	if mock.patchPath != "" {
		t.Errorf("media source should NOT patch a parent; got path=%q", mock.patchPath)
	}
	m := block["image"].(map[string]interface{})
	if m["file_upload"].(map[string]interface{})["id"] != "upload-123" {
		t.Errorf("expected upload-123, got %v", m["file_upload"])
	}
}

func writeTempFile(path, contents string) error {
	return os.WriteFile(path, []byte(contents), 0600)
}
