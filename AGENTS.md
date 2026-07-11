# Repository Guidelines

## Project Structure & Module Organization
The executable lives in `cmd/douyu-notifier`; keep process signals and exit behavior there. Application orchestration belongs in `internal/app`, environment/default configuration in `internal/config`, cookie parsing and persistence in `internal/cookies`, provider clients and provider-specific errors in `internal/douyu` and `internal/telegram`, and shared domain types in `internal/model`. Tests live beside each package as `*_test.go` files.

## Build, Test, and Development Commands
Use Go 1.22 or newer:

```bash
go run ./cmd/douyu-notifier
go test ./...
go build -o douyu-notifier ./cmd/douyu-notifier
```

Run focused tests with commands such as `go test -run TestValidateCookies ./internal/app`. Format all Go changes with `gofmt`. The application uses only the standard library.

## Coding Style & Naming Conventions
Follow idiomatic Go. Use short lower-case names for package-private helpers and `PascalCase` only for exported identifiers. Pass `context.Context` through blocking network and wait operations. Keep modules small and single-purpose, keep provider-specific failures in their provider package, and close HTTP response bodies. Prefer constructor-injected interfaces and instance dependencies to package-global test seams. Avoid third-party dependencies.

## Testing Guidelines
Tests use Go's `testing` package, `httptest`, and injected interfaces or HTTP clients at API boundaries. Add or update tests for every behavior change, especially cookie parsing/storage, validation retries, Douyu parsing/errors, Telegram polling conflicts/webhook handling, cookie recovery, `/ping`, and live/offline transitions. Before submitting, run:

```bash
go test ./...
go test -race ./...
go vet ./...
go build -o douyu-notifier ./cmd/douyu-notifier
```

## Commit & Pull Request Guidelines
Use short, imperative commit subjects and keep commits focused. Pull requests should explain behavior and configuration impact and list verification performed. For notification changes, include a sample message or log snippet.

## Security & Configuration Tips
Never commit real Douyu cookies, bot tokens, chat IDs, a populated `cookies.json`, or built binaries. Configure secrets through `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID`. `cookies.json` remains relative to the process working directory and must use mode `0600`.
