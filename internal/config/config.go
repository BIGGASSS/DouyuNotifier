package config

import (
	"os"
	"time"
)

const (
	DefaultPollInterval               = 180 * time.Second
	DefaultCookieValidationRetryDelay = 10 * time.Second
	DefaultCookieValidationRetries    = 3
	DefaultTelegramLongPollTimeout    = 60
	DefaultDouyuRequestTimeout        = 30 * time.Second
	DefaultTelegramRequestGracePeriod = 10 * time.Second
	DefaultDouyuAPIURL                = "https://www.douyu.com/wgapi/livenc/liveweb/follow/list"
	DefaultCookiesFile                = "cookies.json"
)

// Config contains runtime configuration. Secrets are loaded from the environment.
type Config struct {
	TelegramBotToken string
	TelegramChatID   string
	DouyuAPIURL      string
	CookiesFile      string
	PollInterval     time.Duration
}

func Load() Config {
	return Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		DouyuAPIURL:      DefaultDouyuAPIURL,
		CookiesFile:      DefaultCookiesFile,
		PollInterval:     DefaultPollInterval,
	}
}
