package client

import (
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := New("test-token")
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.token != "test-token" {
		t.Error("token not set correctly")
	}
	if c.debug != false {
		t.Error("debug should default to false")
	}
}

func TestSetDebug(t *testing.T) {
	c := New("test-token")
	c.SetDebug(true)
	if !c.debug {
		t.Error("SetDebug(true) should set debug to true")
	}
	c.SetDebug(false)
	if c.debug {
		t.Error("SetDebug(false) should set debug to false")
	}
}

func TestTokenNotInURL(t *testing.T) {
	// Verify that the base URL doesn't contain any token references
	if strings.Contains(BaseURL, "token") {
		t.Error("BaseURL should not contain 'token'")
	}
}

func TestConstants(t *testing.T) {
	if BaseURL != "https://api.notion.com" {
		t.Errorf("BaseURL = %q, want https://api.notion.com", BaseURL)
	}
	if NotionVersion != "2022-06-28" {
		t.Errorf("NotionVersion = %q, want 2022-06-28", NotionVersion)
	}
}
