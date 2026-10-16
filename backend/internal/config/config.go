// Package config loads service configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // embed the tz database: distroless images ship none

	"github.com/caarlos0/env/v11"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

// Config is the full service configuration.
type Config struct {
	TelegramToken string `env:"TELEGRAM_TOKEN,required,notEmpty"`
	// TelegramAPIURL is the Bot API base URL (local Bot API server, proxy or test fake).
	TelegramAPIURL string        `env:"TELEGRAM_API_URL" envDefault:"https://api.telegram.org"`
	FamilyMembers  string        `env:"FAMILY_MEMBERS,required,notEmpty"`
	LLMAPIKey      string        `env:"LLM_API_KEY,required,notEmpty"`
	LLMBaseURL     string        `env:"LLM_BASE_URL" envDefault:"https://openrouter.ai/api/v1"`
	LLMModel       string        `env:"LLM_MODEL,required,notEmpty"`
	LLMTimeout     time.Duration `env:"LLM_TIMEOUT" envDefault:"30s"`
	STTEnabled     bool          `env:"STT_ENABLED" envDefault:"true"`
	STTModel       string        `env:"STT_MODEL" envDefault:"openai/whisper-large-v3"`
	STTMaxSeconds  int           `env:"STT_MAX_SECONDS" envDefault:"60"`
	Timezone       string        `env:"APP_TIMEZONE" envDefault:"Europe/Moscow"`
	DatabaseURL    string        `env:"DATABASE_URL,required,notEmpty"`
	HTTPAddr       string        `env:"HTTP_ADDR" envDefault:":8080"`
	LogLevel       string        `env:"LOG_LEVEL" envDefault:"info"`
	SeedDemo       bool          `env:"SEED_DEMO" envDefault:"false"`

	// Derived values, filled by Load.
	Members  []domain.Member
	Location *time.Location
}

// Load reads configuration from the process environment.
func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}
	return finalize(cfg)
}

// LoadEnv reads configuration from the given map instead of the process environment.
func LoadEnv(environ map[string]string) (Config, error) {
	cfg, err := env.ParseAsWithOptions[Config](env.Options{Environment: environ})
	if err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}
	return finalize(cfg)
}

func finalize(cfg Config) (Config, error) {
	members, err := ParseMembers(cfg.FamilyMembers)
	if err != nil {
		return Config{}, fmt.Errorf("FAMILY_MEMBERS: %w", err)
	}
	cfg.Members = members

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return Config{}, fmt.Errorf("APP_TIMEZONE: %w", err)
	}
	cfg.Location = loc

	if u, err := url.Parse(cfg.TelegramAPIURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Config{}, errors.New("TELEGRAM_API_URL must be an absolute http(s) URL")
	}

	if cfg.STTMaxSeconds <= 0 {
		return Config{}, errors.New("STT_MAX_SECONDS must be positive")
	}
	return cfg, nil
}

// ParseMembers parses "111:Папа, 222:Мама".
func ParseMembers(s string) ([]domain.Member, error) {
	var out []domain.Member
	seen := map[int64]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idStr, name, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("%q: ожидается формат ID:Имя", part)
		}
		id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("%q: некорректный Telegram ID", part)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("%q: пустое имя", part)
		}
		if seen[id] {
			return nil, fmt.Errorf("повторяется Telegram ID %d", id)
		}
		seen[id] = true
		out = append(out, domain.Member{TelegramID: id, Name: name})
	}
	if len(out) == 0 {
		return nil, errors.New("список членов семьи пуст")
	}
	return out, nil
}
