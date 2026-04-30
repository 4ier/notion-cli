package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestNormalizeCodeLanguage_KnownEnum(t *testing.T) {
	// Values that are already in Notion's enum must pass through unchanged.
	for _, v := range []string{"typescript", "javascript", "python", "go", "bash", "yaml", "plain text", "c++", "c#"} {
		if got := normalizeCodeLanguage(v); got != v {
			t.Errorf("normalizeCodeLanguage(%q) = %q, want %q", v, got, v)
		}
	}
}

func TestNormalizeCodeLanguage_Aliases(t *testing.T) {
	cases := map[string]string{
		"ts":         "typescript",
		"tsx":        "typescript",
		"js":         "javascript",
		"jsx":        "javascript",
		"py":         "python",
		"sh":         "shell",
		"zsh":        "shell",
		"yml":        "yaml",
		"md":         "markdown",
		"rs":         "rust",
		"rb":         "ruby",
		"dockerfile": "docker",
		"proto":      "protobuf",
		"cpp":        "c++",
		"cs":         "c#",
		"golang":     "go",
		"node":       "javascript",
		"kt":         "kotlin",
		"ps1":        "powershell",
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			if got := normalizeCodeLanguage(input); got != want {
				t.Errorf("normalizeCodeLanguage(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestNormalizeCodeLanguage_CaseInsensitiveAndTrim(t *testing.T) {
	cases := map[string]string{
		"TypeScript": "typescript",
		"  GO  ":     "go",
		".py":        "python",
		"":           "plain text",
		"   ":        "plain text",
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			if got := normalizeCodeLanguage(input); got != want {
				t.Errorf("normalizeCodeLanguage(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestNormalizeCodeLanguage_UnknownFallsBackWithWarning(t *testing.T) {
	// Capture stderr to verify the warning is printed.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	got := normalizeCodeLanguage("gobbledygook")

	w.Close()
	os.Stderr = oldStderr
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if got != "plain text" {
		t.Errorf("unknown lang got %q, want 'plain text'", got)
	}
	if !strings.Contains(buf.String(), "unknown code language") {
		t.Errorf("expected stderr warning, got: %q", buf.String())
	}
}

// Regression: #22 — code fence with alias parses successfully to the
// canonical enum, not the raw alias.
func TestParseMarkdownToBlocks_CodeFenceAliases(t *testing.T) {
	input := "```ts\nconst x: number = 1;\n```\n\n```sh\necho hi\n```\n\n```yml\nkey: value\n```"
	blocks := parseMarkdownToBlocks(input)
	if len(blocks) != 3 {
		t.Fatalf("got %d blocks, want 3", len(blocks))
	}
	want := []string{"typescript", "shell", "yaml"}
	for i, w := range want {
		code, ok := blocks[i]["code"].(map[string]interface{})
		if !ok {
			t.Fatalf("block[%d] missing code data: %#v", i, blocks[i])
		}
		if code["language"] != w {
			t.Errorf("block[%d] language = %v, want %q", i, code["language"], w)
		}
	}
}
