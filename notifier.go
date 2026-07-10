package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	telegramHTTPClient httpDoer = &http.Client{}

	telegramPreparationMu sync.Mutex
	telegramUpdatesReady  bool

	healthMu          sync.RWMutex
	startTime         = time.Now()
	lastPollTime      time.Time
	hasLastPoll       bool
	liveStreamerCount int

	nowFunc          = time.Now
	sleepContextFunc = sleepContext
	sendTelegramFunc = sendTelegram
)

type telegramChat struct {
	ID json.RawMessage `json:"id"`
}

type telegramMessage struct {
	Chat telegramChat `json:"chat"`
	Text string       `json:"text"`
}

type telegramUpdate struct {
	UpdateID      int64            `json:"update_id"`
	Message       *telegramMessage `json:"message"`
	EditedMessage *telegramMessage `json:"edited_message"`
}

type telegramAPIResponse struct {
	OK          bool             `json:"ok"`
	Description json.RawMessage  `json:"description"`
	Result      []telegramUpdate `json:"result"`
}

func telegramAPIEndpoint(method string) string {
	return "https://api.telegram.org/bot" + telegramBotToken + "/" + method
}

// updateHealthState records information returned by the /ping command.
func updateHealthState(liveCount int) {
	healthMu.Lock()
	defer healthMu.Unlock()
	lastPollTime = nowFunc()
	hasLastPoll = true
	liveStreamerCount = liveCount
}

func handlePingCommand(ctx context.Context) {
	now := nowFunc()
	healthMu.RLock()
	started := startTime
	polledAt := lastPollTime
	wasPolled := hasLastPoll
	liveCount := liveStreamerCount
	healthMu.RUnlock()

	uptimeSeconds := int(now.Sub(started).Seconds())
	hours := uptimeSeconds / 3600
	minutes := (uptimeSeconds % 3600) / 60
	seconds := uptimeSeconds % 60

	var uptime string
	switch {
	case hours != 0:
		uptime = fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	case minutes != 0:
		uptime = fmt.Sprintf("%dm %ds", minutes, seconds)
	default:
		uptime = fmt.Sprintf("%ds", seconds)
	}

	lastPoll := "never"
	if wasPolled {
		secondsAgo := int(now.Sub(polledAt).Seconds())
		if secondsAgo < 60 {
			lastPoll = fmt.Sprintf("%ds ago", secondsAgo)
		} else {
			lastPoll = fmt.Sprintf("%dm ago", secondsAgo/60)
		}
	}

	text := fmt.Sprintf(
		"<b>Pong!</b>\nUptime: %s\nLast poll: %s\nLive streamers: %d",
		uptime,
		lastPoll,
		liveCount,
	)
	sendTelegramFunc(ctx, text)
}

func processPingCommands(ctx context.Context, offset int64, timeout int) (int64, error) {
	updates, nextOffset, err := getTelegramUpdates(ctx, &offset, timeout)
	if err != nil {
		var conflict *TelegramPollingConflict
		if errors.As(err, &conflict) {
			return offset, nil
		}
		return offset, err
	}

	for _, update := range updates {
		message := update.Message
		if message == nil {
			message = update.EditedMessage
		}
		if message == nil || telegramChatIDString(message.Chat.ID) != telegramChatID {
			continue
		}
		if strings.TrimSpace(message.Text) == "/ping" {
			handlePingCommand(ctx)
		}
	}
	return nextOffset, nil
}

// sendTelegram sends one HTML-formatted message to the configured chat.
func sendTelegram(ctx context.Context, text string) bool {
	body, _ := json.Marshal(map[string]string{
		"chat_id":    telegramChatID,
		"text":       text,
		"parse_mode": "HTML",
	})

	requestContext, cancel := context.WithTimeout(ctx, telegramRequestGracePeriod)
	defer cancel()
	req, err := http.NewRequestWithContext(
		requestContext,
		http.MethodPost,
		telegramAPIEndpoint("sendMessage"),
		bytes.NewReader(body),
	)
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		var response *http.Response
		response, err = telegramHTTPClient.Do(req)
		if err == nil {
			defer response.Body.Close()
			_, _ = io.Copy(io.Discard, response.Body)
			if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusBadRequest {
				return true
			}
			err = fmt.Errorf("HTTP status %s", response.Status)
		}
	}

	if ctx.Err() == nil {
		fmt.Printf("Warning: Failed to send Telegram message: %v\n", err)
	}
	return false
}

func getNextUpdateOffset(ctx context.Context) (int64, error) {
	updates, nextOffset, err := getTelegramUpdates(ctx, nil, 0)
	if err != nil {
		return 0, err
	}
	if len(updates) > 0 {
		return nextOffset, nil
	}
	return 0, nil
}

// prepareTelegramUpdates disables webhooks once so getUpdates can long poll.
func prepareTelegramUpdates(ctx context.Context) error {
	telegramPreparationMu.Lock()
	defer telegramPreparationMu.Unlock()
	if telegramUpdatesReady {
		return nil
	}

	body := []byte(`{"drop_pending_updates":false}`)
	requestContext, cancel := context.WithTimeout(ctx, telegramRequestGracePeriod)
	defer cancel()
	req, err := http.NewRequestWithContext(
		requestContext,
		http.MethodPost,
		telegramAPIEndpoint("deleteWebhook"),
		bytes.NewReader(body),
	)
	if err != nil {
		fmt.Printf("Warning: Failed to delete Telegram webhook: %v\n", err)
		return nil
	}
	req.Header.Set("Content-Type", "application/json")

	response, err := telegramHTTPClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Printf("Warning: Failed to delete Telegram webhook: %v\n", err)
		return nil
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		fmt.Printf("Warning: Failed to delete Telegram webhook: %v\n", readErr)
		return nil
	}

	description := extractTelegramDescription(responseBody)
	if response.StatusCode == http.StatusConflict {
		return &TelegramPollingConflict{Message: buildPollingConflictMessage(description)}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusBadRequest {
		fmt.Printf(
			"Warning: Telegram deleteWebhook failed: %d %s\n",
			response.StatusCode,
			description,
		)
		return nil
	}

	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		telegramUpdatesReady = true
		return nil
	}
	if ok, _ := payload["ok"].(bool); !ok {
		fmt.Printf("Warning: Telegram deleteWebhook returned an error: %s\n", strings.TrimSpace(string(responseBody)))
		return nil
	}

	telegramUpdatesReady = true
	return nil
}

func getTelegramUpdates(
	ctx context.Context,
	offset *int64,
	timeout int,
) ([]telegramUpdate, int64, error) {
	if err := prepareTelegramUpdates(ctx); err != nil {
		return nil, offsetValue(offset), err
	}

	endpoint, err := url.Parse(telegramAPIEndpoint("getUpdates"))
	if err != nil {
		return nil, offsetValue(offset), err
	}
	query := endpoint.Query()
	query.Set("timeout", strconv.Itoa(timeout))
	if offset != nil {
		query.Set("offset", strconv.FormatInt(*offset, 10))
	}
	endpoint.RawQuery = query.Encode()

	requestTimeout := time.Duration(timeout)*time.Second + telegramRequestGracePeriod
	requestContext, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestContext, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, offsetValue(offset), err
	}

	response, err := telegramHTTPClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, offsetValue(offset), ctx.Err()
		}
		fmt.Printf("Warning: Failed to fetch Telegram updates: %v\n", err)
		return nil, offsetValue(offset), nil
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		fmt.Printf("Warning: Failed to fetch Telegram updates: %v\n", readErr)
		return nil, offsetValue(offset), nil
	}

	description := extractTelegramDescription(responseBody)
	if response.StatusCode == http.StatusConflict {
		return nil, offsetValue(offset), &TelegramPollingConflict{
			Message: buildPollingConflictMessage(description),
		}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusBadRequest {
		fmt.Printf(
			"Warning: Telegram updates request failed: %d %s\n",
			response.StatusCode,
			description,
		)
		return nil, offsetValue(offset), nil
	}

	var payload telegramAPIResponse
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, offsetValue(offset), fmt.Errorf("invalid Telegram response: %w", err)
	}
	if !payload.OK {
		fmt.Printf("Warning: Telegram API returned an error: %s\n", strings.TrimSpace(string(responseBody)))
		return nil, offsetValue(offset), nil
	}

	nextOffset := offsetValue(offset)
	if len(payload.Result) > 0 {
		nextOffset = payload.Result[len(payload.Result)-1].UpdateID + 1
	}
	return payload.Result, nextOffset, nil
}

func waitForChatMessage(ctx context.Context, offset int64) (string, int64, error) {
	currentOffset := offset
	for {
		updates, nextOffset, err := getTelegramUpdates(ctx, &currentOffset, telegramLongPollTimeout)
		if err != nil {
			return "", currentOffset, err
		}
		currentOffset = nextOffset
		if len(updates) == 0 {
			if err := sleepContextFunc(ctx, 5*time.Second); err != nil {
				return "", currentOffset, err
			}
			continue
		}

		for _, update := range updates {
			message := update.Message
			if message == nil {
				message = update.EditedMessage
			}
			if message == nil || telegramChatIDString(message.Chat.ID) != telegramChatID {
				continue
			}
			if text := strings.TrimSpace(message.Text); text != "" {
				return text, currentOffset, nil
			}
		}
	}
}

func extractTelegramDescription(body []byte) string {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return strings.TrimSpace(string(body))
	}
	description, ok := payload["description"]
	if !ok {
		return ""
	}
	return strings.TrimSpace(rawScalarString(description, ""))
}

func buildPollingConflictMessage(description string) string {
	if strings.Contains(strings.ToLower(description), "webhook") {
		return "Telegram bot polling is blocked by an active webhook. " +
			"Disable the webhook for this bot token and try again."
	}
	return "Telegram getUpdates is already being consumed by another process for " +
		"this bot token. Stop the other bot instance or use a different bot token."
}

// notifyNewLive sends one message for each room that became live.
func notifyNewLive(ctx context.Context, rooms []Room, previousLive map[string]struct{}) map[string]struct{} {
	currentLive := currentLiveRoomIDs(rooms)
	if previousLive != nil {
		for _, room := range rooms {
			if _, liveNow := currentLive[room.RoomID]; !liveNow {
				continue
			}
			if _, wasLive := previousLive[room.RoomID]; wasLive {
				continue
			}

			text := fmt.Sprintf(
				"<b>%s</b> is now live!\n%s\nCategory: %s\n<a href=\"%s\">Watch</a>",
				htmlEscape(room.StreamerName),
				htmlEscape(room.RoomName),
				htmlEscape(room.AreaName),
				room.URL,
			)
			sendTelegramFunc(ctx, text)
			fmt.Printf("  Notified: %s is live\n", room.StreamerName)
		}
	}
	return currentLive
}

// notifyStreamEnd sends one message for each room that stopped streaming.
func notifyStreamEnd(ctx context.Context, rooms []Room, previousLive map[string]struct{}) map[string]struct{} {
	currentLive := currentLiveRoomIDs(rooms)
	if previousLive == nil {
		return currentLive
	}

	roomLookup := make(map[string]Room, len(rooms))
	for _, room := range rooms {
		roomLookup[room.RoomID] = room
	}
	for roomID := range previousLive {
		if _, stillLive := currentLive[roomID]; stillLive {
			continue
		}

		streamerName := roomID
		if room, ok := roomLookup[roomID]; ok {
			streamerName = htmlEscape(room.StreamerName)
		}
		sendTelegramFunc(ctx, fmt.Sprintf("<b>%s</b> has ended their stream.", streamerName))
		fmt.Printf("  Notified: %s ended stream\n", streamerName)
	}
	return currentLive
}

func currentLiveRoomIDs(rooms []Room) map[string]struct{} {
	currentLive := map[string]struct{}{}
	for _, room := range rooms {
		if room.IsLive {
			currentLive[room.RoomID] = struct{}{}
		}
	}
	return currentLive
}

func htmlEscape(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	text = strings.ReplaceAll(text, "\"", "&quot;")
	return strings.ReplaceAll(text, "'", "&#x27;")
}

func telegramChatIDString(raw json.RawMessage) string {
	return rawScalarString(raw, "")
}

func offsetValue(offset *int64) int64 {
	if offset == nil {
		return 0
	}
	return *offset
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
