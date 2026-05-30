package slack

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"testing"
	"time"
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

func TestSignedOAuthStateExpires(t *testing.T) {
	const secret = "test-signing-secret-for-oauth-state"
	// Mint a state with a timestamp older than the max age, MAC'd correctly.
	stale := mintOAuthStateAt(secret, time.Now().Add(-oauthStateMaxAge-time.Minute))
	if verifySignedOAuthState(secret, stale) {
		t.Fatal("expired state should not verify")
	}

	fresh := mintOAuthStateAt(secret, time.Now())
	if !verifySignedOAuthState(secret, fresh) {
		t.Fatal("fresh state should verify")
	}
}

// mintOAuthStateAt builds a valid signed state with a chosen timestamp, mirroring
// newSignedOAuthState, so expiry can be exercised without waiting.
func mintOAuthStateAt(secret string, at time.Time) string {
	payload := make([]byte, oauthStateNonceLen+oauthStateTimestampLen)
	binary.BigEndian.PutUint64(payload[oauthStateNonceLen:], uint64(at.Unix()))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(payload))
}
