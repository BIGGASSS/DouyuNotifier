# Repository Guidelines

## Project Structure & Module Organization
This repository is a small flat Python application. Runtime modules live at the repo root: `main.py` drives the polling loop, `fetcher.py` talks to Douyu, `notifier.py` handles Telegram messaging, `auth.py` manages cookie input/storage, `config.py` holds constants and environment lookups, and `models.py` defines shared data structures and exceptions. Tests also live at the root as `test_*.py`.

## Build, Test, and Development Commands
Use `uv` for local setup, matching the README:

```bash
uv venv
source .venv/bin/activate
uv pip install -e .
```

Run the notifier locally with `uv run python main.py`. Run the full test suite with `python -m unittest`. For focused work, run a single file such as `python -m unittest test_auth`.

## Coding Style & Naming Conventions
Follow the existing Python style: 4-space indentation, `snake_case` for functions and variables, `PascalCase` for classes, and explicit type hints on public functions. Keep modules small and single-purpose. Prefer short docstrings where behavior is not obvious. Match the current standard library-first import order, then third-party imports such as `requests`.

No formatter or linter is configured in-repo, so contributors should stay close to PEP 8 and existing file conventions rather than introducing a new tool ad hoc.

## Testing Guidelines
Tests use the standard `unittest` framework with `unittest.mock` for API boundaries. Add or update `test_*.py` files for every behavior change, especially around cookie parsing, retry logic, Telegram polling conflicts, and notification flow. Keep test names descriptive, for example `test_retries_transient_api_error`.

## Commit & Pull Request Guidelines
Recent history uses short, imperative commit subjects such as `Fix 409 error` and `Add support for cookie expiration notification`. Keep commits focused and message subjects concise. Pull requests should explain the behavior change, note any config or environment impact, and list the verification performed. For user-facing notification changes, include a sample message or log snippet.

## Security & Configuration Tips
Never commit real Douyu cookies, bot tokens, chat IDs, or a populated `cookies.json`. Configure secrets through `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID`, and treat `cookies.json` as local machine state only.
