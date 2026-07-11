package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/BIGGASSS/DouyuNotifier/internal/app"
	"github.com/BIGGASSS/DouyuNotifier/internal/config"
	"github.com/BIGGASSS/DouyuNotifier/internal/cookies"
	"github.com/BIGGASSS/DouyuNotifier/internal/douyu"
	"github.com/BIGGASSS/DouyuNotifier/internal/telegram"
)

func main() {
	cfg := config.Load()
	douyuClient := douyu.NewClient(cfg.DouyuAPIURL, &http.Client{Timeout: config.DefaultDouyuRequestTimeout})
	telegramClient := telegram.NewClient(cfg.TelegramBotToken, cfg.TelegramChatID, &http.Client{}, config.DefaultTelegramRequestGracePeriod, config.DefaultTelegramLongPollTimeout)
	application := app.New(app.Config{
		BotToken: cfg.TelegramBotToken, ChatID: cfg.TelegramChatID,
		PollInterval:            cfg.PollInterval,
		ValidationRetryDelay:    config.DefaultCookieValidationRetryDelay,
		ValidationRetries:       config.DefaultCookieValidationRetries,
		TelegramLongPollTimeout: config.DefaultTelegramLongPollTimeout,
	}, douyuClient, cookies.NewStore(cfg.CookiesFile, os.Stdout), telegramClient, os.Stdout)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := application.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Println("\n\nStopping...")
			return
		}
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
