package cmd

import "testing"

func TestPageArchiveCommandAliases(t *testing.T) {
	// The canonical command is `archive`; `delete` and `trash` are aliases.
	wantAliases := map[string]bool{"delete": true, "trash": true}
	got := map[string]bool{}
	for _, a := range pageArchiveCmd.Aliases {
		got[a] = true
	}
	if len(got) != len(wantAliases) {
		t.Errorf("aliases = %v, want %v", pageArchiveCmd.Aliases, []string{"delete", "trash"})
	}
	for a := range wantAliases {
		if !got[a] {
			t.Errorf("missing alias %q", a)
		}
	}
}

func TestPageArchiveCommandNameIsCanonical(t *testing.T) {
	// `Use:` should start with "archive", making that the primary term in
	// --help output. The old "delete" lives only as an alias.
	if got := pageArchiveCmd.Name(); got != "archive" {
		t.Errorf("canonical name = %q, want 'archive'", got)
	}
}

func TestPageArchiveResolvesViaAlias(t *testing.T) {
	// Cobra resolves an alias by matching against command.Aliases.
	// Each alias must route to the same cobra.Command.
	for _, alias := range []string{"delete", "trash", "archive"} {
		cmd, _, err := pageCmd.Find([]string{alias})
		if err != nil {
			t.Errorf("alias %q: Find returned error: %v", alias, err)
			continue
		}
		if cmd != pageArchiveCmd {
			t.Errorf("alias %q resolved to %q, want the archive command", alias, cmd.Name())
		}
	}
}
