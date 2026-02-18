package cmd

import (
	"testing"
)

func TestParseMarkdownToBlocks(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		checkFirst func(t *testing.T, block map[string]interface{})
	}{
		{
			name:      "heading 1",
			input:     "# Hello",
			wantCount: 1,
			checkFirst: func(t *testing.T, b map[string]interface{}) {
				if b["type"] != "heading_1" {
					t.Errorf("type = %v, want heading_1", b["type"])
				}
			},
		},
		{
			name:      "heading 2",
			input:     "## Sub heading",
			wantCount: 1,
			checkFirst: func(t *testing.T, b map[string]interface{}) {
				if b["type"] != "heading_2" {
					t.Errorf("type = %v, want heading_2", b["type"])
				}
			},
		},
		{
			name:      "heading 3",
			input:     "### Sub sub heading",
			wantCount: 1,
			checkFirst: func(t *testing.T, b map[string]interface{}) {
				if b["type"] != "heading_3" {
					t.Errorf("type = %v, want heading_3", b["type"])
				}
			},
		},
		{
			name:      "bullet list",
			input:     "- item one\n- item two\n- item three",
			wantCount: 3,
			checkFirst: func(t *testing.T, b map[string]interface{}) {
				if b["type"] != "bulleted_list_item" {
					t.Errorf("type = %v, want bulleted_list_item", b["type"])
				}
			},
		},
		{
			name:      "bullet with asterisk",
			input:     "* item",
			wantCount: 1,
			checkFirst: func(t *testing.T, b map[string]interface{}) {
				if b["type"] != "bulleted_list_item" {
					t.Errorf("type = %v, want bulleted_list_item", b["type"])
				}
			},
		},
		{
			name:      "numbered list",
			input:     "1. first\n2. second",
			wantCount: 2,
			checkFirst: func(t *testing.T, b map[string]interface{}) {
				if b["type"] != "numbered_list_item" {
					t.Errorf("type = %v, want numbered_list_item", b["type"])
				}
			},
		},
		{
			name:      "quote",
			input:     "> This is a quote",
			wantCount: 1,
			checkFirst: func(t *testing.T, b map[string]interface{}) {
				if b["type"] != "quote" {
					t.Errorf("type = %v, want quote", b["type"])
				}
			},
		},
		{
			name:      "divider",
			input:     "---",
			wantCount: 1,
			checkFirst: func(t *testing.T, b map[string]interface{}) {
				if b["type"] != "divider" {
					t.Errorf("type = %v, want divider", b["type"])
				}
			},
		},
		{
			name:      "code block",
			input:     "```go\nfmt.Println(\"hello\")\n```",
			wantCount: 1,
			checkFirst: func(t *testing.T, b map[string]interface{}) {
				if b["type"] != "code" {
					t.Errorf("type = %v, want code", b["type"])
				}
				code, ok := b["code"].(map[string]interface{})
				if !ok {
					t.Fatal("missing code block data")
				}
				if code["language"] != "go" {
					t.Errorf("language = %v, want go", code["language"])
				}
			},
		},
		{
			name:      "code block no language",
			input:     "```\nsome code\n```",
			wantCount: 1,
			checkFirst: func(t *testing.T, b map[string]interface{}) {
				code := b["code"].(map[string]interface{})
				if code["language"] != "plain text" {
					t.Errorf("language = %v, want 'plain text'", code["language"])
				}
			},
		},
		{
			name:      "todo unchecked",
			input:     "- [ ] do this",
			wantCount: 1,
			checkFirst: func(t *testing.T, b map[string]interface{}) {
				if b["type"] != "to_do" {
					t.Errorf("type = %v, want to_do", b["type"])
				}
				td := b["to_do"].(map[string]interface{})
				if td["checked"] != false {
					t.Error("checked should be false")
				}
			},
		},
		{
			name:      "todo checked",
			input:     "- [x] done",
			wantCount: 1,
			checkFirst: func(t *testing.T, b map[string]interface{}) {
				td := b["to_do"].(map[string]interface{})
				if td["checked"] != true {
					t.Error("checked should be true")
				}
			},
		},
		{
			name:      "paragraph fallback",
			input:     "Just a regular paragraph",
			wantCount: 1,
			checkFirst: func(t *testing.T, b map[string]interface{}) {
				if b["type"] != "paragraph" {
					t.Errorf("type = %v, want paragraph", b["type"])
				}
			},
		},
		{
			name:      "empty lines skipped",
			input:     "\n\n\nHello\n\n\n",
			wantCount: 1,
		},
		{
			name:      "mixed content",
			input:     "# Title\n\nA paragraph.\n\n- bullet one\n- bullet two\n\n> a quote\n\n---",
			wantCount: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := parseMarkdownToBlocks(tt.input)
			if len(blocks) != tt.wantCount {
				t.Errorf("got %d blocks, want %d", len(blocks), tt.wantCount)
				for i, b := range blocks {
					t.Logf("  block[%d]: type=%v", i, b["type"])
				}
				return
			}
			if tt.checkFirst != nil && len(blocks) > 0 {
				tt.checkFirst(t, blocks[0])
			}
		})
	}
}

func TestMakeTextBlock(t *testing.T) {
	block := makeTextBlock("paragraph", "Hello World")
	if block["type"] != "paragraph" {
		t.Errorf("type = %v, want paragraph", block["type"])
	}
	if block["object"] != "block" {
		t.Errorf("object = %v, want block", block["object"])
	}
	p, ok := block["paragraph"].(map[string]interface{})
	if !ok {
		t.Fatal("missing paragraph data")
	}
	rt, ok := p["rich_text"].([]map[string]interface{})
	if !ok || len(rt) != 1 {
		t.Fatal("expected 1 rich_text element")
	}
	text := rt[0]["text"].(map[string]interface{})
	if text["content"] != "Hello World" {
		t.Errorf("content = %v, want 'Hello World'", text["content"])
	}
}
