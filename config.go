package main

import (
	"os"
	"time"
)

const (
	pollInterval               = 180 * time.Second
	cookieValidationRetryDelay = 10 * time.Second
	cookieValidationRetries    = 3
	telegramLongPollTimeout    = 60
	douyuRequestTimeout        = 30 * time.Second
	telegramRequestGracePeriod = 10 * time.Second
)

var (
	douyuAPIURL      = "https://www.douyu.com/wgapi/livenc/liveweb/follow/list"
	telegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	telegramChatID   = os.Getenv("TELEGRAM_CHAT_ID")
)
