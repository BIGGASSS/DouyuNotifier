import unittest
from unittest.mock import patch

from main import validate_cookies
from models import DouyuAPIError, NotLoginError, Room


def build_room() -> Room:
    return Room(
        room_id='dy_1',
        room_name='Room',
        streamer_name='Streamer',
        cover='',
        avatar='',
        is_live=True,
        area_name='Game',
        url='https://www.douyu.com/1',
    )


class ValidateCookiesTest(unittest.TestCase):
    @patch('main.time.sleep', return_value=None)
    @patch('main.fetch_douyu_live_status')
    def test_retries_transient_api_error(
        self,
        fetch_douyu_live_status_mock,
        sleep_mock,
    ) -> None:
        fetch_douyu_live_status_mock.side_effect = [
            DouyuAPIError('temporary'),
            [build_room()],
        ]

        rooms = validate_cookies({'acf_uid': '123'})

        self.assertEqual(rooms, [build_room()])
        self.assertEqual(fetch_douyu_live_status_mock.call_count, 2)
        sleep_mock.assert_called_once()

    @patch('main.fetch_douyu_live_status', side_effect=NotLoginError('expired'))
    def test_does_not_retry_invalid_cookie(self, fetch_douyu_live_status_mock) -> None:
        with self.assertRaises(NotLoginError):
            validate_cookies({'acf_uid': '123'})

        self.assertEqual(fetch_douyu_live_status_mock.call_count, 1)


if __name__ == '__main__':
    unittest.main()
