package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/config"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

func baseEnv() map[string]string {
	return map[string]string{
		"TELEGRAM_TOKEN": "tg-token",
		"FAMILY_MEMBERS": "111:Папа, 222:Мама",
		"LLM_API_KEY":    "key",
		"LLM_MODEL":      "anthropic/claude-haiku-5.5",
		"DATABASE_URL":   "postgres://x",
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := config.LoadEnv(baseEnv())
	require.NoError(t, err)
	assert.Equal(t, "https://openrouter.ai/api/v1", cfg.LLMBaseURL)
	assert.Equal(t, 30*time.Second, cfg.LLMTimeout)
	assert.True(t, cfg.STTEnabled)
	assert.Equal(t, "openai/whisper-large-v3", cfg.STTModel)
	assert.Equal(t, 60, cfg.STTMaxSeconds)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, "Europe/Moscow", cfg.Location.String())
	assert.False(t, cfg.SeedDemo)
	assert.Equal(t, "https://api.telegram.org", cfg.TelegramAPIURL)
	assert.Equal(t, []domain.Member{{TelegramID: 111, Name: "Папа"}, {TelegramID: 222, Name: "Мама"}}, cfg.Members)
}

func TestLoadMissingRequired(t *testing.T) {
	env := baseEnv()
	delete(env, "LLM_MODEL")
	_, err := config.LoadEnv(env)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLM_MODEL")
}

func TestLoadBadTimezone(t *testing.T) {
	env := baseEnv()
	for _, tz := range []string{"Mars/Olympus", "Local"} {
		env["APP_TIMEZONE"] = tz
		_, err := config.LoadEnv(env)
		assert.ErrorContains(t, err, "APP_TIMEZONE", tz)
	}
}

func TestParseMembersErrors(t *testing.T) {
	for _, in := range []string{"", "abc", "111", "111:", ":Папа", "0:Папа", "111:A,111:B"} {
		_, err := config.ParseMembers(in)
		assert.Error(t, err, "input %q", in)
	}
}

func TestLoadEmptyRequired(t *testing.T) {
	for _, key := range []string{"TELEGRAM_TOKEN", "FAMILY_MEMBERS", "LLM_API_KEY", "LLM_MODEL", "DATABASE_URL"} {
		env := baseEnv()
		env[key] = ""
		_, err := config.LoadEnv(env)
		require.Error(t, err, key)
		assert.Contains(t, err.Error(), key)
	}
}

func TestLoadTelegramAPIURL(t *testing.T) {
	env := baseEnv()
	env["TELEGRAM_API_URL"] = "http://localhost:8081"
	cfg, err := config.LoadEnv(env)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8081", cfg.TelegramAPIURL)

	for _, bad := range []string{"api.telegram.org", "ftp://x", "/relative", "http://", "://", "http://x?a=1", "http://x#f", "http://x?"} {
		env["TELEGRAM_API_URL"] = bad
		_, err := config.LoadEnv(env)
		assert.ErrorContains(t, err, "TELEGRAM_API_URL", "input %q", bad)
	}
}

func TestLoadTelegramAPIURLNormalised(t *testing.T) {
	for in, want := range map[string]string{
		"http://localhost:8081/":  "http://localhost:8081",
		"http://localhost:8081//": "http://localhost:8081",
		"":                        "https://api.telegram.org",
		"  ":                      "https://api.telegram.org",
	} {
		env := baseEnv()
		env["TELEGRAM_API_URL"] = in
		cfg, err := config.LoadEnv(env)
		require.NoError(t, err, "input %q", in)
		assert.Equal(t, want, cfg.TelegramAPIURL, "input %q", in)
	}
}
