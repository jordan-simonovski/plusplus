package slack

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidSignature = errors.New("invalid slack signature")
	ErrStaleTimestamp   = errors.New("stale slack timestamp")
)

const maxSignatureAge = 5 * time.Minute

func VerifyRequestSignature(signingSecret, timestampHeader, signatureHeader string, body []byte) error {
	if signingSecret == "" || timestampHeader == "" || signatureHeader == "" {
		return ErrInvalidSignature
	}

	ts, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return fmt.Errorf("parse timestamp: %w", err)
	}

	skew := time.Now().Unix() - ts
	if skew < 0 {
		skew = -skew
	}
	if skew > int64(maxSignatureAge.Seconds()) {
		return ErrStaleTimestamp
	}

	expected := computeSignature(signingSecret, timestampHeader, body)
	if !hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signatureHeader))) {
		return ErrInvalidSignature
	}

	return nil
}

func computeSignature(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + timestamp + ":"))
	mac.Write(body)
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifiedBody is the gate in front of every Slack endpoint (AGENTS.md rule 1): it reads
// the raw body, checks Slack's signature over it, and only then calls next with the body.
// Unsigned, stale or tampered requests get a 401 and never reach next.
func VerifiedBody(secret string, next func(w http.ResponseWriter, r *http.Request, body []byte)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		if err := VerifyRequestSignature(secret, r.Header.Get("X-Slack-Request-Timestamp"), r.Header.Get("X-Slack-Signature"), body); err != nil {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		next(w, r, body)
	}
}
