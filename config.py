import os

POLL_INTERVAL = 180  # 3 minutes in seconds
API_URL = "https://www.douyu.com/wgapi/livenc/liveweb/follow/list"

TELEGRAM_BOT_TOKEN = os.environ.get("TELEGRAM_BOT_TOKEN", "")
TELEGRAM_CHAT_ID = os.environ.get("TELEGRAM_CHAT_ID", "")
