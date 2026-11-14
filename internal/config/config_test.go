package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

var configVars = []string{
	"PORT", "SLACK_SIGNING_SECRET", "SLACK_BOT_TOKEN", "SLACK_CLIENT_ID", "SLACK_CLIENT_SECRET",
	"PUBLIC_BASE_URL", "SLACK_OAUTH_SCOPES", "TOKEN_ENCRYPTION_KEY", "AWS_REGION", "DYNAMODB_ENDPOINT",
	"DYNAMODB_KARMA_TABLE", "DYNAMODB_SETTINGS_TABLE", "DYNAMODB_WORKSPACES_TABLE", "MAX_KARMA_PER_ACTION",
}

var validKey = base64.StdEncoding.EncodeToString(make([]byte, 32))

// setEnv clears every variable Load reads, then applies env, so the ambient environment
// cannot leak into a case.
func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, k := range configVars {
		t.Setenv(k, env[k])
	}
}

func TestLoadAndValidateServer(t *testing.T) {
	single := map[string]string{"SLACK_SIGNING_SECRET": "s", "SLACK_BOT_TOKEN": "xoxb-1"}
	oauth := map[string]string{
		"SLACK_SIGNING_SECRET": "s", "SLACK_CLIENT_ID": "id", "SLACK_CLIENT_SECRET": "secret",
		"PUBLIC_BASE_URL": "https://example.com", "TOKEN_ENCRYPTION_KEY": validKey,
	}
	with := func(base map[string]string, k, v string) map[string]string {
		out := map[string]string{k: v}
		for bk, bv := range base {
			if bk != k {
				out[bk] = bv
			}
		}
		return out
	}

	cases := []struct {
		name        string
		env         map[string]string
		loadErr     string // substring; "" means Load succeeds
		validateErr string // substring; "" means ValidateServer succeeds
	}{
		{name: "single workspace", env: single},
		{name: "oauth", env: oauth},
		{name: "no slack settings loads (migrate, integration tests)", env: map[string]string{}, validateErr: "SLACK_SIGNING_SECRET"},
		{name: "missing signing secret", env: with(single, "SLACK_SIGNING_SECRET", ""), validateErr: "SLACK_SIGNING_SECRET"},
		{name: "neither bot token nor oauth", env: map[string]string{"SLACK_SIGNING_SECRET": "s"}, validateErr: "SLACK_BOT_TOKEN"},
		{name: "non-numeric max karma", env: with(single, "MAX_KARMA_PER_ACTION", "abc"), loadErr: "MAX_KARMA_PER_ACTION"},
		{name: "zero max karma", env: with(single, "MAX_KARMA_PER_ACTION", "0"), loadErr: "MAX_KARMA_PER_ACTION"},
		{name: "non-numeric port", env: with(single, "PORT", "http"), loadErr: "PORT"},
		{name: "port out of range", env: with(single, "PORT", "70000"), loadErr: "PORT"},
		{name: "oauth without client secret", env: with(oauth, "SLACK_CLIENT_SECRET", ""), loadErr: "SLACK_CLIENT_SECRET"},
		{name: "oauth without public base url", env: with(oauth, "PUBLIC_BASE_URL", ""), loadErr: "PUBLIC_BASE_URL"},
		{name: "oauth without encryption key", env: with(oauth, "TOKEN_ENCRYPTION_KEY", ""), loadErr: "TOKEN_ENCRYPTION_KEY"},
		{name: "oauth with a short key", env: with(oauth, "TOKEN_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 16))), loadErr: "TOKEN_ENCRYPTION_KEY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setEnv(t, tc.env)
			cfg, err := Load()
			if !matches(err, tc.loadErr) {
				t.Fatalf("Load error = %v, want %q", err, tc.loadErr)
			}
			if err != nil {
				return
			}
			if err := cfg.ValidateServer(); !matches(err, tc.validateErr) {
				t.Fatalf("ValidateServer error = %v, want %q", err, tc.validateErr)
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	setEnv(t, map[string]string{"SLACK_SIGNING_SECRET": "s", "SLACK_BOT_TOKEN": "xoxb-1"})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8080" || cfg.AWSRegion != "ap-southeast-2" || cfg.MaxKarmaPerAction != 5 ||
		cfg.KarmaTable != "plusplus_karma" || cfg.SettingsTable != "plusplus_channel_settings" ||
		cfg.WorkspacesTable != "plusplus_workspaces" || cfg.WorkspaceEncryptor != nil {
		t.Fatalf("defaults = %+v", cfg)
	}

	setEnv(t, map[string]string{"SLACK_SIGNING_SECRET": "s", "SLACK_BOT_TOKEN": "xoxb-1", "MAX_KARMA_PER_ACTION": "3"})
	if cfg, err := Load(); err != nil || cfg.MaxKarmaPerAction != 3 {
		t.Fatalf("MAX_KARMA_PER_ACTION=3: got %d, %v", cfg.MaxKarmaPerAction, err)
	}
}

func matches(err error, want string) bool {
	if want == "" {
		return err == nil
	}
	return err != nil && strings.Contains(err.Error(), want)
}
