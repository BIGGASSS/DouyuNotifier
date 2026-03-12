import os

POLL_INTERVAL = 180  # 3 minutes in seconds
API_URL = "https://www.douyu.com/wgapi/livenc/liveweb/follow/list"
COOKIE_VALIDATION_RETRY_DELAY = 10  # seconds between retrying validation after API errors
COOKIE_VALIDATION_RETRIES = 3
TELEGRAM_LONG_POLL_TIMEOUT = 60  # seconds for getUpdates long polling

TELEGRAM_BOT_TOKEN = os.environ.get("TELEGRAM_BOT_TOKEN", "")
TELEGRAM_CHAT_ID = os.environ.get("TELEGRAM_CHAT_ID", "")
