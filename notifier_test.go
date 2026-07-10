package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type httpDoerFunc func(*http.Request) (*http.Response, error)

func (f httpDoerFunc) Do(request *http.Request) (*http.Response, error) {
	return f(request)
}

func testHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestGetTelegramUpdatesRaisesClearConflictOn409(t *testing.T) {
	setTelegramPreparedForTest(t, true)
	originalClient := telegramHTTPClient
	telegramHTTPClient = httpDoerFunc(func(*http.Request) (*http.Response, error) {
		return testHTTPResponse(http.StatusConflict, `{
			"ok": false,
			"description": "Conflict: terminated by other getUpdates request"
		}`), nil
	})
	t.Cleanup(func() { telegramHTTPClient = originalClient })

	_, _, err := getTelegramUpdates(context.Background(), nil, telegramLongPollTimeout)
	var conflict *TelegramPollingConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %T %v, want *TelegramPollingConflict", err, err)
	}
	if !strings.Contains(conflict.Error(), "already being consumed") {
		t.Fatalf("conflict message = %q", conflict.Error())
	}
}

func TestPrepareTelegramUpdatesDeletesWebhookOnce(t *testing.T) {
	setTelegramPreparedForTest(t, false)
	originalClient := telegramHTTPClient
	calls := 0
	telegramHTTPClient = httpDoerFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.URL.Path != "/botTOKEN/deleteWebhook" {
			t.Fatalf("request path = %q", request.URL.Path)
		}
		var body map[string]bool
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["drop_pending_updates"] {
			t.Fatal("drop_pending_updates should be false")
		}
		return testHTTPResponse(http.StatusOK, `{"ok":true,"result":true}`), nil
	})
	originalToken := telegramBotToken
	telegramBotToken = "TOKEN"
	t.Cleanup(func() {
		telegramHTTPClient = originalClient
		telegramBotToken = originalToken
	})

	if err := prepareTelegramUpdates(context.Background()); err != nil {
		t.Fatalf("prepareTelegramUpdates() error = %v", err)
	}
	if err := prepareTelegramUpdates(context.Background()); err != nil {
		t.Fatalf("second prepareTelegramUpdates() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("deleteWebhook calls = %d, want 1", calls)
	}
}

func TestGetTelegramUpdatesPassesOffsetAndTimeout(t *testing.T) {
	setTelegramPreparedForTest(t, true)
	originalClient := telegramHTTPClient
	telegramHTTPClient = httpDoerFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.URL.Query().Get("offset"); got != "41" {
			t.Fatalf("offset query = %q", got)
		}
		if got := request.URL.Query().Get("timeout"); got != "17" {
			t.Fatalf("timeout query = %q", got)
		}
		return testHTTPResponse(http.StatusOK, `{
			"ok": true,
			"result": [{"update_id": 44, "message": {"chat": {"id": 7}, "text": "hello"}}]
		}`), nil
	})
	t.Cleanup(func() { telegramHTTPClient = originalClient })

	offset := int64(41)
	updates, nextOffset, err := getTelegramUpdates(context.Background(), &offset, 17)
	if err != nil {
		t.Fatalf("getTelegramUpdates() error = %v", err)
	}
	if len(updates) != 1 || nextOffset != 45 {
		t.Fatalf("updates = %#v, next offset = %d", updates, nextOffset)
	}
}

func TestProcessPingCommandsHandlesConfiguredChat(t *testing.T) {
	setTelegramPreparedForTest(t, true)
	originalClient := telegramHTTPClient
	telegramHTTPClient = httpDoerFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("timeout") != "17" {
			t.Fatalf("timeout = %q", request.URL.Query().Get("timeout"))
		}
		return testHTTPResponse(http.StatusOK, `{
			"ok": true,
			"result": [
				{"update_id": 41, "message": {"chat": {"id": 99}, "text": "/ping"}},
				{"update_id": 42, "edited_message": {"chat": {"id": 7}, "text": " /ping "}}
			]
		}`), nil
	})
	originalChatID := telegramChatID
	telegramChatID = "7"
	originalSend := sendTelegramFunc
	var sent []string
	sendTelegramFunc = func(_ context.Context, text string) bool {
		sent = append(sent, text)
		return true
	}
	t.Cleanup(func() {
		telegramHTTPClient = originalClient
		telegramChatID = originalChatID
		sendTelegramFunc = originalSend
	})

	nextOffset, err := processPingCommands(context.Background(), 40, 17)
	if err != nil {
		t.Fatalf("processPingCommands() error = %v", err)
	}
	if nextOffset != 43 {
		t.Fatalf("next offset = %d, want 43", nextOffset)
	}
	if len(sent) != 1 || !strings.Contains(sent[0], "<b>Pong!</b>") {
		t.Fatalf("sent messages = %#v", sent)
	}
}

func TestProcessPingCommandsIgnoresPollingConflict(t *testing.T) {
	setTelegramPreparedForTest(t, true)
	originalClient := telegramHTTPClient
	telegramHTTPClient = httpDoerFunc(func(*http.Request) (*http.Response, error) {
		return testHTTPResponse(
			http.StatusConflict,
			`{"ok":false,"description":"terminated by another getUpdates request"}`,
		), nil
	})
	t.Cleanup(func() { telegramHTTPClient = originalClient })

	offset, err := processPingCommands(context.Background(), 17, 0)
	if err != nil || offset != 17 {
		t.Fatalf("offset = %d, error = %v", offset, err)
	}
}

func TestPingShowsHealthStatus(t *testing.T) {
	originalNow := nowFunc
	originalSend := sendTelegramFunc
	healthMu.Lock()
	originalStart, originalLast := startTime, lastPollTime
	originalHasLast, originalLive := hasLastPoll, liveStreamerCount
	startTime = time.Unix(1000, 0)
	hasLastPoll = false
	liveStreamerCount = 0
	healthMu.Unlock()

	nowFunc = func() time.Time { return time.Unix(1365, 0) }
	var sent string
	sendTelegramFunc = func(_ context.Context, text string) bool {
		sent = text
		return true
	}
	t.Cleanup(func() {
		nowFunc = originalNow
		sendTelegramFunc = originalSend
		healthMu.Lock()
		startTime, lastPollTime = originalStart, originalLast
		hasLastPoll, liveStreamerCount = originalHasLast, originalLive
		healthMu.Unlock()
	})

	updateHealthState(3)
	handlePingCommand(context.Background())
	for _, want := range []string{"<b>Pong!</b>", "Uptime: 6m 5s", "Last poll: 0s ago", "Live streamers: 3"} {
		if !strings.Contains(sent, want) {
			t.Fatalf("ping response %q does not contain %q", sent, want)
		}
	}
}

func TestPingShowsHoursAndMinutesSincePoll(t *testing.T) {
	originalNow := nowFunc
	originalSend := sendTelegramFunc
	healthMu.Lock()
	originalStart, originalLast := startTime, lastPollTime
	originalHasLast, originalLive := hasLastPoll, liveStreamerCount
	startTime = time.Unix(1000, 0)
	lastPollTime = time.Unix(7200, 0)
	hasLastPoll = true
	liveStreamerCount = 5
	healthMu.Unlock()
	nowFunc = func() time.Time { return time.Unix(7261, 0) }
	var sent string
	sendTelegramFunc = func(_ context.Context, text string) bool { sent = text; return true }
	t.Cleanup(func() {
		nowFunc = originalNow
		sendTelegramFunc = originalSend
		healthMu.Lock()
		startTime, lastPollTime = originalStart, originalLast
		hasLastPoll, liveStreamerCount = originalHasLast, originalLive
		healthMu.Unlock()
	})

	handlePingCommand(context.Background())
	for _, want := range []string{"Uptime: 1h 44m 21s", "Last poll: 1m ago", "Live streamers: 5"} {
		if !strings.Contains(sent, want) {
			t.Fatalf("ping response %q does not contain %q", sent, want)
		}
	}
}

func TestPingShowsRecentPollInSeconds(t *testing.T) {
	originalNow := nowFunc
	originalSend := sendTelegramFunc
	healthMu.Lock()
	originalStart, originalLast := startTime, lastPollTime
	originalHasLast, originalLive := hasLastPoll, liveStreamerCount
	startTime = time.Unix(1000, 0)
	lastPollTime = time.Unix(1090, 0)
	hasLastPoll = true
	liveStreamerCount = 0
	healthMu.Unlock()
	nowFunc = func() time.Time { return time.Unix(1100, 0) }
	var sent string
	sendTelegramFunc = func(_ context.Context, text string) bool { sent = text; return true }
	t.Cleanup(func() {
		nowFunc = originalNow
		sendTelegramFunc = originalSend
		healthMu.Lock()
		startTime, lastPollTime = originalStart, originalLast
		hasLastPoll, liveStreamerCount = originalHasLast, originalLive
		healthMu.Unlock()
	})

	handlePingCommand(context.Background())
	if !strings.Contains(sent, "Last poll: 10s ago") {
		t.Fatalf("ping response = %q", sent)
	}
}

func TestNotifyNewLiveSkipsFirstRunAndEscapesHTML(t *testing.T) {
	originalSend := sendTelegramFunc
	var sent []string
	sendTelegramFunc = func(_ context.Context, text string) bool {
		sent = append(sent, text)
		return true
	}
	t.Cleanup(func() { sendTelegramFunc = originalSend })

	rooms := []Room{{
		RoomID:       "dy_1",
		StreamerName: `A&B<Z>"'`,
		RoomName:     "Room & <fun>",
		AreaName:     "Game",
		URL:          "https://www.douyu.com/1",
		IsLive:       true,
	}}
	first := notifyNewLive(context.Background(), rooms, nil)
	if len(sent) != 0 {
		t.Fatalf("first run sent %#v", sent)
	}
	notifyNewLive(context.Background(), rooms, map[string]struct{}{})
	if len(first) != 1 || len(sent) != 1 {
		t.Fatalf("first set = %#v, sent = %#v", first, sent)
	}
	if want := `<b>A&amp;B&lt;Z&gt;&quot;&#x27;</b> is now live!`; !strings.Contains(sent[0], want) {
		t.Fatalf("message %q does not contain %q", sent[0], want)
	}
}

func TestNotifyStreamEndSkipsFirstRunAndUnchangedRooms(t *testing.T) {
	originalSend := sendTelegramFunc
	var sent []string
	sendTelegramFunc = func(_ context.Context, text string) bool {
		sent = append(sent, text)
		return true
	}
	t.Cleanup(func() { sendTelegramFunc = originalSend })

	rooms := []Room{{RoomID: "dy_1", StreamerName: "Alice", IsLive: true}}
	first := notifyStreamEnd(context.Background(), rooms, nil)
	stillLive := notifyStreamEnd(context.Background(), rooms, map[string]struct{}{"dy_1": {}})
	newlyLive := notifyStreamEnd(context.Background(), rooms, map[string]struct{}{})
	if len(first) != 1 || len(stillLive) != 1 || len(newlyLive) != 1 {
		t.Fatalf("live sets = first %#v, unchanged %#v, new %#v", first, stillLive, newlyLive)
	}
	if len(sent) != 0 {
		t.Fatalf("sent = %#v, want no stream-end messages", sent)
	}
}

func TestNotifyStreamEndHandlesMultipleRooms(t *testing.T) {
	originalSend := sendTelegramFunc
	var sent []string
	sendTelegramFunc = func(_ context.Context, text string) bool {
		sent = append(sent, text)
		return true
	}
	t.Cleanup(func() { sendTelegramFunc = originalSend })

	rooms := []Room{
		{RoomID: "dy_1", StreamerName: "Alice", IsLive: false},
		{RoomID: "dy_2", StreamerName: "Bob", IsLive: false},
	}
	current := notifyStreamEnd(
		context.Background(),
		rooms,
		map[string]struct{}{"dy_1": {}, "dy_2": {}},
	)
	if len(current) != 0 || len(sent) != 2 {
		t.Fatalf("current = %#v, sent = %#v", current, sent)
	}
	joined := strings.Join(sent, "\n")
	if !strings.Contains(joined, "<b>Alice</b>") || !strings.Contains(joined, "<b>Bob</b>") {
		t.Fatalf("sent = %#v", sent)
	}
}

func TestNotifyStreamEndTransitions(t *testing.T) {
	originalSend := sendTelegramFunc
	var sent []string
	sendTelegramFunc = func(_ context.Context, text string) bool {
		sent = append(sent, text)
		return true
	}
	t.Cleanup(func() { sendTelegramFunc = originalSend })

	rooms := []Room{
		{RoomID: "dy_1", StreamerName: "Alice", IsLive: false},
		{RoomID: "dy_2", StreamerName: "Bob", IsLive: true},
	}
	current := notifyStreamEnd(
		context.Background(),
		rooms,
		map[string]struct{}{"dy_1": {}, "dy_2": {}},
	)
	if len(current) != 1 {
		t.Fatalf("current live = %#v", current)
	}
	if len(sent) != 1 || sent[0] != "<b>Alice</b> has ended their stream." {
		t.Fatalf("sent = %#v", sent)
	}
}

func TestSendTelegramUsesHTMLPayload(t *testing.T) {
	originalClient := telegramHTTPClient
	originalToken, originalChatID := telegramBotToken, telegramChatID
	telegramBotToken, telegramChatID = "TOKEN", "123"
	telegramHTTPClient = httpDoerFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/botTOKEN/sendMessage" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var payload map[string]string
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["chat_id"] != "123" || payload["parse_mode"] != "HTML" || payload["text"] != "<b>hello</b>" {
			t.Fatalf("payload = %#v", payload)
		}
		return testHTTPResponse(http.StatusOK, `{"ok":true}`), nil
	})
	t.Cleanup(func() {
		telegramHTTPClient = originalClient
		telegramBotToken, telegramChatID = originalToken, originalChatID
	})

	if !sendTelegram(context.Background(), "<b>hello</b>") {
		t.Fatal("sendTelegram() = false, want true")
	}
}

func TestBuildPollingConflictMessageForWebhook(t *testing.T) {
	got := buildPollingConflictMessage("Conflict: can't use getUpdates while webhook is active")
	if !strings.Contains(got, "active webhook") {
		t.Fatalf("message = %q", got)
	}
}

func TestTelegramEndpointProducesValidURL(t *testing.T) {
	originalToken := telegramBotToken
	telegramBotToken = "123:abc"
	t.Cleanup(func() { telegramBotToken = originalToken })
	if _, err := url.ParseRequestURI(telegramAPIEndpoint("getUpdates")); err != nil {
		t.Fatalf("endpoint is not a valid URL: %v", err)
	}
}

func setTelegramPreparedForTest(t *testing.T, ready bool) {
	t.Helper()
	telegramPreparationMu.Lock()
	original := telegramUpdatesReady
	telegramUpdatesReady = ready
	telegramPreparationMu.Unlock()
	t.Cleanup(func() {
		telegramPreparationMu.Lock()
		telegramUpdatesReady = original
		telegramPreparationMu.Unlock()
	})
}
