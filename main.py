#!/usr/bin/env python3
"""
Douyu Live Status Notifier

Fetches live status of followed streamers from Douyu TV
and sends Telegram notifications when a stream starts.
"""

import time
import sys
from datetime import datetime
from typing import Set, Optional

from config import POLL_INTERVAL, TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID
from auth import get_douyu_cookies
from fetcher import fetch_douyu_live_status
from notifier import notify_new_live
from models import NotLoginError, DouyuAPIError


def main():
    """Main loop for polling Douyu live status."""
    print("Douyu Live Status Notifier")
    print("==========================")

    if not TELEGRAM_BOT_TOKEN or not TELEGRAM_CHAT_ID:
        print("Error: TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID env vars required.")
        sys.exit(1)

    cookies = get_douyu_cookies()

    if not cookies:
        print("Error: No cookies available.")
        print("Please make sure you are logged into Douyu in your browser.")
        sys.exit(1)

    print(f"Found {len(cookies)} cookies")
    print(f"Polling every {POLL_INTERVAL} seconds")
    print("Press Ctrl+C to stop\n")

    previous_live: Optional[Set[str]] = None

    while True:
        try:
            rooms = fetch_douyu_live_status(cookies)
            live_count = sum(1 for r in rooms if r.is_live)
            print(f"[{datetime.now():%H:%M:%S}] Checked: {live_count}/{len(rooms)} live")

            previous_live = notify_new_live(rooms, previous_live)

            time.sleep(POLL_INTERVAL)

        except NotLoginError as e:
            print(f"\nAuthentication Error: {e}")
            print("Please log into Douyu in your browser and try again.")
            sys.exit(1)

        except DouyuAPIError as e:
            print(f"\nAPI Error: {e}")
            print("Retrying in 30 seconds...")
            time.sleep(30)

        except KeyboardInterrupt:
            print("\n\nStopping...")
            break

        except Exception as e:
            print(f"\nUnexpected error: {e}")
            print("Retrying in 30 seconds...")
            time.sleep(30)


if __name__ == "__main__":
    main()
