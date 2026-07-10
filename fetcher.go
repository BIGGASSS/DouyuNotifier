package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

var douyuHTTPClient httpDoer = &http.Client{Timeout: douyuRequestTimeout}

type douyuAPIResponse struct {
	Message json.RawMessage `json:"msg"`
	Error   json.RawMessage `json:"error"`
	Data    struct {
		Rooms []map[string]json.RawMessage `json:"list"`
	} `json:"data"`
}

// fetchDouyuLiveStatus fetches and parses the authenticated Douyu follow list.
func fetchDouyuLiveStatus(ctx context.Context, cookies map[string]string) ([]Room, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, douyuAPIURL, nil)
	if err != nil {
		return nil, &DouyuAPIError{Message: fmt.Sprintf("Request failed: %v", err)}
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://www.douyu.com/")
	req.Header.Set("Cookie", cookieHeader(cookies))

	response, err := douyuHTTPClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &DouyuAPIError{Message: fmt.Sprintf("Request failed: %v", err)}
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, &DouyuAPIError{Message: fmt.Sprintf("Request failed: %v", err)}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusBadRequest {
		return nil, &DouyuAPIError{Message: fmt.Sprintf("Request failed: HTTP status %s", response.Status)}
	}

	var payload douyuAPIResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &DouyuAPIError{Message: fmt.Sprintf("Request failed: invalid JSON response: %v", err)}
	}

	message := rawText(payload.Message, "")
	errorCode := rawScalarString(payload.Error, "None")
	if errorCode == "-1" && strings.Contains(message, "未登") {
		return nil, &NotLoginError{Message: message}
	}
	if errorCode != "0" {
		detail := message
		if detail == "" {
			detail = strings.TrimSpace(string(body))
		}
		return nil, &DouyuAPIError{Message: fmt.Sprintf("API error: %s", detail)}
	}

	return parseResponse(payload), nil
}

func cookieHeader(cookies map[string]string) string {
	keys := make([]string, 0, len(cookies))
	for key := range cookies {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+cookies[key])
	}
	return strings.Join(parts, "; ")
}

// parseResponse converts a successful Douyu response into Room values.
func parseResponse(payload douyuAPIResponse) []Room {
	rooms := make([]Room, 0, len(payload.Data.Rooms))
	for _, roomData := range payload.Data.Rooms {
		roomID := rawScalarString(roomData["room_id"], "")
		pathRaw, hasPath := roomData["url"]
		path := "/" + roomID
		if hasPath {
			path = rawText(pathRaw, "")
		}

		showStatus := rawNumber(roomData["show_status"], 0)
		videoLoop := rawNumber(roomData["videoLoop"], 0)
		rooms = append(rooms, Room{
			RoomID:       "dy_" + roomID,
			RoomName:     rawText(roomData["room_name"], ""),
			StreamerName: rawText(roomData["nickname"], ""),
			Cover:        rawText(roomData["room_src"], ""),
			Avatar:       rawText(roomData["avatar_small"], ""),
			IsLive:       showStatus == 1 && videoLoop == 0,
			AreaName:     rawText(roomData["game_name"], ""),
			URL:          "https://www.douyu.com" + path,
			Platform:     "douyu",
		})
	}
	return rooms
}

func rawText(raw json.RawMessage, fallback string) string {
	if len(raw) == 0 || string(raw) == "null" {
		return fallback
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return rawScalarString(raw, fallback)
}

func rawScalarString(raw json.RawMessage, fallback string) string {
	if len(raw) == 0 {
		return fallback
	}

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "null" {
		return "None"
	}
	if trimmed == "true" {
		return "True"
	}
	if trimmed == "false" {
		return "False"
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return trimmed
}

func rawNumber(raw json.RawMessage, fallback float64) float64 {
	if len(raw) == 0 {
		return fallback
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "true" {
		return 1
	}
	if trimmed == "false" {
		return 0
	}
	number, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return fallback
	}
	return number
}
