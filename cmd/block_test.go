package cmd

import (
	"strings"
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

func TestParseMarkdownTable(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		checkBlock func(t *testing.T, block map[string]interface{})
	}{
		{
			name: "basic GFM table",
			input: "| Name | Age |\n|------|-----|\n| Alice | 30 |\n| Bob | 25 |",
			wantCount: 1,
			checkBlock: func(t *testing.T, b map[string]interface{}) {
				if b["type"] != "table" {
					t.Errorf("type = %v, want table", b["type"])
				}
				tableData, ok := b["table"].(map[string]interface{})
				if !ok {
					t.Fatal("missing table data")
				}
				if tableData["table_width"] != 2 {
					t.Errorf("table_width = %v, want 2", tableData["table_width"])
				}
				if tableData["has_column_header"] != true {
					t.Errorf("has_column_header should be true")
				}
				children, ok := tableData["children"].([]map[string]interface{})
				if !ok {
					t.Fatal("missing table.children")
				}
				// header + 2 data rows = 3 rows (separator skipped)
				if len(children) != 3 {
					t.Errorf("got %d rows, want 3", len(children))
				}
				// Check first row type
				if children[0]["type"] != "table_row" {
					t.Errorf("child type = %v, want table_row", children[0]["type"])
				}
			},
		},
		{
			name: "table with alignment markers",
			input: "| Left | Center | Right |\n|:-----|:------:|------:|\n| a | b | c |",
			wantCount: 1,
			checkBlock: func(t *testing.T, b map[string]interface{}) {
				if b["type"] != "table" {
					t.Errorf("type = %v, want table", b["type"])
				}
				tableData := b["table"].(map[string]interface{})
				if tableData["table_width"] != 3 {
					t.Errorf("table_width = %v, want 3", tableData["table_width"])
				}
				children := tableData["children"].([]map[string]interface{})
				if len(children) != 2 { // header + 1 data row
					t.Errorf("got %d rows, want 2", len(children))
				}
			},
		},
		{
			name: "table is not parsed without separator",
			// No separator row → should NOT produce a table block
			input: "| Name | Age |\n| Alice | 30 |",
			wantCount: 2, // treated as 2 paragraphs
			checkBlock: func(t *testing.T, b map[string]interface{}) {
				if b["type"] == "table" {
					t.Error("should not produce table without separator")
				}
			},
		},
		{
			name: "table mixed with text",
			input: "Intro\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\nOutro",
			wantCount: 3, // paragraph + table + paragraph
			checkBlock: func(t *testing.T, b map[string]interface{}) {
				if b["type"] != "paragraph" {
					t.Errorf("first block type = %v, want paragraph", b["type"])
				}
			},
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
			if tt.checkBlock != nil && len(blocks) > 0 {
				tt.checkBlock(t, blocks[0])
			}
		})
	}
}

func TestParseInlineFormatting(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantParts int
		checkParts func(t *testing.T, parts []map[string]interface{})
	}{
		{
			name:      "plain text",
			input:     "hello world",
			wantParts: 1,
			checkParts: func(t *testing.T, parts []map[string]interface{}) {
				text := parts[0]["text"].(map[string]interface{})
				if text["content"] != "hello world" {
					t.Errorf("content = %v", text["content"])
				}
				if _, hasAnn := parts[0]["annotations"]; hasAnn {
					t.Error("plain text should have no annotations")
				}
			},
		},
		{
			name:      "bold",
			input:     "**bold text**",
			wantParts: 1,
			checkParts: func(t *testing.T, parts []map[string]interface{}) {
				ann := parts[0]["annotations"].(map[string]interface{})
				if ann["bold"] != true {
					t.Error("expected bold=true")
				}
				text := parts[0]["text"].(map[string]interface{})
				if text["content"] != "bold text" {
					t.Errorf("content = %v, want 'bold text'", text["content"])
				}
			},
		},
		{
			name:      "italic with asterisk",
			input:     "*italic*",
			wantParts: 1,
			checkParts: func(t *testing.T, parts []map[string]interface{}) {
				ann := parts[0]["annotations"].(map[string]interface{})
				if ann["italic"] != true {
					t.Error("expected italic=true")
				}
			},
		},
		{
			name:      "italic with underscore",
			input:     "_italic_",
			wantParts: 1,
			checkParts: func(t *testing.T, parts []map[string]interface{}) {
				ann := parts[0]["annotations"].(map[string]interface{})
				if ann["italic"] != true {
					t.Error("expected italic=true")
				}
			},
		},
		{
			name:      "inline code",
			input:     "`some code`",
			wantParts: 1,
			checkParts: func(t *testing.T, parts []map[string]interface{}) {
				ann := parts[0]["annotations"].(map[string]interface{})
				if ann["code"] != true {
					t.Error("expected code=true")
				}
			},
		},
		{
			name:      "strikethrough",
			input:     "~~deleted~~",
			wantParts: 1,
			checkParts: func(t *testing.T, parts []map[string]interface{}) {
				ann := parts[0]["annotations"].(map[string]interface{})
				if ann["strikethrough"] != true {
					t.Error("expected strikethrough=true")
				}
			},
		},
		{
			name:      "link",
			input:     "[Notion](https://notion.so)",
			wantParts: 1,
			checkParts: func(t *testing.T, parts []map[string]interface{}) {
				text := parts[0]["text"].(map[string]interface{})
				if text["content"] != "Notion" {
					t.Errorf("link text = %v, want 'Notion'", text["content"])
				}
				link := text["link"].(map[string]interface{})
				if link["url"] != "https://notion.so" {
					t.Errorf("link url = %v", link["url"])
				}
			},
		},
		{
			name:      "mixed inline",
			input:     "Hello **world** and *you*",
			wantParts: 4, // "Hello ", "world"(bold), " and ", "you"(italic)
		},
		{
			name:      "bold in table cell",
			input:     "**Header**",
			wantParts: 1,
			checkParts: func(t *testing.T, parts []map[string]interface{}) {
				ann := parts[0]["annotations"].(map[string]interface{})
				if ann["bold"] != true {
					t.Error("expected bold=true in table cell")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := parseInlineFormatting(tt.input)
			if len(parts) != tt.wantParts {
				t.Errorf("got %d parts, want %d", len(parts), tt.wantParts)
				for i, p := range parts {
					t.Logf("  part[%d]: %v", i, p)
				}
				return
			}
			if tt.checkParts != nil {
				tt.checkParts(t, parts)
			}
		})
	}
}

func TestIsTableSeparator(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"| --- | --- |", true},
		{"|---|---|", true},
		{"| :--- | ---: | :---: |", true},
		{"| Name | Age |", false},
		{"---", false},
		{"| - |", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isTableSeparator(tt.input)
			if got != tt.want {
				t.Errorf("isTableSeparator(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMapBlockTypeAliases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"h1", "heading_1"},
		{"h2", "heading_2"},
		{"h3", "heading_3"},
		{"heading1", "heading_1"},
		{"heading2", "heading_2"},
		{"heading3", "heading_3"},
		{"bullet", "bulleted_list_item"},
		{"numbered", "numbered_list_item"},
		{"todo", "to_do"},
		{"p", "paragraph"},
		{"paragraph", "paragraph"},
		{"quote", "quote"},
		{"code", "code"},
		{"callout", "callout"},
		{"divider", "divider"},
		// passthrough for native Notion types
		{"heading_1", "heading_1"},
		{"bulleted_list_item", "bulleted_list_item"},
		{"unknown_type", "unknown_type"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapBlockType(tt.input)
			if got != tt.want {
				t.Errorf("mapBlockType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildExternalImageBlock(t *testing.T) {
	t.Run("without caption", func(t *testing.T) {
		b := buildExternalImageBlock("https://example.com/a.png", "")
		if b["type"] != "image" {
			t.Fatalf("type = %v, want image", b["type"])
		}
		img, ok := b["image"].(map[string]interface{})
		if !ok {
			t.Fatal("image field not a map")
		}
		if img["type"] != "external" {
			t.Errorf("image.type = %v, want external", img["type"])
		}
		ext, _ := img["external"].(map[string]interface{})
		if ext["url"] != "https://example.com/a.png" {
			t.Errorf("external.url = %v", ext["url"])
		}
		if _, has := img["caption"]; has {
			t.Error("caption should be omitted when empty")
		}
	})

	t.Run("with caption", func(t *testing.T) {
		b := buildExternalImageBlock("https://x/y.png", "图 1-1 悬挂式")
		img, _ := b["image"].(map[string]interface{})
		cap, ok := img["caption"].([]map[string]interface{})
		if !ok || len(cap) != 1 {
			t.Fatalf("caption shape wrong: %#v", img["caption"])
		}
		txt, _ := cap[0]["text"].(map[string]interface{})
		if txt["content"] != "图 1-1 悬挂式" {
			t.Errorf("caption content = %v", txt["content"])
		}
	})
}

func TestValidateMediaFlags(t *testing.T) {
	tests := []struct {
		name     string
		imageURL string
		filePath string
		text     string
		wantErr  string
	}{
		{"no image url passes", "", "notes.md", "hi", ""},
		{"valid https", "https://x/y.png", "", "", ""},
		{"valid http", "http://x/y.png", "", "", ""},
		{"ftp rejected", "ftp://x/y.png", "", "", "http://"},
		{"plain text rejected", "not-a-url", "", "", "http://"},
		{"empty url treated as absent", "", "", "", ""},
		{"image-url with file conflicts", "https://x/y.png", "notes.md", "", "--file"},
		{"image-url with text conflicts", "https://x/y.png", "", "hello", "positional text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMediaFlags(tt.imageURL, tt.filePath, tt.text)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want contains %q", err.Error(), tt.wantErr)
			}
		})
	}
}
