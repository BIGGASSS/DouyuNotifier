# Douyu Live Status Notifier

Monitors followed streamers on Douyu TV and sends Telegram notifications when a stream starts or ends. It is implemented in Go with no third-party runtime dependencies.

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
   go build -o douyu-notifier ./cmd/douyu-notifier
   ```

## Usage

```bash
./douyu-notifier
# Alternatively:
go run ./cmd/douyu-notifier
```

If `cookies.json` is missing or Douyu rejects the saved cookies, the bot asks for a replacement in the configured Telegram chat. Copy the full `Cookie` header from a logged-in request to `douyu.com`, then reply with `name=value; name2=value2`.

The notifier validates the reply before saving it to `cookies.json` in the process working directory with mode `0600`. Treat that file and the Telegram reply as secrets. Only one process may poll a given Telegram bot token; the notifier disables an existing webhook. A polling conflict while awaiting a cookie reply is reported with recovery guidance, while an optional `/ping` poll conflict does not stop Douyu monitoring.

The application:

- polls Douyu every three minutes;
- notifies when a followed room becomes live or goes offline;
- avoids transition notifications while establishing the initial snapshot;
- processes `/ping` between Douyu polls; and
- reports uptime, last-poll age, and live-streamer count in `/ping` replies.

Press Ctrl+C to stop.

## Configuration

Runtime secrets come from `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID`. Endpoints, intervals, and timeouts are defined in [`internal/config/config.go`](internal/config/config.go).

## Development

```bash
go test ./...
go test -race ./...
go vet ./...
go build -o douyu-notifier ./cmd/douyu-notifier
```

Format Go changes with `gofmt -w` on the changed files.

## Project Structure

```text
cmd/douyu-notifier/  # executable wiring, signal handling, and exit behavior
internal/app/         # startup, cookie recovery, polling, and orchestration
internal/config/      # environment loading and runtime defaults
internal/cookies/     # cookie parsing and 0600 local persistence
internal/douyu/       # Douyu HTTP client, response parsing, and errors
internal/model/       # shared domain types (model.Room)
internal/telegram/    # Telegram API, /ping, health, and notifications
```
