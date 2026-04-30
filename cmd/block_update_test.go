package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildUpdateBlockBody_PlainText(t *testing.T) {
	body, err := buildUpdateBlockBody("paragraph", "hello", "", false)
	if err != nil {
		t.Fatal(err)
	}
	p := body["paragraph"].(map[string]interface{})
	rt := p["rich_text"].([]map[string]interface{})
	if len(rt) != 1 {
		t.Fatalf("expected 1 rich_text, got %d", len(rt))
	}
	text := rt[0]["text"].(map[string]interface{})
	if text["content"] != "hello" {
		t.Errorf("content = %v, want hello", text["content"])
	}
	if _, hasAnn := rt[0]["annotations"]; hasAnn {
		t.Error("plain text should have no annotations")
	}
}

func TestBuildUpdateBlockBody_TextWithMarkdown(t *testing.T) {
	body, err := buildUpdateBlockBody("paragraph", "See **bold** text", "", true)
	if err != nil {
		t.Fatal(err)
	}
	p := body["paragraph"].(map[string]interface{})
	rt := p["rich_text"].([]map[string]interface{})
	// "See " (plain), "bold" (bold), " text" (plain) → 3 parts
	if len(rt) != 3 {
		t.Fatalf("expected 3 rich_text parts, got %d: %+v", len(rt), rt)
	}
	// middle part should be bold
	ann, _ := rt[1]["annotations"].(map[string]interface{})
	if ann["bold"] != true {
		t.Errorf("middle part should be bold: %+v", rt[1])
	}
}

func TestBuildUpdateBlockBody_FileMarkdown(t *testing.T) {
	tmp := t.TempDir()
	mdPath := filepath.Join(tmp, "patch.md")
	if err := os.WriteFile(mdPath, []byte("Updated via **file**"), 0600); err != nil {
		t.Fatal(err)
	}

	body, err := buildUpdateBlockBody("paragraph", "", mdPath, false)
	if err != nil {
		t.Fatal(err)
	}
	p := body["paragraph"].(map[string]interface{})
	rt := p["rich_text"].([]map[string]interface{})
	if len(rt) < 2 {
		t.Errorf("expected markdown to produce inline parts, got %d", len(rt))
	}
}

func TestBuildUpdateBlockBody_FileCodeBlockPreservesLanguage(t *testing.T) {
	tmp := t.TempDir()
	mdPath := filepath.Join(tmp, "patch.md")
	if err := os.WriteFile(mdPath, []byte("```ts\nconst x: number = 1;\n```"), 0600); err != nil {
		t.Fatal(err)
	}

	body, err := buildUpdateBlockBody("code", "", mdPath, false)
	if err != nil {
		t.Fatal(err)
	}
	code := body["code"].(map[string]interface{})
	if code["language"] != "typescript" {
		t.Errorf("language = %v, want typescript (ts alias normalization)", code["language"])
	}
}

func TestBuildUpdateBlockBody_FileTypeMismatch(t *testing.T) {
	tmp := t.TempDir()
	mdPath := filepath.Join(tmp, "patch.md")
	// File has a heading, but target is paragraph — should error.
	if err := os.WriteFile(mdPath, []byte("# Heading"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := buildUpdateBlockBody("paragraph", "", mdPath, false)
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
	if !strings.Contains(err.Error(), "block type mismatch") {
		t.Errorf("error should mention type mismatch: %v", err)
	}
}

func TestBuildUpdateBlockBody_FileMustBeSingleBlock(t *testing.T) {
	tmp := t.TempDir()
	mdPath := filepath.Join(tmp, "patch.md")
	if err := os.WriteFile(mdPath, []byte("first block\n\nsecond block"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := buildUpdateBlockBody("paragraph", "", mdPath, false)
	if err == nil {
		t.Fatal("expected multi-block error")
	}
	if !strings.Contains(err.Error(), "exactly one block") {
		t.Errorf("error should mention one-block requirement: %v", err)
	}
}

func TestBuildUpdateBlockBody_FileMissing(t *testing.T) {
	_, err := buildUpdateBlockBody("paragraph", "", "/nonexistent/path.md", false)
	if err == nil || !strings.Contains(err.Error(), "read file") {
		t.Errorf("expected read error, got %v", err)
	}
}
