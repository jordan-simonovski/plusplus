package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"plusplus/internal/domain"
)

// recordingSettings records what the settings commands asked for.
type recordingSettings struct {
	modes  []ReplyMode
	levels []int
	err    error
}

func (r *recordingSettings) SetReplyMode(_ context.Context, _, _, _ string, mode ReplyMode) (string, error) {
	r.modes = append(r.modes, mode)
	return "Reply mode set to " + string(mode) + ".", r.err
}

func (r *recordingSettings) SetSnarkLevel(_ context.Context, _, _, _ string, level int) (string, error) {
	r.levels = append(r.levels, level)
	if r.err != nil {
		return "", r.err
	}
	return fmt.Sprintf("Snark level set to %d.", level), nil
}

func (r *recordingSettings) GetSnarkLevel(context.Context, string, string) (int, error) {
	return domain.DefaultSnarkLevel, nil
}

// recordingReactions records emoji commands; err is returned from every call.
type recordingReactions struct {
	calls []string
	err   error
}

func (r *recordingReactions) Describe(context.Context, string, string) (string, error) {
	r.calls = append(r.calls, "describe")
	return "karma reactions", r.err
}

func (r *recordingReactions) SetTier(_ context.Context, _, _ string, workspace bool, _, tier string, emojis []string) (string, error) {
	r.calls = append(r.calls, fmt.Sprintf("set workspace=%v tier=%s %s", workspace, tier, strings.Join(emojis, " ")))
	return "tier set", r.err
}

func (r *recordingReactions) Reset(_ context.Context, _, _ string, workspace bool, _ string) (string, error) {
	r.calls = append(r.calls, fmt.Sprintf("reset workspace=%v", workspace))
	return "reset", r.err
}

func runSettings(t *testing.T, p *CommandsProcessor, text string) (int, string) {
	t.Helper()
	body := []byte("team_id=T1&channel_id=C1&user_id=U1&command=%2Fplusplus-settings&text=" + url.QueryEscape(text))
	req := httptest.NewRequest(http.MethodPost, "/slack/commands", bytes.NewReader(body))
	addSignedHeaders(req, "secret", body)
	rec := httptest.NewRecorder()
	serveCommand(p, rec, req)
	var resp MessageResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return rec.Code, resp.Text
}

func TestSettingsReplyMode(t *testing.T) {
	settings := &recordingSettings{}
	p := NewCommandsProcessor(fakeLeaderboardService{}, settings, "test")
	for _, mode := range []string{"thread", "channel"} {
		if code, text := runSettings(t, p, "reply_mode "+mode); code != http.StatusOK || !strings.Contains(text, mode) {
			t.Fatalf("reply_mode %s: %d %q", mode, code, text)
		}
	}
	if fmt.Sprint(settings.modes) != "[thread channel]" {
		t.Fatalf("modes set %v", settings.modes)
	}
	if _, text := runSettings(t, p, "reply_mode sideways"); !strings.Contains(text, "PlusPlus help") || len(settings.modes) != 2 {
		t.Fatalf("reply_mode sideways: %q, modes %v; want usage and no change", text, settings.modes)
	}
}

func TestSettingsSnarkText(t *testing.T) {
	settings := &recordingSettings{}
	p := NewCommandsProcessor(fakeLeaderboardService{}, settings, "test")
	if code, text := runSettings(t, p, "snark 7"); code != http.StatusOK || text != "Snark level set to 7." {
		t.Fatalf("snark 7: %d %q", code, text)
	}
	if _, text := runSettings(t, p, "snark abc"); !strings.HasPrefix(text, "Invalid level.") || len(settings.levels) != 1 {
		t.Fatalf("snark abc: %q, levels %v", text, settings.levels)
	}
	settings.err = errors.New("invalid snark level: 42 (allowed 1–10)")
	if code, text := runSettings(t, p, "snark 42"); code != http.StatusOK || text != settings.err.Error() {
		t.Fatalf("service validation error: %d %q; want it shown with 200", code, text)
	}
}

func TestSettingsEmoji(t *testing.T) {
	reactions := &recordingReactions{}
	p := NewCommandsProcessor(fakeLeaderboardService{}, &recordingSettings{}, "test").WithReactionSettings(reactions)
	for _, text := range []string{"emoji", "emoji reset", "emoji workspace reset", "emoji 3 :fire:"} {
		if code, _ := runSettings(t, p, text); code != http.StatusOK {
			t.Fatalf("%q: status %d", text, code)
		}
	}
	want := "describe|reset workspace=false|reset workspace=true|set workspace=false tier=3 :fire:"
	if got := strings.Join(reactions.calls, "|"); got != want {
		t.Fatalf("calls %q, want %q", got, want)
	}

	reactions.err = &InputError{Msg: "tier must be -5 to -1 or 1 to 5"}
	if _, text := runSettings(t, p, "emoji 9 :fire:"); !strings.HasPrefix(text, reactions.err.Error()) || !strings.Contains(text, "Example:") {
		t.Fatalf("input error: %q; want the message with the example", text)
	}
	reactions.err = errors.New("dynamo down")
	if _, text := runSettings(t, p, "emoji reset"); strings.Contains(text, "dynamo") || !strings.Contains(text, "try again") {
		t.Fatalf("other error: %q; want the generic retry message only", text)
	}
}

func TestSettingsEmojiWithoutReactionSettingsShowsUsage(t *testing.T) {
	p := NewCommandsProcessor(fakeLeaderboardService{}, &recordingSettings{}, "test")
	if _, text := runSettings(t, p, "emoji"); !strings.Contains(text, "PlusPlus help") {
		t.Fatalf("got %q, want usage", text)
	}
}
