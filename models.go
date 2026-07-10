package main

import "fmt"

// Room describes one followed Douyu room.
type Room struct {
	RoomID       string
	RoomName     string
	StreamerName string
	Cover        string
	Avatar       string
	IsLive       bool
	AreaName     string
	URL          string
	Platform     string
}

func (r Room) String() string {
	status := "OFFLINE"
	if r.IsLive {
		status = "LIVE"
	}
	return fmt.Sprintf("[%s] %s: %s (%s)", status, r.StreamerName, r.RoomName, r.AreaName)
}

// NotLoginError reports that Douyu rejected the current cookies.
type NotLoginError struct {
	Message string
}

func (e *NotLoginError) Error() string { return e.Message }

// DouyuAPIError reports a temporary or non-authentication Douyu API failure.
type DouyuAPIError struct {
	Message string
}

func (e *DouyuAPIError) Error() string { return e.Message }

// TelegramPollingConflict reports that getUpdates cannot be used by this process.
type TelegramPollingConflict struct {
	Message string
}

func (e *TelegramPollingConflict) Error() string { return e.Message }
