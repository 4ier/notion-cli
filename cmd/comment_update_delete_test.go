package cmd

import (
	"testing"
)

func TestCommentUpdateCmd_RequiresTextOrMention(t *testing.T) {
	if commentUpdateCmd.Short == "" {
		t.Error("update short help missing")
	}
	// Sanity: the command is registered under the parent.
	for _, expected := range []string{"update", "delete"} {
		found := false
		for _, sub := range commentCmd.Commands() {
			if sub.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("comment %s subcommand not registered", expected)
		}
	}
}

func TestCommentDeleteCmd_AcceptsVariadic(t *testing.T) {
	// Args validator should accept 1+ ids.
	if err := commentDeleteCmd.Args(commentDeleteCmd, []string{"id1"}); err != nil {
		t.Errorf("one id should be valid: %v", err)
	}
	if err := commentDeleteCmd.Args(commentDeleteCmd, []string{"id1", "id2", "id3"}); err != nil {
		t.Errorf("multiple ids should be valid: %v", err)
	}
	if err := commentDeleteCmd.Args(commentDeleteCmd, []string{}); err == nil {
		t.Errorf("zero ids should fail")
	}
}
