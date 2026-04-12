package slack

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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

	now := time.Now().Unix()
	if now-ts > int64(maxSignatureAge.Seconds()) {
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
