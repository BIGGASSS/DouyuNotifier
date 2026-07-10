package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testRoom(roomID string, live bool) Room {
	return Room{
		RoomID:       roomID,
		RoomName:     "Room",
		StreamerName: "Streamer",
		IsLive:       live,
		AreaName:     "Game",
		URL:          "https://www.douyu.com/1",
		Platform:     "douyu",
	}
}

func TestValidateCookiesRetriesTransientAPIError(t *testing.T) {
	originalFetch := fetchDouyuLiveStatusFunc
	originalSleep := sleepContextFunc
	calls, sleeps := 0, 0
	fetchDouyuLiveStatusFunc = func(context.Context, map[string]string) ([]Room, error) {
		calls++
		if calls == 1 {
			return nil, &DouyuAPIError{Message: "temporary"}
		}
		return []Room{testRoom("dy_1", true)}, nil
	}
	sleepContextFunc = func(context.Context, time.Duration) error {
		sleeps++
		return nil
	}
	t.Cleanup(func() {
		fetchDouyuLiveStatusFunc = originalFetch
		sleepContextFunc = originalSleep
	})

	rooms, err := validateCookies(context.Background(), map[string]string{"acf_uid": "123"})
	if err != nil {
		t.Fatalf("validateCookies() error = %v", err)
	}
	if calls != 2 || sleeps != 1 || !reflect.DeepEqual(rooms, []Room{testRoom("dy_1", true)}) {
		t.Fatalf("calls = %d, sleeps = %d, rooms = %#v", calls, sleeps, rooms)
	}
}

func TestValidateCookiesDoesNotRetryInvalidCookie(t *testing.T) {
	originalFetch := fetchDouyuLiveStatusFunc
	originalSleep := sleepContextFunc
	calls := 0
	fetchDouyuLiveStatusFunc = func(context.Context, map[string]string) ([]Room, error) {
		calls++
		return nil, &NotLoginError{Message: "expired"}
	}
	sleepContextFunc = func(context.Context, time.Duration) error {
		t.Fatal("validateCookies slept after authentication error")
		return nil
	}
	t.Cleanup(func() {
		fetchDouyuLiveStatusFunc = originalFetch
		sleepContextFunc = originalSleep
	})

	_, err := validateCookies(context.Background(), map[string]string{"acf_uid": "123"})
	var notLoggedIn *NotLoginError
	if !errors.As(err, &notLoggedIn) || calls != 1 {
		t.Fatalf("error = %T %v, calls = %d", err, err, calls)
	}
}

func TestValidateCookiesStopsAfterConfiguredRetries(t *testing.T) {
	originalFetch := fetchDouyuLiveStatusFunc
	originalSleep := sleepContextFunc
	calls, sleeps := 0, 0
	fetchDouyuLiveStatusFunc = func(context.Context, map[string]string) ([]Room, error) {
		calls++
		return nil, &DouyuAPIError{Message: "still down"}
	}
	sleepContextFunc = func(context.Context, time.Duration) error { sleeps++; return nil }
	t.Cleanup(func() {
		fetchDouyuLiveStatusFunc = originalFetch
		sleepContextFunc = originalSleep
	})

	_, err := validateCookies(context.Background(), nil)
	var apiError *DouyuAPIError
	if !errors.As(err, &apiError) || calls != cookieValidationRetries || sleeps != cookieValidationRetries-1 {
		t.Fatalf("error = %T %v, calls = %d, sleeps = %d", err, err, calls, sleeps)
	}
}

func TestProcessRoomNotificationsUsesSamePreviousSnapshot(t *testing.T) {
	originalSend := sendTelegramFunc
	var sent []string
	sendTelegramFunc = func(_ context.Context, text string) bool {
		sent = append(sent, text)
		return true
	}
	t.Cleanup(func() { sendTelegramFunc = originalSend })

	current := processRoomNotifications(
		context.Background(),
		[]Room{testRoom("dy_1", false)},
		map[string]struct{}{"dy_1": {}},
	)
	if len(current) != 0 {
		t.Fatalf("current = %#v, want empty", current)
	}
	if len(sent) != 1 || sent[0] != "<b>Streamer</b> has ended their stream." {
		t.Fatalf("sent = %#v", sent)
	}
}

func TestWaitWithPingChecksUsesRemainingLongPollTimeouts(t *testing.T) {
	originalNow := nowFunc
	originalProcess := processPingCommandsFunc
	times := []time.Time{
		time.Unix(100, 0),
		time.Unix(100, 0),
		time.Unix(130, 0),
		time.Unix(171, 0),
	}
	index := 0
	nowFunc = func() time.Time {
		if index >= len(times) {
			t.Fatalf("nowFunc called too many times")
		}
		value := times[index]
		index++
		return value
	}
	type pingCall struct {
		offset  int64
		timeout int
	}
	var calls []pingCall
	processPingCommandsFunc = func(_ context.Context, offset int64, timeout int) (int64, error) {
		calls = append(calls, pingCall{offset: offset, timeout: timeout})
		return offset + 1, nil
	}
	t.Cleanup(func() {
		nowFunc = originalNow
		processPingCommandsFunc = originalProcess
	})

	offset, err := waitWithPingChecks(context.Background(), 70*time.Second, 19)
	if err != nil {
		t.Fatalf("waitWithPingChecks() error = %v", err)
	}
	wantCalls := []pingCall{{offset: 19, timeout: 60}, {offset: 20, timeout: 40}}
	if offset != 21 || !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("offset = %d, calls = %#v, want %#v", offset, calls, wantCalls)
	}
}

func TestWaitWithPingChecksUsesShortRemainingTimeout(t *testing.T) {
	originalNow := nowFunc
	originalProcess := processPingCommandsFunc
	times := []time.Time{
		time.Unix(100, 0),
		time.Unix(100, 0),
		time.Unix(104, 200_000_000),
	}
	index := 0
	nowFunc = func() time.Time {
		value := times[index]
		index++
		return value
	}
	var gotOffset int64
	var gotTimeout int
	processPingCommandsFunc = func(_ context.Context, offset int64, timeout int) (int64, error) {
		gotOffset, gotTimeout = offset, timeout
		return 55, nil
	}
	t.Cleanup(func() {
		nowFunc = originalNow
		processPingCommandsFunc = originalProcess
	})

	offset, err := waitWithPingChecks(context.Background(), 4*time.Second, 12)
	if err != nil || offset != 55 || gotOffset != 12 || gotTimeout != 4 {
		t.Fatalf(
			"offset = %d, error = %v, ping call = (%d, %d)",
			offset,
			err,
			gotOffset,
			gotTimeout,
		)
	}
}

func TestWaitWithPingChecksHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	offset, err := waitWithPingChecks(ctx, time.Minute, 12)
	if !errors.Is(err, context.Canceled) || offset != 12 {
		t.Fatalf("offset = %d, error = %v", offset, err)
	}
}

func TestRecoverCookiesViaTelegramValidatesAndSavesReply(t *testing.T) {
	originalClient := telegramHTTPClient
	originalToken, originalChatID := telegramBotToken, telegramChatID
	originalFetch := fetchDouyuLiveStatusFunc
	originalSend := sendTelegramFunc
	originalFile := cookiesFile

	telegramBotToken, telegramChatID = "TOKEN", "123"
	cookiesFile = filepath.Join(t.TempDir(), "cookies.json")
	fetchDouyuLiveStatusFunc = func(_ context.Context, cookies map[string]string) ([]Room, error) {
		want := map[string]string{"acf_uid": "1", "dy_did": "2"}
		if !reflect.DeepEqual(cookies, want) {
			t.Fatalf("validated cookies = %#v, want %#v", cookies, want)
		}
		return []Room{testRoom("dy_1", true)}, nil
	}
	sendTelegramFunc = sendTelegram
	setTelegramPreparedForTest(t, false)

	updatesCalls := 0
	var sentMessages []string
	telegramHTTPClient = httpDoerFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/deleteWebhook"):
			return testHTTPResponse(http.StatusOK, `{"ok":true}`), nil
		case strings.HasSuffix(request.URL.Path, "/getUpdates"):
			updatesCalls++
			if updatesCalls == 1 {
				if request.URL.Query().Get("offset") != "" || request.URL.Query().Get("timeout") != "0" {
					t.Fatalf("initial getUpdates query = %q", request.URL.RawQuery)
				}
				return testHTTPResponse(http.StatusOK, `{
					"ok":true,
					"result":[{"update_id":5,"message":{"chat":{"id":123},"text":"old"}}]
				}`), nil
			}
			if request.URL.Query().Get("offset") != "6" {
				t.Fatalf("reply offset = %q, want 6", request.URL.Query().Get("offset"))
			}
			return testHTTPResponse(http.StatusOK, `{
				"ok":true,
				"result":[{"update_id":6,"message":{"chat":{"id":123},"text":"Cookie: acf_uid=1; dy_did=2"}}]
			}`), nil
		case strings.HasSuffix(request.URL.Path, "/sendMessage"):
			var payload map[string]string
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatalf("decode Telegram message: %v", err)
			}
			sentMessages = append(sentMessages, payload["text"])
			return testHTTPResponse(http.StatusOK, `{"ok":true}`), nil
		default:
			t.Fatalf("unexpected Telegram request: %s", request.URL)
			return nil, nil
		}
	})
	t.Cleanup(func() {
		telegramHTTPClient = originalClient
		telegramBotToken, telegramChatID = originalToken, originalChatID
		fetchDouyuLiveStatusFunc = originalFetch
		sendTelegramFunc = originalSend
		cookiesFile = originalFile
	})

	cookies, rooms, err := recoverCookiesViaTelegram(context.Background(), "expired <cookie>")
	if err != nil {
		t.Fatalf("recoverCookiesViaTelegram() error = %v", err)
	}
	if len(cookies) != 2 || len(rooms) != 1 || updatesCalls != 2 {
		t.Fatalf("cookies = %#v, rooms = %#v, update calls = %d", cookies, rooms, updatesCalls)
	}
	if len(sentMessages) != 2 {
		t.Fatalf("sent messages = %#v, want prompt and success", sentMessages)
	}
	if !strings.Contains(sentMessages[0], "expired &lt;cookie&gt;") {
		t.Fatalf("prompt did not HTML-escape reason: %q", sentMessages[0])
	}
	if got := loadCookies(); !reflect.DeepEqual(got, cookies) {
		t.Fatalf("saved cookies = %#v, want %#v", got, cookies)
	}
}

func TestRunPollsAndProcessesOfflineTransition(t *testing.T) {
	originalToken, originalChatID := telegramBotToken, telegramChatID
	originalFile := cookiesFile
	originalFetch := fetchDouyuLiveStatusFunc
	originalProcess := processPingCommandsFunc
	originalSend := sendTelegramFunc
	telegramBotToken, telegramChatID = "TOKEN", "123"
	cookiesFile = filepath.Join(t.TempDir(), "cookies.json")
	saveCookies(map[string]string{"acf_uid": "1"})

	fetchCalls := 0
	fetchDouyuLiveStatusFunc = func(context.Context, map[string]string) ([]Room, error) {
		fetchCalls++
		if fetchCalls == 1 {
			return []Room{testRoom("dy_1", true)}, nil
		}
		return []Room{testRoom("dy_1", false)}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	processPingCommandsFunc = func(_ context.Context, offset int64, _ int) (int64, error) {
		cancel()
		return offset + 1, nil
	}
	var sent []string
	sendTelegramFunc = func(_ context.Context, text string) bool {
		sent = append(sent, text)
		return true
	}
	t.Cleanup(func() {
		cancel()
		telegramBotToken, telegramChatID = originalToken, originalChatID
		cookiesFile = originalFile
		fetchDouyuLiveStatusFunc = originalFetch
		processPingCommandsFunc = originalProcess
		sendTelegramFunc = originalSend
	})

	err := run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run() error = %v, want context.Canceled", err)
	}
	if fetchCalls != 2 {
		t.Fatalf("fetch calls = %d, want initial validation and first poll", fetchCalls)
	}
	if len(sent) != 1 || sent[0] != "<b>Streamer</b> has ended their stream." {
		t.Fatalf("sent = %#v", sent)
	}
}

func TestRunRequiresTelegramConfiguration(t *testing.T) {
	originalToken, originalChatID := telegramBotToken, telegramChatID
	telegramBotToken, telegramChatID = "", ""
	t.Cleanup(func() { telegramBotToken, telegramChatID = originalToken, originalChatID })

	err := run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "TELEGRAM_BOT_TOKEN") {
		t.Fatalf("run() error = %v", err)
	}
}
