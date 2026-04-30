package cmd

import (
	"fmt"
	"os"
	"strings"
)

// Notion API hard limits that the CLI absorbs transparently when possible.
// See: https://developers.notion.com/reference/request-limits
const (
	// maxChildrenPerRequest is the maximum `children.length` a single
	// PATCH /v1/blocks/<id>/children call will accept.
	maxChildrenPerRequest = 100

	// maxRichTextContentLen is the maximum length of a single rich_text
	// item's text.content (applies to code blocks too).
	maxRichTextContentLen = 2000
)

// oversizeMode controls behavior when a single block's text exceeds
// maxRichTextContentLen. Values come from --on-oversize.
type oversizeMode string

const (
	oversizeSplit    oversizeMode = "split"
	oversizeTruncate oversizeMode = "truncate"
	oversizeFail     oversizeMode = "fail"
)

// parseOversizeMode validates the flag value and returns the canonical enum.
func parseOversizeMode(raw string) (oversizeMode, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "", "split":
		return oversizeSplit, nil
	case "truncate":
		return oversizeTruncate, nil
	case "fail":
		return oversizeFail, nil
	default:
		return "", fmt.Errorf("--on-oversize must be one of: split, truncate, fail (got %q)", raw)
	}
}

// handleOversizedBlocks walks `blocks` and returns a new slice where every
// block is within Notion's per-item content limit. Today only code blocks
// (which are by far the most common offender) are split along newline
// boundaries; other rich_text blocks are split by character.
//
// In `fail` mode, the first oversize block returns an error with a clear
// message so the caller can print it verbatim.
func handleOversizedBlocks(blocks []map[string]interface{}, mode oversizeMode) ([]map[string]interface{}, error) {
	out := make([]map[string]interface{}, 0, len(blocks))
	for idx, b := range blocks {
		split, err := splitBlockIfOversize(b, idx, mode)
		if err != nil {
			return nil, err
		}
		out = append(out, split...)
	}
	return out, nil
}

func splitBlockIfOversize(block map[string]interface{}, idx int, mode oversizeMode) ([]map[string]interface{}, error) {
	blockType, _ := block["type"].(string)
	data, ok := block[blockType].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{block}, nil
	}
	content, segments := extractContentFromBlock(data)
	if segments != 1 || len(content) <= maxRichTextContentLen {
		return []map[string]interface{}{block}, nil
	}

	switch mode {
	case oversizeFail:
		return nil, fmt.Errorf(
			"block[%d] of type %q exceeds Notion's %d-char rich_text limit (%d chars); re-run with --on-oversize=split or truncate",
			idx, blockType, maxRichTextContentLen, len(content),
		)
	case oversizeTruncate:
		truncated := content[:maxRichTextContentLen]
		newBlock := cloneBlockWithContent(block, blockType, truncated)
		return []map[string]interface{}{newBlock}, nil
	}

	// default: split
	parts := splitContentForNotion(content, blockType == "code")
	result := make([]map[string]interface{}, 0, len(parts))
	for _, part := range parts {
		result = append(result, cloneBlockWithContent(block, blockType, part))
	}
	return result, nil
}

// extractContentFromBlock returns the first rich_text's text.content and
// the number of rich_text segments in the block. When the block has more
// than one segment (rich formatting), we conservatively skip splitting —
// the user likely crafted that carefully and it's rarely the offender.
func extractContentFromBlock(data map[string]interface{}) (string, int) {
	// rich_text can be []interface{} (from API response) or
	// []map[string]interface{} (from our parser). Handle both.
	if rt, ok := data["rich_text"].([]map[string]interface{}); ok {
		if len(rt) == 0 {
			return "", 0
		}
		textObj, _ := rt[0]["text"].(map[string]interface{})
		content, _ := textObj["content"].(string)
		return content, len(rt)
	}
	if rt, ok := data["rich_text"].([]interface{}); ok {
		if len(rt) == 0 {
			return "", 0
		}
		first, _ := rt[0].(map[string]interface{})
		textObj, _ := first["text"].(map[string]interface{})
		content, _ := textObj["content"].(string)
		return content, len(rt)
	}
	return "", 0
}

// splitContentForNotion slices content into ≤maxRichTextContentLen chunks.
// For code blocks we try to cut on newline boundaries so syntax highlighting
// still looks reasonable after the split.
func splitContentForNotion(content string, preferNewlineBoundary bool) []string {
	var parts []string
	remaining := content
	for len(remaining) > maxRichTextContentLen {
		cut := maxRichTextContentLen
		if preferNewlineBoundary {
			if nl := strings.LastIndex(remaining[:maxRichTextContentLen], "\n"); nl > maxRichTextContentLen/2 {
				// Only prefer the newline if it's not absurdly early
				// (avoids emitting tiny first chunks for one-huge-line inputs).
				cut = nl
			}
		}
		parts = append(parts, remaining[:cut])
		remaining = strings.TrimPrefix(remaining[cut:], "\n")
	}
	if remaining != "" {
		parts = append(parts, remaining)
	}
	if len(parts) == 0 {
		return []string{""}
	}
	return parts
}

// cloneBlockWithContent builds a new block with the same type and language
// (if a code block) but replaces its rich_text[0].text.content with `content`.
func cloneBlockWithContent(orig map[string]interface{}, blockType, content string) map[string]interface{} {
	newData := map[string]interface{}{
		"rich_text": []map[string]interface{}{
			{"text": map[string]interface{}{"content": content}},
		},
	}
	// Preserve language on code blocks.
	if origData, ok := orig[blockType].(map[string]interface{}); ok {
		if lang, ok := origData["language"].(string); ok {
			newData["language"] = lang
		}
		// Preserve to_do checked state
		if v, ok := origData["checked"]; ok {
			newData["checked"] = v
		}
	}
	return map[string]interface{}{
		"object":  "block",
		"type":    blockType,
		blockType: newData,
	}
}

// chunkChildren slices children into groups of at most maxChildrenPerRequest.
func chunkChildren(children []map[string]interface{}) [][]map[string]interface{} {
	if len(children) <= maxChildrenPerRequest {
		return [][]map[string]interface{}{children}
	}
	var out [][]map[string]interface{}
	for i := 0; i < len(children); i += maxChildrenPerRequest {
		end := i + maxChildrenPerRequest
		if end > len(children) {
			end = len(children)
		}
		out = append(out, children[i:end])
	}
	return out
}

// blockAppender is the minimal client surface appendChildrenBatched needs.
// Keeping it as an interface makes the batching logic testable without
// hitting the network.
type blockAppender interface {
	Patch(path string, body interface{}) ([]byte, error)
}

// appendChildrenBatched PATCHes children in groups of ≤100, preserving
// order. When `afterID` is non-empty it's used for the FIRST batch only
// (Notion only accepts `after` for the initial insertion; subsequent
// batches rely on sequential append to stay in order).
//
// Progress is printed to stderr when there is more than one batch, so
// stdout can still be piped to jq etc.
func appendChildrenBatched(c blockAppender, parentID, afterID string, children []map[string]interface{}) ([]byte, error) {
	batches := chunkChildren(children)
	var lastResp []byte
	var err error

	if len(batches) > 1 {
		fmt.Fprintf(os.Stderr, "note: appending %d blocks in %d batches of ≤%d...\n",
			len(children), len(batches), maxChildrenPerRequest)
	}

	for i, batch := range batches {
		reqBody := map[string]interface{}{
			"children": batch,
		}
		if i == 0 && afterID != "" {
			reqBody["after"] = afterID
		}
		lastResp, err = c.Patch(fmt.Sprintf("/v1/blocks/%s/children", parentID), reqBody)
		if err != nil {
			return nil, fmt.Errorf("batch %d/%d failed after writing %d block(s): %w",
				i+1, len(batches), i*maxChildrenPerRequest, err)
		}
		if len(batches) > 1 {
			fmt.Fprintf(os.Stderr, "  ✓ batch %d/%d (%d blocks)\n", i+1, len(batches), len(batch))
		}
	}
	return lastResp, nil
}
