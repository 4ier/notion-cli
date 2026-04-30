package cmd

import "testing"

func TestNormalizeAPIPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/v1/users/me", "/v1/users/me"},
		{"/v1", "/v1"},
		{"/users/me", "/v1/users/me"},
		{"users/me", "/v1/users/me"},
		{"/search", "/v1/search"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeAPIPath(tt.input)
			if got != tt.want {
				t.Errorf("normalizeAPIPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
