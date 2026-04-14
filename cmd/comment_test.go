package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCommentAddWithMentionUsersUsesRichTextMentions(t *testing.T) {
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/comments" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "comment-123"})
	}))
	defer server.Close()

	t.Setenv("NOTION_BASE_URL", server.URL)
	t.Setenv("NOTION_TOKEN", "test-token")
	outputFormat = ""
	debugMode = false

	pageID := "31f4d69381a180629761e1f7c6dd6e7c"
	user1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	user2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	stdout, _, err := executeCommand(
		"comment", "add", pageID,
		"--mention-user", user1,
		"--mention-user", user2,
		"--text", "Please review this",
	)
	if err != nil {
		t.Fatalf("executeCommand returned error: %v", err)
	}
	if !strings.Contains(stdout, "Comment added") {
		t.Fatalf("stdout = %q, want success output", stdout)
	}

	parent, ok := gotBody["parent"].(map[string]interface{})
	if !ok {
		t.Fatalf("parent = %T, want map[string]interface{}", gotBody["parent"])
	}
	if parent["page_id"] != "31f4d693-81a1-8062-9761-e1f7c6dd6e7c" {
		t.Fatalf("page_id = %v, want dashed UUID", parent["page_id"])
	}

	richText, ok := gotBody["rich_text"].([]interface{})
	if !ok {
		t.Fatalf("rich_text = %T, want []interface{}", gotBody["rich_text"])
	}
	if len(richText) != 5 {
		t.Fatalf("len(rich_text) = %d, want 5", len(richText))
	}

	assertMention := func(idx int, wantID string) {
		t.Helper()
		item, ok := richText[idx].(map[string]interface{})
		if !ok {
			t.Fatalf("rich_text[%d] = %T, want map[string]interface{}", idx, richText[idx])
		}
		if item["type"] != "mention" {
			t.Fatalf("rich_text[%d].type = %v, want %q", idx, item["type"], "mention")
		}
		mention, ok := item["mention"].(map[string]interface{})
		if !ok {
			t.Fatalf("rich_text[%d].mention = %T, want map[string]interface{}", idx, item["mention"])
		}
		user, ok := mention["user"].(map[string]interface{})
		if !ok {
			t.Fatalf("rich_text[%d].mention.user = %T, want map[string]interface{}", idx, mention["user"])
		}
		if user["id"] != wantID {
			t.Fatalf("rich_text[%d].mention.user.id = %v, want %q", idx, user["id"], wantID)
		}
	}

	assertText := func(idx int, want string) {
		t.Helper()
		item, ok := richText[idx].(map[string]interface{})
		if !ok {
			t.Fatalf("rich_text[%d] = %T, want map[string]interface{}", idx, richText[idx])
		}
		if item["type"] != "text" {
			t.Fatalf("rich_text[%d].type = %v, want %q", idx, item["type"], "text")
		}
		text, ok := item["text"].(map[string]interface{})
		if !ok {
			t.Fatalf("rich_text[%d].text = %T, want map[string]interface{}", idx, item["text"])
		}
		if text["content"] != want {
			t.Fatalf("rich_text[%d].text.content = %v, want %q", idx, text["content"], want)
		}
	}

	assertMention(0, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	assertText(1, " ")
	assertMention(2, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	assertText(3, " ")
	assertText(4, "Please review this")
}

func TestCommentAddRequiresTextOrMentionUser(t *testing.T) {
	t.Setenv("NOTION_TOKEN", "test-token")
	outputFormat = ""
	debugMode = false

	_, _, err := executeCommand("comment", "add", "31f4d69381a180629761e1f7c6dd6e7c")
	if err == nil {
		t.Fatal("expected error for empty comment")
	}
	if !strings.Contains(err.Error(), "provide text or at least one --mention-user") {
		t.Fatalf("error = %v, want content validation", err)
	}
}
