import time
from html import escape
from typing import Any, Dict, List, Optional, Set, Tuple

import requests

from config import (
    TELEGRAM_BOT_TOKEN,
    TELEGRAM_CHAT_ID,
    TELEGRAM_LONG_POLL_TIMEOUT,
)
from models import Room, TelegramPollingConflict

TELEGRAM_API_BASE = f"https://api.telegram.org/bot{TELEGRAM_BOT_TOKEN}"
TELEGRAM_SEND_MESSAGE_API = f"{TELEGRAM_API_BASE}/sendMessage"
TELEGRAM_GET_UPDATES_API = f"{TELEGRAM_API_BASE}/getUpdates"
TELEGRAM_DELETE_WEBHOOK_API = f"{TELEGRAM_API_BASE}/deleteWebhook"

_TELEGRAM_UPDATES_PREPARED = False

_START_TIME = time.time()
_last_poll_time: Optional[float] = None
_live_streamer_count: int = 0


def update_health_state(live_count: int) -> None:
    """Update health state tracking for /ping responses."""
    global _last_poll_time, _live_streamer_count
    _last_poll_time = time.time()
    _live_streamer_count = live_count


def _handle_ping_command() -> None:
    """Respond to a /ping command with bot health status."""
    import datetime

    uptime_seconds = int(time.time() - _START_TIME)
    hours, remainder = divmod(uptime_seconds, 3600)
    minutes, seconds = divmod(remainder, 60)

    if hours:
        uptime_str = f"{hours}h {minutes}m {seconds}s"
    elif minutes:
        uptime_str = f"{minutes}m {seconds}s"
    else:
        uptime_str = f"{seconds}s"

    if _last_poll_time is not None:
        seconds_ago = int(time.time() - _last_poll_time)
        if seconds_ago < 60:
            last_poll_str = f"{seconds_ago}s ago"
        else:
            last_poll_str = f"{seconds_ago // 60}m ago"
    else:
        last_poll_str = "never"

    text = (
        f"<b>Pong!</b>\n"
        f"Uptime: {uptime_str}\n"
        f"Last poll: {last_poll_str}\n"
        f"Live streamers: {_live_streamer_count}"
    )
    send_telegram(text)


def _process_ping_commands(offset: int) -> int:
    """
    Check for and handle any /ping commands in the Telegram update queue.

    Args:
        offset: Current Telegram update offset

    Returns:
        Updated offset after consuming /ping messages
    """
    current_offset = offset
    try:
        updates, current_offset = get_telegram_updates(
            offset=current_offset, timeout=0
        )
    except TelegramPollingConflict:
        return current_offset

    for update in updates:
        message = update.get('message') or update.get('edited_message') or {}
        chat = message.get('chat') or {}
        if str(chat.get('id', '')) != str(TELEGRAM_CHAT_ID):
            continue

        text = message.get('text', '').strip()
        if text == '/ping':
            _handle_ping_command()

    return current_offset


def send_telegram(text: str) -> bool:
    """Send a message via Telegram bot."""
    try:
        response = requests.post(
            TELEGRAM_SEND_MESSAGE_API,
            json={
                'chat_id': TELEGRAM_CHAT_ID,
                'text': text,
                'parse_mode': 'HTML',
            },
            timeout=10,
        )
        response.raise_for_status()
        return True
    except requests.exceptions.RequestException as error:
        print(f'Warning: Failed to send Telegram message: {error}')
        return False


def get_next_update_offset() -> int:
    """
    Return the next update offset so only future chat messages are consumed.

    Returns:
        Telegram update offset for the next unread message
    """
    updates, next_offset = get_telegram_updates(timeout=0)
    if updates:
        return next_offset
    return 0


def prepare_telegram_updates() -> None:
    """Disable Telegram webhooks once so long polling can use getUpdates."""
    global _TELEGRAM_UPDATES_PREPARED

    if _TELEGRAM_UPDATES_PREPARED:
        return

    try:
        response = requests.post(
            TELEGRAM_DELETE_WEBHOOK_API,
            json={'drop_pending_updates': False},
            timeout=10,
        )
    except requests.exceptions.RequestException as error:
        print(f'Warning: Failed to delete Telegram webhook: {error}')
        return

    description = _extract_telegram_description(response)
    if response.status_code == 409:
        raise TelegramPollingConflict(_build_polling_conflict_message(description))

    if not response.ok:
        print(
            'Warning: Telegram deleteWebhook failed: '
            f'{response.status_code} {description}'
        )
        return

    try:
        payload = response.json()
    except ValueError:
        payload = {'ok': True}

    if not payload.get('ok', False):
        print(f'Warning: Telegram deleteWebhook returned an error: {payload}')
        return

    _TELEGRAM_UPDATES_PREPARED = True


def get_telegram_updates(
    offset: Optional[int] = None,
    timeout: int = TELEGRAM_LONG_POLL_TIMEOUT,
) -> Tuple[List[Dict[str, Any]], int]:
    """
    Fetch Telegram bot updates.

    Args:
        offset: Optional Telegram update offset
        timeout: Long-poll timeout in seconds

    Returns:
        Tuple of updates and the next update offset
    """
    prepare_telegram_updates()
    params: Dict[str, Any] = {'timeout': timeout}
    if offset is not None:
        params['offset'] = offset

    try:
        response = requests.get(
            TELEGRAM_GET_UPDATES_API,
            params=params,
            timeout=timeout + 10,
        )
    except requests.exceptions.RequestException as error:
        print(f'Warning: Failed to fetch Telegram updates: {error}')
        return [], offset or 0

    description = _extract_telegram_description(response)
    if response.status_code == 409:
        raise TelegramPollingConflict(_build_polling_conflict_message(description))

    if not response.ok:
        print(
            'Warning: Telegram updates request failed: '
            f'{response.status_code} {description}'
        )
        return [], offset or 0

    payload = response.json()

    if not payload.get('ok'):
        print(f'Warning: Telegram API returned an error: {payload}')
        return [], offset or 0

    updates = payload.get('result', [])
    next_offset = offset or 0
    if updates:
        next_offset = int(updates[-1]['update_id']) + 1

    return updates, next_offset


def wait_for_chat_message(offset: int) -> Tuple[Optional[str], int]:
    """
    Wait for the next text message from the configured Telegram chat.

    Args:
        offset: Telegram update offset to start from

    Returns:
        Tuple of message text and next update offset
    """
    current_offset = offset

    while True:
        updates, current_offset = get_telegram_updates(offset=current_offset)
        if not updates:
            time.sleep(5)
            continue

        for update in updates:
            message = update.get('message') or update.get('edited_message') or {}
            chat = message.get('chat') or {}
            if str(chat.get('id', '')) != str(TELEGRAM_CHAT_ID):
                continue

            text = message.get('text', '').strip()
            if text:
                return text, current_offset


def _extract_telegram_description(response: requests.Response) -> str:
    """Extract Telegram API error descriptions safely."""
    try:
        payload = response.json()
    except ValueError:
        return response.text.strip()

    return str(payload.get('description', '')).strip()


def _build_polling_conflict_message(description: str) -> str:
    """Return a readable explanation for Telegram getUpdates conflicts."""
    if 'webhook' in description.lower():
        return (
            'Telegram bot polling is blocked by an active webhook. '
            'Disable the webhook for this bot token and try again.'
        )

    return (
        'Telegram getUpdates is already being consumed by another process for '
        'this bot token. Stop the other bot instance or use a different bot token.'
    )


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
                    f'<b>{escape(room.streamer_name)}</b> is now live!\n'
                    f'{escape(room.room_name)}\n'
                    f'Category: {escape(room.area_name)}\n'
                    f"<a href=\"{room.url}\">Watch</a>"
                )
                send_telegram(text)
                print(f"  Notified: {room.streamer_name} is live")

    return current_live
