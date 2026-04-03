#!/usr/bin/env python3
"""
Douyu Live Status Notifier

Fetches live status of followed streamers from Douyu TV
and sends Telegram notifications when a stream starts.
"""

import time
import sys
from html import escape
from datetime import datetime
from typing import Dict, List, Optional, Set, Tuple

from auth import load_cookies, parse_cookie_string, save_cookies
from config import (
    COOKIE_VALIDATION_RETRIES,
    COOKIE_VALIDATION_RETRY_DELAY,
    POLL_INTERVAL,
    TELEGRAM_BOT_TOKEN,
    TELEGRAM_CHAT_ID,
    TELEGRAM_LONG_POLL_TIMEOUT,
)
from fetcher import fetch_douyu_live_status
from models import DouyuAPIError, NotLoginError, Room, TelegramPollingConflict
from notifier import (
    _process_ping_commands,
    get_next_update_offset,
    notify_new_live,
    send_telegram,
    update_health_state,
    wait_for_chat_message,
)


def validate_cookies(cookies: Dict[str, str]) -> List[Room]:
    """
    Validate cookies by calling Douyu and retrying transient API failures.

    Args:
        cookies: Cookie dictionary to validate

    Returns:
        Live room list returned by Douyu

    Raises:
        NotLoginError: If Douyu rejects the cookies
        DouyuAPIError: If validation fails after retries
    """
    last_error: Optional[DouyuAPIError] = None

    for attempt in range(1, COOKIE_VALIDATION_RETRIES + 1):
        try:
            return fetch_douyu_live_status(cookies)
        except NotLoginError:
            raise
        except DouyuAPIError as error:
            last_error = error
            if attempt == COOKIE_VALIDATION_RETRIES:
                break

            print(
                f'Cookie validation hit a temporary API error '
                f'({attempt}/{COOKIE_VALIDATION_RETRIES}): {error}'
            )
            time.sleep(COOKIE_VALIDATION_RETRY_DELAY)

    raise last_error or DouyuAPIError('Cookie validation failed.')


def recover_cookies_via_telegram(reason: str) -> Tuple[Dict[str, str], List[Room]]:
    """
    Ask the configured Telegram chat for a replacement cookie until one works.

    Args:
        reason: Human-readable reason the existing cookie failed

    Returns:
        Tuple of validated cookies and the corresponding room list
    """
    print(f'Authentication requires a new cookie: {reason}')
    try:
        next_offset = get_next_update_offset()
    except TelegramPollingConflict as error:
        print(f'Telegram polling conflict: {error}')
        send_telegram(
            'I could send notifications, but I cannot receive your reply because '
            f'this bot token has a Telegram polling conflict.\nReason: '
            f'<code>{escape(str(error))}</code>'
        )
        raise

    prompt_sent = send_telegram(
        'Douyu cookie expired or is invalid.\n'
        f'Reason: <code>{escape(reason)}</code>\n'
        'Reply in this chat with a fresh full cookie string in the format:\n'
        '<code>name=value; name2=value2</code>'
    )

    if prompt_sent:
        print('Sent Telegram prompt for a new cookie.')
    else:
        print('Failed to send Telegram prompt; still listening for chat replies.')

    while True:
        print('Waiting for a new cookie in Telegram...')
        try:
            cookie_message, next_offset = wait_for_chat_message(next_offset)
        except TelegramPollingConflict as error:
            print(f'Telegram polling conflict: {error}')
            send_telegram(
                'I cannot receive your cookie reply because this bot token has a '
                f'Telegram polling conflict.\nReason: <code>{escape(str(error))}</code>'
            )
            raise

        if not cookie_message:
            continue

        candidate_cookies = parse_cookie_string(cookie_message)
        if not candidate_cookies:
            print('Received a Telegram message that could not be parsed as cookies.')
            send_telegram(
                'I could not parse that message as a cookie string.\n'
                'Send the full value in this format:\n'
                '<code>name=value; name2=value2</code>'
            )
            continue

        print(f'Received {len(candidate_cookies)} cookies from Telegram; validating...')
        try:
            rooms = validate_cookies(candidate_cookies)
        except NotLoginError:
            print('Telegram provided cookie was rejected by Douyu.')
            send_telegram(
                'That cookie did not work. Make sure it comes from a logged-in '
                'Douyu browser session, then send a fresh full cookie string.'
            )
            continue
        except DouyuAPIError as error:
            print(f'Could not validate Telegram cookie due to API error: {error}')
            send_telegram(
                'I received your cookie, but Douyu could not be reached to '
                f'verify it yet: <code>{escape(str(error))}</code>\n'
                'Please send the cookie again after the service recovers.'
            )
            continue

        save_cookies(candidate_cookies)
        send_telegram('New Douyu cookie verified successfully. Monitoring resumed.')
        print('Telegram cookie accepted and saved.')
        return candidate_cookies, rooms


def wait_with_ping_checks(delay_seconds: int, ping_offset: int) -> int:
    """
    Wait until the next Douyu poll while continuing to process /ping commands.

    Args:
        delay_seconds: Total wait duration in seconds
        ping_offset: Current Telegram update offset

    Returns:
        Updated Telegram update offset after processing pending commands
    """
    deadline = time.monotonic() + delay_seconds
    current_offset = ping_offset

    while True:
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            return current_offset

        timeout = min(TELEGRAM_LONG_POLL_TIMEOUT, max(1, int(remaining)))
        current_offset = _process_ping_commands(current_offset, timeout=timeout)


def main():
    """Main loop for polling Douyu live status."""
    print('Douyu Live Status Notifier')
    print('==========================')

    if not TELEGRAM_BOT_TOKEN or not TELEGRAM_CHAT_ID:
        print('Error: TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID env vars required.')
        sys.exit(1)

    cookies = load_cookies()
    initial_rooms: List[Room]

    if not cookies:
        print('No local cookies available. Requesting one through Telegram.')
        try:
            cookies, initial_rooms = recover_cookies_via_telegram('No local cookie found.')
        except TelegramPollingConflict as error:
            print(f'Error: {error}')
            sys.exit(1)
    else:
        try:
            initial_rooms = validate_cookies(cookies)
        except NotLoginError as error:
            try:
                cookies, initial_rooms = recover_cookies_via_telegram(str(error))
            except TelegramPollingConflict as conflict_error:
                print(f'Error: {conflict_error}')
                sys.exit(1)
        except DouyuAPIError as error:
            print(f'Initial validation failed: {error}')
            print('Starting monitor anyway and retrying in the polling loop.')
            initial_rooms = []

    print(f'Found {len(cookies)} cookies')
    print(f'Polling every {POLL_INTERVAL} seconds')
    print('Press Ctrl+C to stop\n')

    previous_live: Optional[Set[str]] = None
    previous_live = notify_new_live(initial_rooms, previous_live)

    ping_offset = 0

    while True:
        try:
            rooms = fetch_douyu_live_status(cookies)
            live_count = sum(1 for r in rooms if r.is_live)
            print(f'[{datetime.now():%H:%M:%S}] Checked: {live_count}/{len(rooms)} live')

            update_health_state(live_count)
            previous_live = notify_new_live(rooms, previous_live)
            ping_offset = wait_with_ping_checks(POLL_INTERVAL, ping_offset)

        except NotLoginError as e:
            print(f'\nAuthentication Error: {e}')
            try:
                cookies, rooms = recover_cookies_via_telegram(str(e))
            except TelegramPollingConflict as error:
                print(f'Error: {error}')
                sys.exit(1)
            live_count = sum(1 for r in rooms if r.is_live)
            print(
                f'[{datetime.now():%H:%M:%S}] Recovered with '
                f'{live_count}/{len(rooms)} live'
            )
            update_health_state(live_count)
            previous_live = notify_new_live(rooms, previous_live)
            ping_offset = wait_with_ping_checks(POLL_INTERVAL, ping_offset)

        except DouyuAPIError as e:
            print(f'\nAPI Error: {e}')
            print('Retrying in 30 seconds...')
            ping_offset = wait_with_ping_checks(30, ping_offset)

        except KeyboardInterrupt:
            print('\n\nStopping...')
            break

        except Exception as e:
            print(f'\nUnexpected error: {e}')
            print('Retrying in 30 seconds...')
            ping_offset = wait_with_ping_checks(30, ping_offset)


if __name__ == '__main__':
    main()
