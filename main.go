package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	fetchDouyuLiveStatusFunc = fetchDouyuLiveStatus
	processPingCommandsFunc  = processPingCommands
)

// validateCookies calls Douyu and retries temporary API failures.
func validateCookies(ctx context.Context, cookies map[string]string) ([]Room, error) {
	var lastError *DouyuAPIError
	for attempt := 1; attempt <= cookieValidationRetries; attempt++ {
		rooms, err := fetchDouyuLiveStatusFunc(ctx, cookies)
		if err == nil {
			return rooms, nil
		}

		var notLoggedIn *NotLoginError
		if errors.As(err, &notLoggedIn) {
			return nil, err
		}

		var apiError *DouyuAPIError
		if !errors.As(err, &apiError) {
			return nil, err
		}
		lastError = apiError
		if attempt == cookieValidationRetries {
			break
		}

		fmt.Printf(
			"Cookie validation hit a temporary API error (%d/%d): %v\n",
			attempt,
			cookieValidationRetries,
			apiError,
		)
		if err := sleepContextFunc(ctx, cookieValidationRetryDelay); err != nil {
			return nil, err
		}
	}

	if lastError != nil {
		return nil, lastError
	}
	return nil, &DouyuAPIError{Message: "Cookie validation failed."}
}

// recoverCookiesViaTelegram requests replacement cookies until Douyu accepts one.
func recoverCookiesViaTelegram(ctx context.Context, reason string) (map[string]string, []Room, error) {
	fmt.Printf("Authentication requires a new cookie: %s\n", reason)
	nextOffset, err := getNextUpdateOffset(ctx)
	if err != nil {
		var conflict *TelegramPollingConflict
		if errors.As(err, &conflict) {
			fmt.Printf("Telegram polling conflict: %v\n", conflict)
			sendTelegramFunc(
				ctx,
				"I could send notifications, but I cannot receive your reply because "+
					"this bot token has a Telegram polling conflict.\nReason: "+
					"<code>"+htmlEscape(conflict.Error())+"</code>",
			)
		}
		return nil, nil, err
	}

	promptSent := sendTelegramFunc(
		ctx,
		"Douyu cookie expired or is invalid.\n"+
			"Reason: <code>"+htmlEscape(reason)+"</code>\n"+
			"Reply in this chat with a fresh full cookie string in the format:\n"+
			"<code>name=value; name2=value2</code>",
	)
	if promptSent {
		fmt.Println("Sent Telegram prompt for a new cookie.")
	} else {
		fmt.Println("Failed to send Telegram prompt; still listening for chat replies.")
	}

	for {
		fmt.Println("Waiting for a new cookie in Telegram...")
		cookieMessage, newOffset, err := waitForChatMessage(ctx, nextOffset)
		nextOffset = newOffset
		if err != nil {
			var conflict *TelegramPollingConflict
			if errors.As(err, &conflict) {
				fmt.Printf("Telegram polling conflict: %v\n", conflict)
				sendTelegramFunc(
					ctx,
					"I cannot receive your cookie reply because this bot token has a "+
						"Telegram polling conflict.\nReason: <code>"+
						htmlEscape(conflict.Error())+"</code>",
				)
			}
			return nil, nil, err
		}
		if cookieMessage == "" {
			continue
		}

		candidateCookies := parseCookieString(cookieMessage)
		if len(candidateCookies) == 0 {
			fmt.Println("Received a Telegram message that could not be parsed as cookies.")
			sendTelegramFunc(
				ctx,
				"I could not parse that message as a cookie string.\n"+
					"Send the full value in this format:\n"+
					"<code>name=value; name2=value2</code>",
			)
			continue
		}

		fmt.Printf("Received %d cookies from Telegram; validating...\n", len(candidateCookies))
		rooms, err := validateCookies(ctx, candidateCookies)
		if err != nil {
			var notLoggedIn *NotLoginError
			if errors.As(err, &notLoggedIn) {
				fmt.Println("Telegram provided cookie was rejected by Douyu.")
				sendTelegramFunc(
					ctx,
					"That cookie did not work. Make sure it comes from a logged-in "+
						"Douyu browser session, then send a fresh full cookie string.",
				)
				continue
			}

			var apiError *DouyuAPIError
			if errors.As(err, &apiError) {
				fmt.Printf("Could not validate Telegram cookie due to API error: %v\n", apiError)
				sendTelegramFunc(
					ctx,
					"I received your cookie, but Douyu could not be reached to "+
						"verify it yet: <code>"+htmlEscape(apiError.Error())+"</code>\n"+
						"Please send the cookie again after the service recovers.",
				)
				continue
			}
			return nil, nil, err
		}

		saveCookies(candidateCookies)
		sendTelegramFunc(ctx, "New Douyu cookie verified successfully. Monitoring resumed.")
		fmt.Println("Telegram cookie accepted and saved.")
		return candidateCookies, rooms, nil
	}
}

// waitWithPingChecks waits for the next poll while servicing Telegram /ping.
func waitWithPingChecks(ctx context.Context, delay time.Duration, pingOffset int64) (int64, error) {
	deadline := nowFunc().Add(delay)
	currentOffset := pingOffset

	for {
		if err := ctx.Err(); err != nil {
			return currentOffset, err
		}
		remaining := deadline.Sub(nowFunc())
		if remaining <= 0 {
			return currentOffset, nil
		}

		timeout := int(remaining / time.Second)
		if timeout < 1 {
			timeout = 1
		}
		if timeout > telegramLongPollTimeout {
			timeout = telegramLongPollTimeout
		}

		nextOffset, err := processPingCommandsFunc(ctx, currentOffset, timeout)
		if err != nil {
			return currentOffset, err
		}
		currentOffset = nextOffset
	}
}

// processRoomNotifications evaluates live and offline transitions against the
// same previous snapshot, then returns the current snapshot.
func processRoomNotifications(
	ctx context.Context,
	rooms []Room,
	previousLive map[string]struct{},
) map[string]struct{} {
	notifyNewLive(ctx, rooms, previousLive)
	notifyStreamEnd(ctx, rooms, previousLive)
	return currentLiveRoomIDs(rooms)
}

func run(ctx context.Context) error {
	fmt.Println("Douyu Live Status Notifier")
	fmt.Println("==========================")

	if telegramBotToken == "" || telegramChatID == "" {
		return errors.New("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID env vars required.")
	}

	cookies := loadCookies()
	var initialRooms []Room
	if len(cookies) == 0 {
		fmt.Println("No local cookies available. Requesting one through Telegram.")
		var err error
		cookies, initialRooms, err = recoverCookiesViaTelegram(ctx, "No local cookie found.")
		if err != nil {
			return err
		}
	} else {
		rooms, err := validateCookies(ctx, cookies)
		if err == nil {
			initialRooms = rooms
		} else {
			var notLoggedIn *NotLoginError
			var apiError *DouyuAPIError
			switch {
			case errors.As(err, &notLoggedIn):
				cookies, initialRooms, err = recoverCookiesViaTelegram(ctx, notLoggedIn.Error())
				if err != nil {
					return err
				}
			case errors.As(err, &apiError):
				fmt.Printf("Initial validation failed: %v\n", apiError)
				fmt.Println("Starting monitor anyway and retrying in the polling loop.")
				initialRooms = []Room{}
			default:
				return err
			}
		}
	}

	fmt.Printf("Found %d cookies\n", len(cookies))
	fmt.Printf("Polling every %d seconds\n", int(pollInterval/time.Second))
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	previousLive := processRoomNotifications(ctx, initialRooms, nil)
	var pingOffset int64

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		rooms, err := fetchDouyuLiveStatusFunc(ctx, cookies)
		if err == nil {
			liveCount := countLiveRooms(rooms)
			fmt.Printf(
				"[%s] Checked: %d/%d live\n",
				nowFunc().Format("15:04:05"),
				liveCount,
				len(rooms),
			)
			updateHealthState(liveCount)
			previousLive = processRoomNotifications(ctx, rooms, previousLive)
			pingOffset, err = waitWithPingChecks(ctx, pollInterval, pingOffset)
			if err == nil {
				continue
			}
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		var notLoggedIn *NotLoginError
		if errors.As(err, &notLoggedIn) {
			fmt.Printf("\nAuthentication Error: %v\n", notLoggedIn)
			var rooms []Room
			cookies, rooms, err = recoverCookiesViaTelegram(ctx, notLoggedIn.Error())
			if err != nil {
				return err
			}

			liveCount := countLiveRooms(rooms)
			fmt.Printf(
				"[%s] Recovered with %d/%d live\n",
				nowFunc().Format("15:04:05"),
				liveCount,
				len(rooms),
			)
			updateHealthState(liveCount)
			previousLive = processRoomNotifications(ctx, rooms, previousLive)
			pingOffset, err = waitWithPingChecks(ctx, pollInterval, pingOffset)
			if err != nil {
				return err
			}
			continue
		}

		var apiError *DouyuAPIError
		if errors.As(err, &apiError) {
			fmt.Printf("\nAPI Error: %v\n", apiError)
			fmt.Println("Retrying in 30 seconds...")
			pingOffset, err = waitWithPingChecks(ctx, 30*time.Second, pingOffset)
			if err != nil {
				return err
			}
			continue
		}

		fmt.Printf("\nUnexpected error: %v\n", err)
		fmt.Println("Retrying in 30 seconds...")
		pingOffset, err = waitWithPingChecks(ctx, 30*time.Second, pingOffset)
		if err != nil {
			return err
		}
	}
}

func countLiveRooms(rooms []Room) int {
	count := 0
	for _, room := range rooms {
		if room.IsLive {
			count++
		}
	}
	return count
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Println("\n\nStopping...")
			return
		}
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
