package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Media block kinds supported by Notion. The string is both the
// user-facing flag prefix ("image", "file", "video", "audio", "pdf") and
// the Notion block type on the wire.
var mediaKinds = []string{"image", "file", "video", "audio", "pdf"}

// mediaClient is the minimal client surface mediaBuilder needs. Kept as an
// interface so media flag resolution is testable without a real HTTP client.
type mediaClient interface {
	fileUploadAPI
}

// mediaSource describes a user-chosen way to produce a media block.
// Exactly one source is active per CLI invocation.
type mediaSource struct {
	kind    string // "image" | "file" | "video" | "audio" | "pdf"
	mode    string // "external" | "file" | "upload"
	value   string // URL / local path / file_upload id
	caption string
}

// IsActive reports whether any media flag was set.
func (m *mediaSource) IsActive() bool {
	return m != nil && m.kind != ""
}

// Build performs any required upload step and returns the assembled block.
// For "external" sources there's no network I/O; for "file" sources we
// upload the local path and then reference the returned file_upload id.
func (m *mediaSource) Build(c mediaClient) (map[string]interface{}, error) {
	switch m.mode {
	case "external":
		return buildExternalMediaBlock(m.kind, m.value, m.caption), nil
	case "upload":
		return buildFileUploadMediaBlock(m.kind, m.value, m.caption), nil
	case "file":
		outcome, err := uploadFile(c, m.value, "")
		if err != nil {
			return nil, fmt.Errorf("upload %s: %w", m.value, err)
		}
		return buildFileUploadMediaBlock(m.kind, outcome.UploadID, m.caption), nil
	default:
		return nil, fmt.Errorf("internal: unknown media mode %q", m.mode)
	}
}

// registerMediaFlags adds all media source flags to a cobra command.
// The flag names follow a strict pattern:
//
//	--image-url     http(s) URL (external image)
//	--image-file    local path (upload + embed)
//	--image-upload  existing file_upload id (embed only)
//
// The same triple exists for file / video / audio / pdf, minus the -url
// variant for non-image types (image is the only one that previously
// shipped --image-url, and the Notion API accepts external URLs for every
// media type — we keep parity by also adding -url for the others, so
// e.g. `--video-url https://…/clip.mp4` works).
func registerMediaFlags(cmd *cobra.Command) {
	cmd.Flags().String("caption", "", "Caption for the media block (applies to any of the --*-url/--*-file/--*-upload flags)")
	for _, kind := range mediaKinds {
		cmd.Flags().String(kind+"-url", "", fmt.Sprintf("External %s URL (http/https)", kind))
		cmd.Flags().String(kind+"-file", "", fmt.Sprintf("Local %s file to upload and embed", kind))
		cmd.Flags().String(kind+"-upload", "", fmt.Sprintf("Existing file_upload ID to embed as %s", kind))
	}
}

// resolveMediaSource inspects every media flag on the command and returns
// the single active source, or nil if none were used. It returns an error
// when more than one source is set, or when a source collides with --file
// (markdown) or positional text.
func resolveMediaSource(cmd *cobra.Command, filePath, text string) (*mediaSource, error) {
	caption, _ := cmd.Flags().GetString("caption")

	var picked []string
	var active *mediaSource

	for _, kind := range mediaKinds {
		for _, mode := range []string{"url", "file", "upload"} {
			flag := kind + "-" + mode
			v, _ := cmd.Flags().GetString(flag)
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			picked = append(picked, "--"+flag)
			normalizedMode := mode
			if mode == "url" {
				normalizedMode = "external"
			}
			active = &mediaSource{
				kind:    kind,
				mode:    normalizedMode,
				value:   v,
				caption: caption,
			}
		}
	}

	if len(picked) == 0 {
		if caption != "" && filePath == "" && text == "" {
			return nil, fmt.Errorf("--caption requires one of --<media>-url/--<media>-file/--<media>-upload")
		}
		return nil, nil
	}
	if len(picked) > 1 {
		return nil, fmt.Errorf("at most one media source may be set, got: %s", strings.Join(picked, ", "))
	}
	if filePath != "" {
		return nil, fmt.Errorf("%s cannot be combined with --file", picked[0])
	}
	if text != "" {
		return nil, fmt.Errorf("%s cannot be combined with a positional text argument", picked[0])
	}

	if active.mode == "external" {
		if !strings.HasPrefix(active.value, "http://") && !strings.HasPrefix(active.value, "https://") {
			return nil, fmt.Errorf("%s must be an http:// or https:// URL", picked[0])
		}
	}
	return active, nil
}

// buildExternalMediaBlock produces a block whose media source is an
// external URL (no upload needed).
func buildExternalMediaBlock(kind, url, caption string) map[string]interface{} {
	media := map[string]interface{}{
		"type":     "external",
		"external": map[string]interface{}{"url": url},
	}
	if caption != "" {
		media["caption"] = captionRichText(caption)
	}
	return map[string]interface{}{
		"object": "block",
		"type":   kind,
		kind:     media,
	}
}

// buildFileUploadMediaBlock produces a block that references a previously
// created file_upload by id.
func buildFileUploadMediaBlock(kind, uploadID, caption string) map[string]interface{} {
	media := map[string]interface{}{
		"type":        "file_upload",
		"file_upload": map[string]interface{}{"id": uploadID},
	}
	if caption != "" {
		media["caption"] = captionRichText(caption)
	}
	return map[string]interface{}{
		"object": "block",
		"type":   kind,
		kind:     media,
	}
}

func captionRichText(caption string) []map[string]interface{} {
	return []map[string]interface{}{
		{"type": "text", "text": map[string]interface{}{"content": caption}},
	}
}
