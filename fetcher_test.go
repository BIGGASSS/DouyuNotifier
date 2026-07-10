package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchDouyuLiveStatusParsesRoomsAndHeaders(t *testing.T) {
	var cookieHeaderValue string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookieHeaderValue = r.Header.Get("Cookie")
		if r.Header.Get("Referer") != "https://www.douyu.com/" {
			t.Errorf("Referer = %q", r.Header.Get("Referer"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"error": 0,
			"msg": "",
			"data": {"list": [
				{
					"room_id": 123,
					"room_name": "Ranked games",
					"nickname": "Alice",
					"room_src": "cover.jpg",
					"avatar_small": "avatar.jpg",
					"show_status": 1,
					"videoLoop": 0,
					"game_name": "Game",
					"url": "/123"
				},
				{
					"room_id": "456",
					"nickname": "Bob",
					"show_status": 1,
					"videoLoop": 1
				}
			]}
		}`))
	}))
	defer server.Close()

	originalURL, originalClient := douyuAPIURL, douyuHTTPClient
	douyuAPIURL, douyuHTTPClient = server.URL, server.Client()
	t.Cleanup(func() {
		douyuAPIURL, douyuHTTPClient = originalURL, originalClient
	})

	rooms, err := fetchDouyuLiveStatus(context.Background(), map[string]string{
		"dy_did":  "abc",
		"acf_uid": "123",
	})
	if err != nil {
		t.Fatalf("fetchDouyuLiveStatus() error = %v", err)
	}
	if cookieHeaderValue != "acf_uid=123; dy_did=abc" {
		t.Fatalf("Cookie header = %q", cookieHeaderValue)
	}
	if len(rooms) != 2 {
		t.Fatalf("len(rooms) = %d, want 2", len(rooms))
	}
	if rooms[0].RoomID != "dy_123" || !rooms[0].IsLive || rooms[0].Platform != "douyu" {
		t.Fatalf("first room = %#v", rooms[0])
	}
	if rooms[0].URL != "https://www.douyu.com/123" {
		t.Fatalf("first room URL = %q", rooms[0].URL)
	}
	if rooms[1].IsLive {
		t.Fatalf("video-loop room should be offline: %#v", rooms[1])
	}
	if rooms[1].URL != "https://www.douyu.com/456" {
		t.Fatalf("fallback room URL = %q", rooms[1].URL)
	}
}

func TestFetchDouyuLiveStatusReturnsNotLoginError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"error":-1,"msg":"未登录，请重新登录"}`))
	}))
	defer server.Close()
	useDouyuTestServer(t, server)

	_, err := fetchDouyuLiveStatus(context.Background(), map[string]string{"cookie": "value"})
	var notLoggedIn *NotLoginError
	if !errors.As(err, &notLoggedIn) {
		t.Fatalf("error = %T %v, want *NotLoginError", err, err)
	}
}

func TestFetchDouyuLiveStatusReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"error":2,"msg":"busy"}`))
	}))
	defer server.Close()
	useDouyuTestServer(t, server)

	_, err := fetchDouyuLiveStatus(context.Background(), nil)
	var apiError *DouyuAPIError
	if !errors.As(err, &apiError) || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("error = %T %v, want API error containing busy", err, err)
	}
}

func TestFetchDouyuLiveStatusWrapsHTTPAndJSONFailures(t *testing.T) {
	t.Run("HTTP status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "down", http.StatusBadGateway)
		}))
		defer server.Close()
		useDouyuTestServer(t, server)

		_, err := fetchDouyuLiveStatus(context.Background(), nil)
		var apiError *DouyuAPIError
		if !errors.As(err, &apiError) {
			t.Fatalf("error = %T %v, want *DouyuAPIError", err, err)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("not-json"))
		}))
		defer server.Close()
		useDouyuTestServer(t, server)

		_, err := fetchDouyuLiveStatus(context.Background(), nil)
		var apiError *DouyuAPIError
		if !errors.As(err, &apiError) {
			t.Fatalf("error = %T %v, want *DouyuAPIError", err, err)
		}
	})
}

func useDouyuTestServer(t *testing.T, server *httptest.Server) {
	t.Helper()
	originalURL, originalClient := douyuAPIURL, douyuHTTPClient
	douyuAPIURL, douyuHTTPClient = server.URL, server.Client()
	t.Cleanup(func() {
		douyuAPIURL, douyuHTTPClient = originalURL, originalClient
	})
}
