# Douyu Live Status Notifier

Monitors your followed streamers on Douyu TV and sends Telegram notifications when they go live.

## Setup

1. Install `uv` if it is not already available on your system.

2. Create a virtual environment and install dependencies:
```bash
uv venv
source .venv/bin/activate
uv pip install -r requirements.txt
```

3. Create a Telegram bot via [@BotFather](https://t.me/BotFather) and get the bot token

4. Get your Telegram chat ID (send a message to your bot, then check `https://api.telegram.org/bot<token>/getUpdates`)

5. Set environment variables:
```bash
export TELEGRAM_BOT_TOKEN="your-bot-token"
export TELEGRAM_CHAT_ID="your-chat-id"
```

6. Log into [douyu.com](https://www.douyu.com) in your browser

## Usage

```bash
uv run python main.py
```

On first run, the program will prompt you to paste your Douyu cookies. You can copy them from Chrome DevTools (F12) > Network tab > click any request to douyu.com > copy the `Cookie` header value. Cookies are saved to `cookies.json` for future runs.

The program polls every 3 minutes and sends a Telegram message whenever a followed streamer goes live.

## Project Structure

```
DouyuNotifier/
├── main.py           # Entry point and polling loop
├── config.py         # Settings and environment variables
├── auth.py           # Cookie loading and manual input
├── fetcher.py        # Douyu API client
├── notifier.py       # Telegram notifications
├── models.py         # Room dataclass and exceptions
└── requirements.txt  # Dependencies
```

## Configuration

Edit `config.py` to change the polling interval:

```python
POLL_INTERVAL = 180  # seconds (default: 3 minutes)
```

## Requirements

- Python 3.7+
- uv
- requests
- A Telegram bot token and chat ID
- Douyu account cookies
