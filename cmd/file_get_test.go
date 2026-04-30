package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRenderFileUpload_AllFields(t *testing.T) {
	// Redirect stdout so we can assert on the rendered lines.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	renderFileUpload(map[string]interface{}{
		"id":             "upload-abc",
		"filename":       "chart.png",
		"status":         "uploaded",
		"content_type":   "image/png",
		"content_length": float64(45231),
		"created_time":   "2026-04-30T05:00:00.000Z",
		"expiry_time":    "2026-05-30T05:00:00.000Z",
		"file": map[string]interface{}{
			"url": "https://notion.s3/chart.png?sig=x",
		},
	})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	out := buf.String()

	for _, want := range []string{
		// Title goes through fatih/color which bypasses our stdout pipe,
		// so we only assert on fields whose VALUES come from fmt.Println.
		"upload-abc",
		"uploaded",
		"image/png",
		"45231 bytes",
		"2026-04-30",
		"2026-05-30",
		"https://notion.s3/chart.png",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRenderFileUpload_MinimalPending(t *testing.T) {
	// Only the bare minimum — a pending upload that hasn't received bytes yet.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	renderFileUpload(map[string]interface{}{
		"id":     "upload-pending",
		"status": "pending",
	})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "upload-pending") || !strings.Contains(out, "pending") {
		t.Errorf("minimal render missing fields:\n%s", out)
	}
	// Should NOT print empty Size / Created / Expires / Content-Type values.
	// We check for the *value pattern* rather than the key (which goes through color lib).
	if strings.Contains(out, "bytes") {
		t.Errorf("unexpected size line for pending object:\n%s", out)
	}
}
