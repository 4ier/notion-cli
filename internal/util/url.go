package util

import (
	"regexp"
	"strings"
)

var (
	// Match Notion URLs like https://www.notion.so/page-title-abc123def456
	// or https://www.notion.so/workspace/abc123def456?v=...
	notionURLRe = regexp.MustCompile(`(?:notion\.so|notion\.site)/(?:.*?)([a-f0-9]{32}|[a-f0-9-]{36})`)
	uuidRe      = regexp.MustCompile(`^[a-f0-9]{8}-?[a-f0-9]{4}-?[a-f0-9]{4}-?[a-f0-9]{4}-?[a-f0-9]{12}$`)
	plainIDRe   = regexp.MustCompile(`^[a-f0-9]{32}$`)
)

// ResolveID extracts a Notion object ID from a URL or raw ID string.
// Accepts: full URLs, UUIDs with/without dashes, 32-char hex IDs.
func ResolveID(input string) string {
	input = strings.TrimSpace(input)

	// Full URL
	if strings.Contains(input, "notion.so") || strings.Contains(input, "notion.site") {
		matches := notionURLRe.FindStringSubmatch(input)
		if len(matches) > 1 {
			return formatUUID(matches[1])
		}
	}

	// Already a UUID with dashes
	if uuidRe.MatchString(input) {
		return input
	}

	// 32-char hex (no dashes)
	if plainIDRe.MatchString(input) {
		return formatUUID(input)
	}

	// Return as-is (let the API handle the error)
	return input
}

// formatUUID inserts dashes into a 32-char hex string to make a standard UUID.
func formatUUID(id string) string {
	id = strings.ReplaceAll(id, "-", "")
	if len(id) != 32 {
		return id
	}
	return id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
}
