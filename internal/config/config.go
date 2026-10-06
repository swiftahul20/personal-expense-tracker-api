package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL     string
	JWTSecret       string
	JWTTTL          time.Duration
	RefreshTTL      time.Duration
	LLMProvider     string
	AnthropicAPIKey string
	GeminiAPIKey    string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is not set")
	}

	ttlHoursStr := os.Getenv("JWT_TTL_HOURS")
	ttlHours, err := strconv.Atoi(ttlHoursStr)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_TTL_HOURS: %w", err)
	}

	ttlDaysStr := os.Getenv("REFRESH_TTL_DAYS")
	ttlDays, err := strconv.Atoi(ttlDaysStr)
	if err != nil {
		return nil, fmt.Errorf("invalid REFRESH_TTL_DAYS: %w", err)
	}

	llmProvider := strings.TrimSpace(strings.ToLower(os.Getenv("LLM_PROVIDER")))
	if llmProvider == "" {
		llmProvider = "anthropic"
	}

	return &Config{
		DatabaseURL:     dbURL,
		JWTSecret:       jwtSecret,
		JWTTTL:          time.Duration(ttlHours) * time.Hour,
		RefreshTTL:      time.Duration(ttlDays) * 24 * time.Hour,
		LLMProvider:     llmProvider,
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		GeminiAPIKey:    os.Getenv("GEMINI_API_KEY"),
	}, nil
}
