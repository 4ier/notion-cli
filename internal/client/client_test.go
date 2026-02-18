package client

import (
	"fmt"
	"strings"
	"testing"
)

func TestErrorHint(t *testing.T) {
	tests := []struct {
		code    string
		message string
		wantHas string // substring that should be in the hint
	}{
		{"object_not_found", "Could not find page", "shared with your integration"},
		{"unauthorized", "API token is invalid", "notion auth login"},
		{"restricted_resource", "Not allowed", "Share the page"},
		{"rate_limited", "Rate limited", "Wait"},
		{"validation_error", "is not a property that exists", "notion db view"},
		{"validation_error", "body failed validation", "--debug"},
		{"conflict_error", "conflict", "Retry"},
		{"internal_server_error", "error", "Notion's servers"},
		{"service_unavailable", "unavailable", "Try again"},
		{"unknown_code", "unknown", ""},
	}

	for i, tt := range tests {
		name := fmt.Sprintf("%d_%s", i, tt.code)
		t.Run(name, func(t *testing.T) {
			hint := errorHint(tt.code, tt.message)
			if tt.wantHas == "" {
				if hint != "" {
					t.Errorf("expected empty hint, got %q", hint)
				}
				return
			}
			if !strings.Contains(hint, tt.wantHas) {
				t.Errorf("hint = %q, want substring %q", hint, tt.wantHas)
			}
		})
	}
}
