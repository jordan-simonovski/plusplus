package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port string

	SlackSigningSecret string
	SlackBotToken      string
	SlackAppToken      string

	DatabaseURL       string
	MaxKarmaPerAction int
}

func Load() (Config, error) {
	cfg := Config{
		Port:             getenvDefault("PORT", "8080"),
		SlackSigningSecret: os.Getenv("SLACK_SIGNING_SECRET"),
		SlackBotToken:      os.Getenv("SLACK_BOT_TOKEN"),
		SlackAppToken:      os.Getenv("SLACK_APP_TOKEN"),
		DatabaseURL:        getenvDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/plusplus?sslmode=disable"),
		MaxKarmaPerAction:  getenvIntDefault("MAX_KARMA_PER_ACTION", 5),
	}

	if err := validatePort(cfg.Port); err != nil {
		return Config{}, err
	}
	if cfg.MaxKarmaPerAction < 1 {
		return Config{}, fmt.Errorf("MAX_KARMA_PER_ACTION must be greater than 0")
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
