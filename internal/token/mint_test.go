package token

import "testing"

func TestParseTokenFound(t *testing.T) {
	out := "Visit the URL...\nYour token:\nsk-ant-oat01-AbC_123-xyz\nDone.\n"
	if got := ParseToken(out); got != "sk-ant-oat01-AbC_123-xyz" {
		t.Fatalf("got %q", got)
	}
}

func TestParseTokenAbsent(t *testing.T) {
	if got := ParseToken("no token here"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
