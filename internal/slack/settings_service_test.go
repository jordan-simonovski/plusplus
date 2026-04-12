package slack

import (
	"context"
	"testing"
)

func TestChannelSettingsServiceDefaultsUnknownModeToThread(t *testing.T) {
	store := &fakeSettingsStore{mode: "unknown"}
	service := NewChannelSettingsService(store)

	mode, err := service.GetReplyMode(context.Background(), "T1", "C1")
	if err != nil {
		t.Fatalf("get reply mode failed: %v", err)
	}
	if mode != ReplyModeThread {
		t.Fatalf("expected thread mode, got %s", mode)
	}
}

func TestChannelSettingsServiceSetReplyMode(t *testing.T) {
	store := &fakeSettingsStore{mode: "thread"}
	service := NewChannelSettingsService(store)

	message, err := service.SetReplyMode(context.Background(), "T1", "C1", "U1", ReplyModeChannel)
	if err != nil {
		t.Fatalf("set reply mode failed: %v", err)
	}
	if message == "" {
		t.Fatalf("expected confirmation message")
	}
	if store.savedMode != "channel" {
		t.Fatalf("expected persisted mode channel, got %s", store.savedMode)
	}
}

type fakeSettingsStore struct {
	mode      string
	savedMode string
}

func (f *fakeSettingsStore) GetReplyMode(_ context.Context, _ string, _ string) (string, error) {
	return f.mode, nil
}

func (f *fakeSettingsStore) SetReplyMode(_ context.Context, _ string, _ string, _ string, replyMode string) error {
	f.savedMode = replyMode
	return nil
}
