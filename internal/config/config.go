package config

import (
	"fmt"
	"os"
	"strconv"

	"plusplus/internal/crypto"
)

type Config struct {
	Port string

	SlackSigningSecret string
	SlackBotToken      string
	SlackAppToken      string

	SlackClientID     string
	SlackClientSecret string
	PublicBaseURL     string

	// WorkspaceEncryptor is set when SLACK_CLIENT_ID is configured (OAuth install + encrypted tokens).
	WorkspaceEncryptor *crypto.AESEncryptor

	// AWSRegion is the DynamoDB region. Credentials come from the AWS SDK default
	// chain (AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY env vars on Railway).
	AWSRegion string
	// DynamoDBEndpoint overrides the DynamoDB endpoint (DynamoDB Local for dev).
	// Empty means use the real AWS endpoint for AWSRegion.
	DynamoDBEndpoint string

	KarmaTable      string
	SettingsTable   string
	WorkspacesTable string

	MaxKarmaPerAction int
}

func Load() (Config, error) {
	cfg := Config{
		Port:               getenvDefault("PORT", "8080"),
		SlackSigningSecret: os.Getenv("SLACK_SIGNING_SECRET"),
		SlackBotToken:      os.Getenv("SLACK_BOT_TOKEN"),
		SlackAppToken:      os.Getenv("SLACK_APP_TOKEN"),
		SlackClientID:      os.Getenv("SLACK_CLIENT_ID"),
		SlackClientSecret:  os.Getenv("SLACK_CLIENT_SECRET"),
		PublicBaseURL:      os.Getenv("PUBLIC_BASE_URL"),
		AWSRegion:          getenvDefault("AWS_REGION", "ap-southeast-2"),
		DynamoDBEndpoint:   os.Getenv("DYNAMODB_ENDPOINT"),
		KarmaTable:         getenvDefault("DYNAMODB_KARMA_TABLE", "plusplus_karma"),
		SettingsTable:      getenvDefault("DYNAMODB_SETTINGS_TABLE", "plusplus_channel_settings"),
		WorkspacesTable:    getenvDefault("DYNAMODB_WORKSPACES_TABLE", "plusplus_workspaces"),
		MaxKarmaPerAction:  getenvIntDefault("MAX_KARMA_PER_ACTION", 5),
	}

	if err := validatePort(cfg.Port); err != nil {
		return Config{}, err
	}
	if cfg.MaxKarmaPerAction < 1 {
		return Config{}, fmt.Errorf("MAX_KARMA_PER_ACTION must be greater than 0")
	}

	if cfg.SlackClientID != "" {
		if cfg.SlackClientSecret == "" {
			return Config{}, fmt.Errorf("SLACK_CLIENT_SECRET is required when SLACK_CLIENT_ID is set")
		}
		key, err := crypto.ParseKeyBase64(os.Getenv("TOKEN_ENCRYPTION_KEY"))
		if err != nil {
			return Config{}, fmt.Errorf("TOKEN_ENCRYPTION_KEY: %w", err)
		}
		enc, err := crypto.NewAESEncryptor(key)
		if err != nil {
			return Config{}, fmt.Errorf("workspace encryptor: %w", err)
		}
		cfg.WorkspaceEncryptor = enc
	}

	return cfg, nil
}

func validatePort(port string) error {
	value, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("PORT must be numeric: %w", err)
	}

	if value < 1 || value > 65535 {
		return fmt.Errorf("PORT must be in range 1-65535")
	}

	return nil
}

func getenvDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getenvIntDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}
