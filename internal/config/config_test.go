package config

import (
	"path/filepath"
	"testing"
)

func TestLoadUsesEnvironmentAndRelativeCookieFile(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "TOKEN")
	t.Setenv("TELEGRAM_CHAT_ID", "123")

	cfg := Load()
	if cfg.TelegramBotToken != "TOKEN" || cfg.TelegramChatID != "123" {
		t.Fatalf("config=%#v", cfg)
	}
	if cfg.CookiesFile != DefaultCookiesFile || filepath.IsAbs(cfg.CookiesFile) {
		t.Fatalf("cookies file=%q, want relative %q", cfg.CookiesFile, DefaultCookiesFile)
	}
	if cfg.DouyuAPIURL != DefaultDouyuAPIURL || cfg.PollInterval != DefaultPollInterval {
		t.Fatalf("config=%#v", cfg)
	}
}
