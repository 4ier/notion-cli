package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSetMarkdownBody_DefaultReplace(t *testing.T) {
	body, err := buildSetMarkdownBody("hello", false, false, "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if body["type"] != "replace_content" {
		t.Errorf("type = %v, want replace_content (default)", body["type"])
	}
	rc := body["replace_content"].(map[string]interface{})
	if rc["new_str"] != "hello" {
		t.Errorf("new_str = %v", rc["new_str"])
	}
	if _, has := rc["allow_deleting_content"]; has {
		t.Error("allow_deleting_content should be omitted when not set")
	}
}

func TestBuildSetMarkdownBody_ReplaceWithAllowDelete(t *testing.T) {
	body, err := buildSetMarkdownBody("x", true, false, "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	rc := body["replace_content"].(map[string]interface{})
	if rc["allow_deleting_content"] != true {
		t.Error("allow_deleting_content should be true")
	}
}

func TestBuildSetMarkdownBody_Append(t *testing.T) {
	body, err := buildSetMarkdownBody("x", false, true, "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if body["type"] != "insert_content" {
		t.Errorf("type = %v", body["type"])
	}
	ins := body["insert_content"].(map[string]interface{})
	if ins["content"] != "x" {
		t.Errorf("content = %v", ins["content"])
	}
	if _, has := ins["after"]; has {
		t.Error("after should be omitted for plain append")
	}
}

func TestBuildSetMarkdownBody_After(t *testing.T) {
	body, err := buildSetMarkdownBody("x", false, false, "Start...End", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if body["type"] != "insert_content" {
		t.Errorf("type = %v", body["type"])
	}
	ins := body["insert_content"].(map[string]interface{})
	if ins["after"] != "Start...End" {
		t.Errorf("after = %v", ins["after"])
	}
}

func TestBuildSetMarkdownBody_Range(t *testing.T) {
	body, err := buildSetMarkdownBody("x", false, false, "", "old...gone", true)
	if err != nil {
		t.Fatal(err)
	}
	if body["type"] != "replace_content_range" {
		t.Errorf("type = %v", body["type"])
	}
	rr := body["replace_content_range"].(map[string]interface{})
	if rr["content_range"] != "old...gone" {
		t.Error("content_range missing")
	}
	if rr["allow_deleting_content"] != true {
		t.Error("allow_deleting_content should be true")
	}
}

func TestBuildSetMarkdownBody_MultipleModesRejected(t *testing.T) {
	_, err := buildSetMarkdownBody("x", true, true, "", "", false)
	if err == nil || !strings.Contains(err.Error(), "at most one") {
		t.Errorf("should reject multiple modes, got %v", err)
	}
}

func TestReadMarkdownSource_MissingBoth(t *testing.T) {
	_, err := readMarkdownSource("", "")
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("expected required error, got %v", err)
	}
}

func TestReadMarkdownSource_BothSetConflict(t *testing.T) {
	_, err := readMarkdownSource("path", "text")
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected conflict error, got %v", err)
	}
}

func TestReadMarkdownSource_Text(t *testing.T) {
	got, err := readMarkdownSource("", "inline md")
	if err != nil || got != "inline md" {
		t.Errorf("got %q err %v", got, err)
	}
}

func TestReadMarkdownSource_File(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "src.md")
	if err := os.WriteFile(p, []byte("from-file"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readMarkdownSource(p, "")
	if err != nil || got != "from-file" {
		t.Errorf("got %q err %v", got, err)
	}
}

func TestReadMarkdownSource_FileNotFound(t *testing.T) {
	_, err := readMarkdownSource("/nonexistent/file.md", "")
	if err == nil || !strings.Contains(err.Error(), "read") {
		t.Errorf("expected read error, got %v", err)
	}
}

func TestReadMarkdownSource_Stdin(t *testing.T) {
	// Redirect stdin to a pipe
	r, w, _ := os.Pipe()
	w.Write([]byte("from-stdin"))
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	got, err := readMarkdownSource("-", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-stdin" {
		t.Errorf("got %q", got)
	}
}
