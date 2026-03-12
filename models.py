from dataclasses import dataclass


@dataclass
class Room:
    room_id: str
    room_name: str
    streamer_name: str
    cover: str
    avatar: str
    is_live: bool
    area_name: str
    url: str
    platform: str = "douyu"

    def __str__(self):
        status = "LIVE" if self.is_live else "OFFLINE"
        return f"[{status}] {self.streamer_name}: {self.room_name} ({self.area_name})"


class NotLoginError(Exception):
    pass


class DouyuAPIError(Exception):
    pass


class TelegramPollingConflict(Exception):
    pass
