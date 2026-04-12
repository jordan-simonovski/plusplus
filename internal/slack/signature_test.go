package slack

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func TestVerifyRequestSignature(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"type":"url_verification"}`)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := sign(secret, timestamp, body)

	if err := VerifyRequestSignature(secret, timestamp, signature, body); err != nil {
		t.Fatalf("expected valid signature: %v", err)
	}
}

func TestVerifyRequestSignatureRejectsInvalidSignature(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"ok":true}`)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	err := VerifyRequestSignature(secret, timestamp, "v0=deadbeef", body)
	if err == nil {
		t.Fatalf("expected signature verification error")
	}
}

func TestVerifyRequestSignatureRejectsStaleTimestamp(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"ok":true}`)
	timestamp := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
	signature := sign(secret, timestamp, body)

	err := VerifyRequestSignature(secret, timestamp, signature, body)
	if err == nil {
		t.Fatalf("expected stale timestamp error")
	}
}

func sign(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + timestamp + ":"))
	mac.Write(body)
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}
