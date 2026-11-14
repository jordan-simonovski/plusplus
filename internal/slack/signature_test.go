package slack

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func TestVerifyRequestSignatureRejectsFutureTimestamp(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"ok":true}`)
	timestamp := fmt.Sprintf("%d", time.Now().Add(10*time.Minute).Unix())
	signature := sign(secret, timestamp, body)

	err := VerifyRequestSignature(secret, timestamp, signature, body)
	if err == nil {
		t.Fatalf("expected stale timestamp error for future-dated request")
	}
}

func sign(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + timestamp + ":"))
	mac.Write(body)
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

// serveEvent and serveCommand send a request through the signature gate the server uses,
// so processor tests keep exercising verification.
func serveEvent(p *EventsProcessor, w http.ResponseWriter, r *http.Request) {
	VerifiedBody("secret", p.HandleEvent).ServeHTTP(w, r)
}

func serveCommand(p *CommandsProcessor, w http.ResponseWriter, r *http.Request) {
	VerifiedBody("secret", p.HandleCommand).ServeHTTP(w, r)
}

func TestVerifiedBody(t *testing.T) {
	body := []byte(`{"type":"event_callback"}`)
	signed := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(body))
		addSlackSignatureHeaders(r, "secret", body)
		return r
	}
	stale := signed()
	staleTS := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte("v0:" + staleTS + ":"))
	mac.Write(body)
	stale.Header.Set("X-Slack-Request-Timestamp", staleTS)
	stale.Header.Set("X-Slack-Signature", "v0="+hex.EncodeToString(mac.Sum(nil)))

	badSig := signed()
	badSig.Header.Set("X-Slack-Signature", "v0=deadbeef")
	noHeaders := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(body))
	get := signed()
	get.Method = http.MethodGet

	cases := map[string]struct {
		req      *http.Request
		wantCode int
		wantNext bool
	}{
		"valid":           {req: signed(), wantCode: http.StatusOK, wantNext: true},
		"missing headers": {req: noHeaders, wantCode: http.StatusUnauthorized},
		"bad signature":   {req: badSig, wantCode: http.StatusUnauthorized},
		"stale timestamp": {req: stale, wantCode: http.StatusUnauthorized},
		"wrong method":    {req: get, wantCode: http.StatusMethodNotAllowed},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var got []byte
			called := false
			h := VerifiedBody("secret", func(w http.ResponseWriter, _ *http.Request, b []byte) {
				called, got = true, b
				w.WriteHeader(http.StatusOK)
			})
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, tc.req)
			if rec.Code != tc.wantCode || called != tc.wantNext {
				t.Fatalf("status %d, next called %v; want %d, %v", rec.Code, called, tc.wantCode, tc.wantNext)
			}
			if called && !bytes.Equal(got, body) {
				t.Fatalf("next got body %q, want %q", got, body)
			}
		})
	}
}
