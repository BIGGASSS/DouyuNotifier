# Repository Guidelines

## Project Structure & Module Organization
This repository is a small, flat Go application using package `main`. `main.go` drives startup, cookie recovery, and the polling loop; `fetcher.go` talks to Douyu; `notifier.go` handles Telegram messaging and `/ping`; `auth.go` parses and stores cookies; `config.go` contains endpoints, intervals, and environment lookups; and `models.go` defines shared room and error types. Tests live beside the implementation as `*_test.go` files.

## Build, Test, and Development Commands
Use the standard Go toolchain (Go 1.22 or newer):

```bash
go run .
go test ./...
go build -o douyu-notifier .
```

Run a focused test with a command such as `go test -run TestValidateCookies ./...`. Format all Go changes with `gofmt -w *.go`. The application uses only the standard library, so no separate dependency installation is required.

## Coding Style & Naming Conventions
Follow idiomatic Go and keep all source formatted with `gofmt`. Use short lower-case names for package-private helpers and `PascalCase` only for exported identifiers. Pass `context.Context` through blocking network and wait operations. Keep modules small and single-purpose, wrap behavior-specific failures in the existing typed errors, and close HTTP response bodies. Avoid adding a third-party dependency when the standard library is sufficient.

## Testing Guidelines
Tests use Go's `testing` package, `httptest`, and small injected function or HTTP-client fakes at API boundaries. Add or update tests for every behavior change, especially cookie parsing, validation retries, Douyu response parsing, Telegram polling conflicts, cookie recovery, `/ping`, and live/offline transition notifications. Do not mark tests parallel when they replace package-level test seams. Run `go test ./...` and `go test -race ./...` before submitting.

## Commit & Pull Request Guidelines
Recent history uses short, imperative subjects such as `Fix 409 error` and `Add support for cookie expiration notification`. Keep commits focused and subjects concise. Pull requests should explain the behavior change, note configuration or environment impact, and list verification performed. For user-facing notification changes, include a sample message or log snippet.

## Security & Configuration Tips
Never commit real Douyu cookies, bot tokens, chat IDs, a populated `cookies.json`, or built binaries. Configure secrets through `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID`, and treat Telegram cookie replies and `cookies.json` as sensitive local state. Keep cookie-file permissions restrictive.
