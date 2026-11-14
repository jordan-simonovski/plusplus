package slack

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

// A 5xx must leave a log line an operator can act on: which workspace, which event, why.
// The message text is user content and must never be logged.
func TestKarmaFailureIsLoggedWithoutMessageText(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	var calls []string
	p := NewEventsProcessor("secret", countingKarma{calls: &calls, fail: map[string]bool{"U3": true}}, nil, nil, &fakeWebClient{})
	if got := deliver(context.Background(), p, "Ev1", "<@U3> ++ for the secret launch plan"); got != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", got)
	}

	out := buf.String()
	for _, want := range []string{`"level":"ERROR"`, `"team":"T1"`, `"kind":"app_mention"`, `"err":"dynamo down"`} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q is missing %s", out, want)
		}
	}
	if strings.Contains(out, "secret launch plan") {
		t.Errorf("log contains the message text: %q", out)
	}
}
