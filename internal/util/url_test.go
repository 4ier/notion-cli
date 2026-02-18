package util

import "testing"

func TestResolveID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain UUID with dashes",
			input: "c9e9f681-ec8e-4eb7-be25-bbbe479b05b0",
			want:  "c9e9f681-ec8e-4eb7-be25-bbbe479b05b0",
		},
		{
			name:  "UUID without dashes (32 hex chars)",
			input: "c9e9f681ec8e4eb7be25bbbe479b05b0",
			want:  "c9e9f681ec8e4eb7be25bbbe479b05b0",
		},
		{
			name:  "notion.so URL with page name",
			input: "https://www.notion.so/My-Page-c9e9f681ec8e4eb7be25bbbe479b05b0",
			want:  "c9e9f681-ec8e-4eb7-be25-bbbe479b05b0",
		},
		{
			name:  "notion.so URL without page name",
			input: "https://www.notion.so/c9e9f681ec8e4eb7be25bbbe479b05b0",
			want:  "c9e9f681-ec8e-4eb7-be25-bbbe479b05b0",
		},
		{
			name:  "notion.so URL with query params",
			input: "https://www.notion.so/My-Page-c9e9f681ec8e4eb7be25bbbe479b05b0?v=abc123",
			want:  "c9e9f681-ec8e-4eb7-be25-bbbe479b05b0",
		},
		{
			name:  "short string passthrough",
			input: "abc123",
			want:  "abc123",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveID(tt.input)
			if got != tt.want {
				t.Errorf("ResolveID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
