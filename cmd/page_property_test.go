package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/4ier/notion-cli/internal/client"
)

func TestFindPropertyIDByName_Found(t *testing.T) {
	page := map[string]interface{}{
		"properties": map[string]interface{}{
			"Name":       map[string]interface{}{"id": "title", "type": "title"},
			"References": map[string]interface{}{"id": "rE%3F%7B", "type": "relation"},
		},
	}
	id, err := findPropertyIDByName(page, "References")
	if err != nil {
		t.Fatal(err)
	}
	if id != "rE%3F%7B" {
		t.Errorf("id = %q", id)
	}
}

func TestFindPropertyIDByName_NotFound(t *testing.T) {
	page := map[string]interface{}{
		"properties": map[string]interface{}{
			"Name": map[string]interface{}{"id": "title"},
		},
	}
	_, err := findPropertyIDByName(page, "Missing")
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "Missing") {
		t.Errorf("error should mention missing name: %v", err)
	}
}

func TestFindPropertyIDByName_NoPropertiesMap(t *testing.T) {
	page := map[string]interface{}{}
	_, err := findPropertyIDByName(page, "x")
	if err == nil || !strings.Contains(err.Error(), "no properties map") {
		t.Errorf("expected no-properties error, got %v", err)
	}
}

func TestSummarizePropertyItem_Types(t *testing.T) {
	cases := []struct {
		name string
		item map[string]interface{}
		want string
	}{
		{
			"relation",
			map[string]interface{}{"type": "relation", "relation": map[string]interface{}{"id": "abc-123"}},
			"abc-123",
		},
		{
			"rich_text",
			map[string]interface{}{"type": "rich_text", "rich_text": map[string]interface{}{"plain_text": "hello"}},
			"hello",
		},
		{
			"title",
			map[string]interface{}{"type": "title", "title": map[string]interface{}{"plain_text": "Page Title"}},
			"Page Title",
		},
		{
			"number",
			map[string]interface{}{"type": "number", "number": float64(42)},
			"42",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := summarizePropertyItem(tc.item)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// Integration: spin up a fake Notion that returns 3 pages of relation
// results and assert fetchPagePropertyAllPages walks all of them.
func TestFetchPagePropertyAllPages_FollowsCursors(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		cursor := r.URL.Query().Get("start_cursor")
		var body map[string]interface{}
		switch cursor {
		case "":
			body = map[string]interface{}{
				"object": "list",
				"type":   "property_item",
				"results": []interface{}{
					map[string]interface{}{"type": "relation", "relation": map[string]interface{}{"id": "a"}},
					map[string]interface{}{"type": "relation", "relation": map[string]interface{}{"id": "b"}},
				},
				"has_more":    true,
				"next_cursor": "cursor-2",
			}
		case "cursor-2":
			body = map[string]interface{}{
				"object": "list",
				"results": []interface{}{
					map[string]interface{}{"type": "relation", "relation": map[string]interface{}{"id": "c"}},
				},
				"has_more":    true,
				"next_cursor": "cursor-3",
			}
		case "cursor-3":
			body = map[string]interface{}{
				"object": "list",
				"results": []interface{}{
					map[string]interface{}{"type": "relation", "relation": map[string]interface{}{"id": "d"}},
				},
				"has_more":    false,
				"next_cursor": nil,
			}
		}
		json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	c := client.NewWithBaseURL("fake-token", srv.URL)
	result, err := fetchPagePropertyAllPages(c, "page-x", "prop-y", 100)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 3 {
		t.Errorf("expected 3 API calls, got %d", requests)
	}
	results, _ := result["results"].([]interface{})
	if len(results) != 4 {
		t.Errorf("expected 4 merged results, got %d", len(results))
	}
	if result["has_more"] != false {
		t.Error("merged result should report has_more=false")
	}
}

func TestFetchPagePropertyAllPages_PropertyItemNoPagination(t *testing.T) {
	// Non-paginated property (title, number, select) returns a single
	// object with `object: "property_item"`.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "property_item",
			"type":   "number",
			"number": float64(42),
		})
	}))
	defer srv.Close()

	c := client.NewWithBaseURL("fake-token", srv.URL)
	result, err := fetchPagePropertyAllPages(c, "page-x", "prop-y", 100)
	if err != nil {
		t.Fatal(err)
	}
	if result["object"] != "property_item" {
		t.Errorf("expected property_item passthrough, got %v", result["object"])
	}
	if result["number"].(float64) != 42 {
		t.Errorf("expected number=42")
	}
}
