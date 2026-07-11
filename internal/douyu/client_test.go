package douyu

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(request *http.Request) (*http.Response, error) { return f(request) }

func TestFetchParsesRoomsAndHeaders(t *testing.T) {
	var cookie string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie = r.Header.Get("Cookie")
		if r.Header.Get("Referer") != "https://www.douyu.com/" {
			t.Errorf("Referer = %q", r.Header.Get("Referer"))
		}
		_, _ = w.Write([]byte(`{"error":0,"msg":"","data":{"list":[{"room_id":123,"room_name":"Ranked games","nickname":"Alice","room_src":"cover.jpg","avatar_small":"avatar.jpg","show_status":1,"videoLoop":0,"game_name":"Game","url":"/123"},{"room_id":"456","nickname":"Bob","show_status":1,"videoLoop":1}]}}`))
	}))
	defer server.Close()
	rooms, err := NewClient(server.URL, server.Client()).Fetch(context.Background(), map[string]string{"dy_did": "abc", "acf_uid": "123"})
	if err != nil {
		t.Fatal(err)
	}
	if cookie != "acf_uid=123; dy_did=abc" {
		t.Fatalf("Cookie = %q", cookie)
	}
	if len(rooms) != 2 || rooms[0].RoomID != "dy_123" || !rooms[0].IsLive || rooms[0].Platform != "douyu" || rooms[0].URL != "https://www.douyu.com/123" {
		t.Fatalf("rooms = %#v", rooms)
	}
	if rooms[1].IsLive || rooms[1].URL != "https://www.douyu.com/456" {
		t.Fatalf("second room = %#v", rooms[1])
	}
}

func TestFetchTransportFailureHonorsCancellationAndClosesResponses(t *testing.T) {
	t.Run("transport failure", func(t *testing.T) {
		_, err := NewClient("https://example.test", doerFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network down")
		})).Fetch(context.Background(), nil)
		var api *APIError
		if !errors.As(err, &api) || !strings.Contains(err.Error(), "network down") {
			t.Fatalf("error=%T %v", err, err)
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := NewClient("https://example.test", doerFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network down")
		})).Fetch(ctx, nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%T %v", err, err)
		}
	})
	t.Run("response body closed", func(t *testing.T) {
		body := &trackedBody{Reader: strings.NewReader("down")}
		_, err := NewClient("https://example.test", doerFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusBadGateway, Status: "502 Bad Gateway", Body: body}, nil
		})).Fetch(context.Background(), nil)
		var api *APIError
		if !errors.As(err, &api) || !body.closed {
			t.Fatalf("error=%T %v closed=%t", err, err, body.closed)
		}
	})
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error {
	b.closed = true
	return nil
}

func TestFetchErrors(t *testing.T) {
	tests := []struct {
		name, body string
		status     int
		check      func(error) bool
	}{
		{"not login", `{"error":-1,"msg":"未登录，请重新登录"}`, 200, func(err error) bool { var target *NotLoginError; return errors.As(err, &target) }},
		{"api", `{"error":2,"msg":"busy"}`, 200, func(err error) bool {
			var target *APIError
			return errors.As(err, &target) && strings.Contains(err.Error(), "busy")
		}},
		{"status", `down`, 502, func(err error) bool { var target *APIError; return errors.As(err, &target) }},
		{"json", `not-json`, 200, func(err error) bool { var target *APIError; return errors.As(err, &target) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			_, err := NewClient(server.URL, server.Client()).Fetch(context.Background(), nil)
			if !tt.check(err) {
				t.Fatalf("error = %T %v", err, err)
			}
		})
	}
}
