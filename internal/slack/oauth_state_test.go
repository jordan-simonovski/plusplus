package slack

import (
	"testing"
)

func TestSignedOAuthStateRoundTrip(t *testing.T) {
	const secret = "test-signing-secret-for-oauth-state"
	s, err := newSignedOAuthState(secret)
	if err != nil {
		t.Fatal(err)
	}
	if !verifySignedOAuthState(secret, s) {
		t.Fatal("verify failed")
	}
	if verifySignedOAuthState("wrong-secret", s) {
		t.Fatal("wrong secret should fail")
	}
	if verifySignedOAuthState(secret, s+"x") {
		t.Fatal("tampered state should fail")
	}
}

func TestSignedOAuthStateEmptySecret(t *testing.T) {
	_, err := newSignedOAuthState("")
	if err == nil {
		t.Fatal("expected error")
	}
	if verifySignedOAuthState("", "anything") {
		t.Fatal("empty secret should not verify")
	}
}
