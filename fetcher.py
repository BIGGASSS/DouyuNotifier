import requests
from typing import List, Dict, Any
from models import Room, NotLoginError, DouyuAPIError
from config import API_URL


def fetch_douyu_live_status(cookies: Dict[str, str]) -> List[Room]:
    """
    Fetch Douyu live status from the follow list API.
    
    Args:
        cookies: Dictionary of cookies for authentication
        
    Returns:
        List of Room objects
        
    Raises:
        NotLoginError: If user is not logged in
        DouyuAPIError: For other API errors
    """
    cookie_str = "; ".join(f"{k}={v}" for k, v in cookies.items())
    headers = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
        "Accept": "application/json, text/plain, */*",
        "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
        "Referer": "https://www.douyu.com/",
        "Cookie": cookie_str
    }

    try:
        response = requests.get(
            API_URL,
            headers=headers,
            timeout=30
        )
        response.raise_for_status()
        data = response.json()

        # Check for login status
        msg = data.get("msg") or ""
        error = data.get("error")
        if str(error) == "-1" and "未登" in msg:
            raise NotLoginError(msg)

        if str(error) != "0":
            raise DouyuAPIError(f"API error: {msg or data}")
        
        return parse_response(data)
        
    except requests.exceptions.RequestException as e:
        raise DouyuAPIError(f"Request failed: {e}")


def parse_response(data: Dict[str, Any]) -> List[Room]:
    """Parse Douyu API response into Room objects."""
    rooms = []
    room_list = data.get("data", {}).get("list", [])
    
    for room_data in room_list:
        room_id = str(room_data.get("room_id", ""))
        room_name = room_data.get("room_name", "")
        streamer_name = room_data.get("nickname", "")
        cover = room_data.get("room_src", "")
        avatar = room_data.get("avatar_small", "")
        show_status = room_data.get("show_status", 0)
        video_loop = room_data.get("videoLoop", 0)
        area_name = room_data.get("game_name", "")
        path = room_data.get("url", f"/{room_id}")
        
        # is_live: true if show_status == 1 AND videoLoop == 0
        is_live = (show_status == 1 and video_loop == 0)
        
        room = Room(
            room_id=f"dy_{room_id}",
            room_name=room_name,
            streamer_name=streamer_name,
            cover=cover,
            avatar=avatar,
            is_live=is_live,
            area_name=area_name,
            url=f"https://www.douyu.com{path}"
        )
        rooms.append(room)
    
    return rooms
