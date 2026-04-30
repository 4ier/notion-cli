package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestChunkChildren(t *testing.T) {
	tests := []struct {
		name string
		size int
		want []int // sizes of each chunk
	}{
		{"empty", 0, nil},
		{"under limit", 50, []int{50}},
		{"exactly at limit", 100, []int{100}},
		{"one over limit", 101, []int{100, 1}},
		{"double limit", 200, []int{100, 100}},
		{"triple limit + tail", 253, []int{100, 100, 53}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			children := make([]map[string]interface{}, tt.size)
			for i := range children {
				children[i] = map[string]interface{}{"i": i}
			}
			got := chunkChildren(children)
			if tt.size == 0 {
				if len(got) != 1 || len(got[0]) != 0 {
					t.Errorf("empty input: got %d chunks", len(got))
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d chunks, want %d", len(got), len(tt.want))
			}
			for i, n := range tt.want {
				if len(got[i]) != n {
					t.Errorf("chunk %d size = %d, want %d", i, len(got[i]), n)
				}
			}
			// Verify order preserved across chunks.
			seen := 0
			for _, batch := range got {
				for _, b := range batch {
					if b["i"] != seen {
						t.Fatalf("order broken: batch item i=%v, want %d", b["i"], seen)
					}
					seen++
				}
			}
		})
	}
}

func TestHandleOversizedBlocks_CodeSplit(t *testing.T) {
	big := strings.Repeat("a", 3000)
	block := map[string]interface{}{
		"object": "block",
		"type":   "code",
		"code": map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"text": map[string]interface{}{"content": big}},
			},
			"language": "typescript",
		},
	}

	result, err := handleOversizedBlocks([]map[string]interface{}{block}, oversizeSplit)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("got %d blocks, want 2 (3000 / 2000 = 2 chunks)", len(result))
	}
	// Each chunk must preserve the code type and language, and be ≤2000 chars.
	for i, b := range result {
		if b["type"] != "code" {
			t.Errorf("chunk %d type = %v, want code", i, b["type"])
		}
		code := b["code"].(map[string]interface{})
		if code["language"] != "typescript" {
			t.Errorf("chunk %d language = %v, want typescript", i, code["language"])
		}
		rt := code["rich_text"].([]map[string]interface{})
		content := rt[0]["text"].(map[string]interface{})["content"].(string)
		if len(content) > maxRichTextContentLen {
			t.Errorf("chunk %d content len = %d, > limit %d", i, len(content), maxRichTextContentLen)
		}
	}
}

func TestHandleOversizedBlocks_NewlineBoundaryPreferred(t *testing.T) {
	// Build a 3000-char block with a newline around position 1800 (within
	// the "not absurdly early" window, so it should be used as the cut).
	first := strings.Repeat("a", 1800)
	second := strings.Repeat("b", 1200)
	content := first + "\n" + second
	block := map[string]interface{}{
		"object": "block",
		"type":   "code",
		"code": map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"text": map[string]interface{}{"content": content}},
			},
			"language": "plain text",
		},
	}
	result, err := handleOversizedBlocks([]map[string]interface{}{block}, oversizeSplit)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("got %d blocks, want 2", len(result))
	}
	first0 := result[0]["code"].(map[string]interface{})["rich_text"].([]map[string]interface{})[0]["text"].(map[string]interface{})["content"].(string)
	if first0 != first {
		t.Errorf("first chunk should be cut at newline: len=%d (want %d)", len(first0), len(first))
	}
}

func TestHandleOversizedBlocks_Truncate(t *testing.T) {
	big := strings.Repeat("x", 2500)
	block := map[string]interface{}{
		"object": "block",
		"type":   "paragraph",
		"paragraph": map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"text": map[string]interface{}{"content": big}},
			},
		},
	}
	result, err := handleOversizedBlocks([]map[string]interface{}{block}, oversizeTruncate)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("truncate should keep 1 block, got %d", len(result))
	}
	rt := result[0]["paragraph"].(map[string]interface{})["rich_text"].([]map[string]interface{})
	content := rt[0]["text"].(map[string]interface{})["content"].(string)
	if len(content) != maxRichTextContentLen {
		t.Errorf("truncated len = %d, want %d", len(content), maxRichTextContentLen)
	}
}

func TestHandleOversizedBlocks_Fail(t *testing.T) {
	big := strings.Repeat("y", 2500)
	block := map[string]interface{}{
		"object": "block",
		"type":   "code",
		"code": map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"text": map[string]interface{}{"content": big}},
			},
			"language": "go",
		},
	}
	_, err := handleOversizedBlocks([]map[string]interface{}{block}, oversizeFail)
	if err == nil {
		t.Fatal("expected error in fail mode")
	}
	if !strings.Contains(err.Error(), "2000-char") {
		t.Errorf("error should mention the limit: %v", err)
	}
}

func TestHandleOversizedBlocks_PassThroughSmall(t *testing.T) {
	block := map[string]interface{}{
		"object":    "block",
		"type":      "paragraph",
		"paragraph": map[string]interface{}{"rich_text": []map[string]interface{}{{"text": map[string]interface{}{"content": "short"}}}},
	}
	result, err := handleOversizedBlocks([]map[string]interface{}{block}, oversizeSplit)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Errorf("expected pass-through, got %d blocks", len(result))
	}
}

func TestParseOversizeMode(t *testing.T) {
	cases := map[string]oversizeMode{
		"":         oversizeSplit,
		"split":    oversizeSplit,
		"SPLIT":    oversizeSplit,
		"truncate": oversizeTruncate,
		"fail":     oversizeFail,
	}
	for in, want := range cases {
		got, err := parseOversizeMode(in)
		if err != nil {
			t.Errorf("%q: unexpected error: %v", in, err)
		}
		if got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
	if _, err := parseOversizeMode("bogus"); err == nil {
		t.Error("bogus input should error")
	}
}

// ---- appendChildrenBatched integration test ----

type recordingAppender struct {
	calls []map[string]interface{}
	fail  int // 1-indexed batch to fail, 0 = never
}

func (r *recordingAppender) Patch(path string, body interface{}) ([]byte, error) {
	m, _ := body.(map[string]interface{})
	// deep-clone so the caller mutating the slice after our recording
	// doesn't affect what we captured.
	data, _ := json.Marshal(m)
	var copy map[string]interface{}
	json.Unmarshal(data, &copy)
	copy["__path"] = path
	r.calls = append(r.calls, copy)
	if r.fail != 0 && len(r.calls) == r.fail {
		return nil, &mockAPIError{"simulated failure"}
	}
	return []byte(`{"results":[]}`), nil
}

type mockAPIError struct{ msg string }

func (e *mockAPIError) Error() string { return e.msg }

func TestAppendChildrenBatched_SplitsAt100(t *testing.T) {
	children := make([]map[string]interface{}, 253)
	for i := range children {
		children[i] = map[string]interface{}{"i": i}
	}
	rec := &recordingAppender{}
	if _, err := appendChildrenBatched(rec, "parent", "", children); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.calls) != 3 {
		t.Fatalf("got %d PATCH calls, want 3", len(rec.calls))
	}
	got := []int{
		len(rec.calls[0]["children"].([]interface{})),
		len(rec.calls[1]["children"].([]interface{})),
		len(rec.calls[2]["children"].([]interface{})),
	}
	if got[0] != 100 || got[1] != 100 || got[2] != 53 {
		t.Errorf("chunk sizes = %v, want [100 100 53]", got)
	}
	// `after` should only be set on the first batch when provided.
	for _, c := range rec.calls {
		if _, ok := c["after"]; ok {
			t.Errorf("no batch should have 'after' when caller passed empty afterID: %v", c)
		}
	}
}

func TestAppendChildrenBatched_AfterIDOnlyFirstBatch(t *testing.T) {
	children := make([]map[string]interface{}, 120)
	for i := range children {
		children[i] = map[string]interface{}{"i": i}
	}
	rec := &recordingAppender{}
	if _, err := appendChildrenBatched(rec, "parent", "anchor-xyz", children); err != nil {
		t.Fatal(err)
	}
	if len(rec.calls) != 2 {
		t.Fatalf("got %d calls, want 2", len(rec.calls))
	}
	if rec.calls[0]["after"] != "anchor-xyz" {
		t.Errorf("first batch should carry after=anchor-xyz, got %v", rec.calls[0]["after"])
	}
	if _, ok := rec.calls[1]["after"]; ok {
		t.Errorf("second batch must not carry 'after'")
	}
}

func TestAppendChildrenBatched_PartialFailureMessage(t *testing.T) {
	children := make([]map[string]interface{}, 250)
	for i := range children {
		children[i] = map[string]interface{}{"i": i}
	}
	rec := &recordingAppender{fail: 2}
	_, err := appendChildrenBatched(rec, "parent", "", children)
	if err == nil {
		t.Fatal("expected error from failing batch")
	}
	// Message should tell the user which batch failed and how many had been
	// written already.
	if !strings.Contains(err.Error(), "batch 2/3") {
		t.Errorf("expected 'batch 2/3' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "100 block") {
		t.Errorf("expected '100 block' in error, got: %v", err)
	}
}

func TestAppendChildrenBatched_UnderLimitSingleCall(t *testing.T) {
	children := make([]map[string]interface{}, 10)
	for i := range children {
		children[i] = map[string]interface{}{"i": i}
	}
	rec := &recordingAppender{}
	if _, err := appendChildrenBatched(rec, "parent", "", children); err != nil {
		t.Fatal(err)
	}
	if len(rec.calls) != 1 {
		t.Errorf("got %d calls, want 1", len(rec.calls))
	}
}

// End-to-end: parse a markdown document that blows both limits at once
// and verify the final block list is within both constraints.
func TestBigDocPipeline_AllLimitsRespected(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 150; i++ {
		sb.WriteString("- item ")
		sb.WriteString(strings.Repeat("x", 10))
		sb.WriteString("\n")
	}
	sb.WriteString("\n```go\n")
	sb.WriteString(strings.Repeat("y", 3000))
	sb.WriteString("\n```\n")
	blocks := parseMarkdownToBlocks(sb.String())
	// 150 bullets + 1 oversize code block (will split into 2) = 152 after
	// oversize handling, still >100 so batching must kick in.
	processed, err := handleOversizedBlocks(blocks, oversizeSplit)
	if err != nil {
		t.Fatal(err)
	}
	if len(processed) < 151 {
		t.Errorf("processed len = %d, want >=151", len(processed))
	}
	chunks := chunkChildren(processed)
	if len(chunks) < 2 {
		t.Errorf("expected >=2 chunks, got %d", len(chunks))
	}
	for i, chunk := range chunks {
		if len(chunk) > maxChildrenPerRequest {
			t.Errorf("chunk %d size %d > %d", i, len(chunk), maxChildrenPerRequest)
		}
		for _, b := range chunk {
			if b["type"] == "code" {
				rt := b["code"].(map[string]interface{})["rich_text"].([]map[string]interface{})
				content := rt[0]["text"].(map[string]interface{})["content"].(string)
				if len(content) > maxRichTextContentLen {
					t.Errorf("code content len %d > limit", len(content))
				}
			}
		}
	}
}
