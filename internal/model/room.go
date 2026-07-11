package model

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
