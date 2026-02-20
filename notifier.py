import requests
from typing import List, Optional, Set
from models import Room
from config import TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID

TELEGRAM_API = f"https://api.telegram.org/bot{TELEGRAM_BOT_TOKEN}/sendMessage"


def send_telegram(text: str):
    """Send a message via Telegram bot."""
    requests.post(TELEGRAM_API, json={
        "chat_id": TELEGRAM_CHAT_ID,
        "text": text,
        "parse_mode": "HTML",
    }, timeout=10)


def notify_new_live(rooms: List[Room], previous_live: Optional[Set[str]]) -> Set[str]:
    """
    Send Telegram notifications for newly live streamers.

    Args:
        rooms: List of Room objects
        previous_live: Set of room_ids that were live in previous check

    Returns:
        Set of currently live room IDs
    """
    current_live = {r.room_id for r in rooms if r.is_live}

    # Only notify on new streams (skip first run to avoid spam)
    if previous_live is not None:
        new_live_ids = current_live - previous_live
        for room in rooms:
            if room.room_id in new_live_ids:
                text = (
                    f"<b>{room.streamer_name}</b> is now live!\n"
                    f"{room.room_name}\n"
                    f"Category: {room.area_name}\n"
                    f"<a href=\"{room.url}\">Watch</a>"
                )
                send_telegram(text)
                print(f"  Notified: {room.streamer_name} is live")

    return current_live
