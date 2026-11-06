package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The reply is exactly the gateway's token-adapter contract: {"headers": {...}}
// with the panel's own cookie and nothing the client could steer.
func TestCredentialsCarryOnlyThePanelCookie(t *testing.T) {
	out, err := credentials("magpie_web_3430", "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("credentials: %v", err)
	}
	var reply struct {
		Headers map[string][]string `json:"headers"`
	}
	if err := json.Unmarshal(out, &reply); err != nil {
		t.Fatalf("reply is not JSON: %v (%s)", err, out)
	}
	if len(reply.Headers) != 1 {
		t.Fatalf("reply has %d headers, want 1: %v", len(reply.Headers), reply.Headers)
	}
	got := reply.Headers["Cookie"]
	if len(got) != 1 {
		t.Fatalf("Cookie = %v, want exactly one value", got)
	}
	if want := "magpie_web_3430=0123456789abcdef0123456789abcdef"; got[0] != want {
		t.Fatalf("Cookie = %q, want %q", got[0], want)
	}
	if !strings.HasSuffix(string(out), "\n") {
		t.Fatal("reply must end in a newline")
	}
}

// A missing or short key fails instead of sending a cookie magpie rejects.
func TestCredentialsRefuseAnUnusableKey(t *testing.T) {
	for _, key := range []string{"", "short", strings.Repeat("a", 15)} {
		if _, err := credentials("magpie_web_3430", key); err == nil {
			t.Fatalf("credentials accepted key %q", key)
		}
	}
	if _, err := credentials("", "0123456789abcdef"); err == nil {
		t.Fatal("credentials accepted an empty cookie name")
	}
}
