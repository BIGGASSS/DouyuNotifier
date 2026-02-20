import os
import json
from typing import Dict

COOKIES_FILE = "cookies.json"


def get_douyu_cookies() -> Dict[str, str]:
    """
    Get Douyu cookies:
    1. Try to load from cookies.json file
    2. Prompt user for manual input
    """
    # Method 1: Load from file
    cookies = _load_from_file()
    if cookies:
        print(f"✓ Loaded cookies from {COOKIES_FILE}")
        return cookies

    # Method 2: Manual input
    print("\nNo saved cookies found.")
    print("Please log into Douyu in your browser and copy the cookies.")
    print("You can find cookies in DevTools > Application/Storage > Cookies")
    cookies = _manual_cookie_input()

    # Save for future use
    if cookies:
        _save_to_file(cookies)

    return cookies


def _load_from_file() -> Dict[str, str]:
    """Load cookies from cookies.json file."""
    try:
        if os.path.exists(COOKIES_FILE):
            with open(COOKIES_FILE, 'r', encoding='utf-8') as f:
                return json.load(f)
    except Exception as e:
        print(f"Warning: Could not load cookies file: {e}")
    return {}


def _save_to_file(cookies: Dict[str, str]):
    """Save cookies to cookies.json file."""
    try:
        with open(COOKIES_FILE, 'w', encoding='utf-8') as f:
            json.dump(cookies, f, indent=2)
        print(f"✓ Saved cookies to {COOKIES_FILE}")
    except Exception as e:
        print(f"Warning: Could not save cookies file: {e}")


def _manual_cookie_input() -> Dict[str, str]:
    """Prompt user for manual cookie input."""
    print("\nEnter cookies (format: name1=value1; name2=value2)")
    print("Or paste individual cookie names and values:")

    cookies = {}

    # Option 1: Paste as string
    cookie_str = input("\nCookie string (or press Enter for individual input): ").strip()

    if cookie_str:
        # Parse cookie string
        for pair in cookie_str.split(';'):
            pair = pair.strip()
            if '=' in pair:
                name, value = pair.split('=', 1)
                cookies[name.strip()] = value.strip()
    else:
        # Option 2: Individual input
        print("\nEnter cookie name and value (empty name to finish):")
        while True:
            name = input("Cookie name: ").strip()
            if not name:
                break
            value = input("Cookie value: ").strip()
            if value:
                cookies[name] = value

    return cookies
