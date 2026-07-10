# Douyu Live Status Notifier

Monitors followed streamers on Douyu TV and sends Telegram notifications when a stream starts or ends. The notifier is implemented in Go and has no third-party runtime dependencies.

## Requirements

- Go 1.22 or newer
- A Telegram bot token and chat ID
- Cookies from a logged-in Douyu account

## Setup

1. Create a Telegram bot with [@BotFather](https://t.me/BotFather) and copy its token.
2. Send the bot a message, then find your chat ID through `https://api.telegram.org/bot<token>/getUpdates`.
3. Export both values:

   ```bash
   export TELEGRAM_BOT_TOKEN="your-bot-token"
   export TELEGRAM_CHAT_ID="your-chat-id"
   ```

4. Build the notifier:

   ```bash
   go build -o douyu-notifier .
   ```

## Usage

```bash
./douyu-notifier
# Alternatively: go run .
```

If `cookies.json` is missing or Douyu rejects the saved cookies, the bot asks for a replacement in the configured Telegram chat. Copy the full `Cookie` header from a logged-in request to `douyu.com`, then reply in this form:

```text
name=value; name2=value2
```

The notifier validates the reply before saving it to `cookies.json`. Treat that file and the Telegram reply as secrets. Only one process may poll a given Telegram bot token; the notifier disables an existing webhook and reports a clear error if another `getUpdates` consumer is active.

The application:

- polls Douyu every three minutes;
- notifies when a followed room becomes live;
- notifies when a previously live room goes offline;
- avoids transition notifications while establishing the initial live-state snapshot;
- continues processing `/ping` between Douyu polls; and
- reports uptime, last-poll age, and live-streamer count in `/ping` replies.

Press Ctrl+C to stop.

## Configuration

Runtime secrets come from `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID`. Polling, retry, API, and timeout constants are in [`config.go`](config.go). The default polling interval is:

```go
pollInterval = 180 * time.Second
```

## Development

Run the test suite and build locally with:

```bash
go test ./...
go build ./...
```

Format changes before committing:

```bash
gofmt -w *.go
```

## Project Structure

```text
DouyuNotifier/
├── main.go           # Startup, cookie validation, recovery, and polling loop
├── config.go         # Endpoints, intervals, and environment configuration
├── auth.go           # Cookie parsing and local persistence
├── fetcher.go        # Douyu API client and response parsing
├── notifier.go       # Telegram polling, commands, and notifications
├── models.go         # Shared room and error types
├── *_test.go         # Go unit and integration-style tests
└── go.mod            # Go module metadata
```
