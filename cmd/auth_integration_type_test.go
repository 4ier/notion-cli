package cmd

import "testing"

func TestDetectIntegrationType(t *testing.T) {
	cases := []struct {
		name string
		bot  map[string]interface{}
		want string
	}{
		{
			name: "internal (workspace-owned)",
			bot: map[string]interface{}{
				"owner": map[string]interface{}{"type": "workspace", "workspace": true},
			},
			want: "internal",
		},
		{
			name: "public (user-owned)",
			bot: map[string]interface{}{
				"owner": map[string]interface{}{"type": "user"},
			},
			want: "public",
		},
		{
			name: "missing owner",
			bot:  map[string]interface{}{},
			want: "",
		},
		{
			name: "unknown owner type",
			bot: map[string]interface{}{
				"owner": map[string]interface{}{"type": "planet"},
			},
			want: "",
		},
		{
			name: "non-map owner",
			bot: map[string]interface{}{
				"owner": "oops",
			},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detectIntegrationType(tc.bot)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
