package cmd

import (
	"testing"
)

func TestMapBlockType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"heading1", "heading_1"},
		{"h1", "heading_1"},
		{"heading2", "heading_2"},
		{"h2", "heading_2"},
		{"heading3", "heading_3"},
		{"h3", "heading_3"},
		{"bullet", "bulleted_list_item"},
		{"numbered", "numbered_list_item"},
		{"todo", "to_do"},
		{"paragraph", "paragraph"},
		{"p", "paragraph"},
		{"quote", "quote"},
		{"code", "code"},
		{"callout", "callout"},
		{"divider", "divider"},
		// passthrough for native types
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

func TestBuildPropertyValue(t *testing.T) {
	tests := []struct {
		name     string
		propType string
		value    string
	}{
		{"title", "title", "Hello World"},
		{"rich_text", "rich_text", "Some text"},
		{"number", "number", "42"},
		{"select", "select", "Option A"},
		{"multi_select", "multi_select", "tag1,tag2,tag3"},
		{"status", "status", "In Progress"},
		{"date", "date", "2026-02-19"},
		{"checkbox_true", "checkbox", "true"},
		{"checkbox_false", "checkbox", "false"},
		{"checkbox_yes", "checkbox", "yes"},
		{"checkbox_no", "checkbox", "no"},
		{"url", "url", "https://example.com"},
		{"email", "email", "test@example.com"},
		{"phone", "phone_number", "+1234567890"},
		{"unknown_fallback", "unknown_type", "fallback text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildPropertyValue(tt.propType, tt.value)
			if result == nil {
				t.Errorf("buildPropertyValue(%q, %q) returned nil", tt.propType, tt.value)
			}
			// Type assertion to verify it's a map
			m, ok := result.(map[string]interface{})
			if !ok {
				t.Errorf("buildPropertyValue(%q, %q) should return map, got %T", tt.propType, tt.value, result)
				return
			}
			if len(m) == 0 {
				t.Errorf("buildPropertyValue(%q, %q) returned empty map", tt.propType, tt.value)
			}
		})
	}
}

func TestBuildPropertyValueCheckbox(t *testing.T) {
	// Verify checkbox boolean values
	trueInputs := []string{"true", "1", "yes"}
	falseInputs := []string{"false", "0", "no", ""}

	for _, input := range trueInputs {
		result := buildPropertyValue("checkbox", input)
		m := result.(map[string]interface{})
		if m["checkbox"] != true {
			t.Errorf("buildPropertyValue(\"checkbox\", %q) = %v, want true", input, m["checkbox"])
		}
	}

	for _, input := range falseInputs {
		result := buildPropertyValue("checkbox", input)
		m := result.(map[string]interface{})
		if m["checkbox"] != false {
			t.Errorf("buildPropertyValue(\"checkbox\", %q) = %v, want false", input, m["checkbox"])
		}
	}
}

func TestBuildPropertyValueMultiSelect(t *testing.T) {
	result := buildPropertyValue("multi_select", "tag1,tag2,tag3")
	m := result.(map[string]interface{})
	options, ok := m["multi_select"].([]map[string]interface{})
	if !ok {
		t.Fatalf("multi_select should be []map, got %T", m["multi_select"])
	}
	if len(options) != 3 {
		t.Errorf("multi_select should have 3 options, got %d", len(options))
	}
	if options[0]["name"] != "tag1" {
		t.Errorf("first option name = %q, want \"tag1\"", options[0]["name"])
	}
}

func TestBuildPropertyValueTitle(t *testing.T) {
	result := buildPropertyValue("title", "My Title")
	m := result.(map[string]interface{})
	titleArr, ok := m["title"].([]map[string]interface{})
	if !ok {
		t.Fatalf("title should be []map, got %T", m["title"])
	}
	if len(titleArr) != 1 {
		t.Fatalf("title should have 1 element, got %d", len(titleArr))
	}
	textObj, ok := titleArr[0]["text"].(map[string]interface{})
	if !ok {
		t.Fatalf("title[0].text should be map, got %T", titleArr[0]["text"])
	}
	if textObj["content"] != "My Title" {
		t.Errorf("title content = %q, want \"My Title\"", textObj["content"])
	}
}

func TestExtractPropertyValue(t *testing.T) {
	tests := []struct {
		name string
		prop map[string]interface{}
		want string
	}{
		{
			name: "select",
			prop: map[string]interface{}{
				"type":   "select",
				"select": map[string]interface{}{"name": "Done"},
			},
			want: "Done",
		},
		{
			name: "status",
			prop: map[string]interface{}{
				"type":   "status",
				"status": map[string]interface{}{"name": "In Progress"},
			},
			want: "In Progress",
		},
		{
			name: "number",
			prop: map[string]interface{}{
				"type":   "number",
				"number": 42.0,
			},
			want: "42",
		},
		{
			name: "number_nil",
			prop: map[string]interface{}{
				"type":   "number",
				"number": nil,
			},
			want: "",
		},
		{
			name: "checkbox_true",
			prop: map[string]interface{}{
				"type":     "checkbox",
				"checkbox": true,
			},
			want: "✓",
		},
		{
			name: "checkbox_false",
			prop: map[string]interface{}{
				"type":     "checkbox",
				"checkbox": false,
			},
			want: "✗",
		},
		{
			name: "url",
			prop: map[string]interface{}{
				"type": "url",
				"url":  "https://example.com",
			},
			want: "https://example.com",
		},
		{
			name: "email",
			prop: map[string]interface{}{
				"type":  "email",
				"email": "test@example.com",
			},
			want: "test@example.com",
		},
		{
			name: "date_simple",
			prop: map[string]interface{}{
				"type": "date",
				"date": map[string]interface{}{"start": "2026-02-19", "end": nil},
			},
			want: "2026-02-19",
		},
		{
			name: "date_range",
			prop: map[string]interface{}{
				"type": "date",
				"date": map[string]interface{}{"start": "2026-02-19", "end": "2026-02-20"},
			},
			want: "2026-02-19 → 2026-02-20",
		},
		{
			name: "rich_text",
			prop: map[string]interface{}{
				"type": "rich_text",
				"rich_text": []interface{}{
					map[string]interface{}{"plain_text": "Hello "},
					map[string]interface{}{"plain_text": "World"},
				},
			},
			want: "Hello World",
		},
		{
			name: "title",
			prop: map[string]interface{}{
				"type": "title",
				"title": []interface{}{
					map[string]interface{}{"plain_text": "Page Title"},
				},
			},
			want: "Page Title",
		},
		{
			name: "multi_select",
			prop: map[string]interface{}{
				"type": "multi_select",
				"multi_select": []interface{}{
					map[string]interface{}{"name": "tag1"},
					map[string]interface{}{"name": "tag2"},
				},
			},
			want: "tag1, tag2",
		},
		{
			name: "people",
			prop: map[string]interface{}{
				"type": "people",
				"people": []interface{}{
					map[string]interface{}{"name": "Alice"},
					map[string]interface{}{"name": "Bob"},
				},
			},
			want: "Alice, Bob",
		},
		{
			name: "created_time",
			prop: map[string]interface{}{
				"type":         "created_time",
				"created_time": "2026-02-19T00:00:00.000Z",
			},
			want: "2026-02-19T00:00:00.000Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPropertyValue(tt.prop)
			if got != tt.want {
				t.Errorf("extractPropertyValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractPlainTextFromRichText(t *testing.T) {
	tests := []struct {
		name  string
		input []interface{}
		want  string
	}{
		{
			name:  "empty",
			input: []interface{}{},
			want:  "",
		},
		{
			name: "single segment",
			input: []interface{}{
				map[string]interface{}{"plain_text": "Hello"},
			},
			want: "Hello",
		},
		{
			name: "multiple segments",
			input: []interface{}{
				map[string]interface{}{"plain_text": "Hello "},
				map[string]interface{}{"plain_text": "World"},
			},
			want: "Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPlainTextFromRichText(tt.input)
			if got != tt.want {
				t.Errorf("extractPlainTextFromRichText() = %q, want %q", got, tt.want)
			}
		})
	}
}
