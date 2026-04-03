import unittest
from unittest.mock import call, patch

from main import validate_cookies, wait_with_ping_checks
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


class WaitWithPingChecksTest(unittest.TestCase):
    @patch('main._process_ping_commands')
    @patch('main.time.monotonic')
    def test_wait_with_ping_checks_uses_ping_processing_until_deadline(
        self,
        monotonic_mock,
        process_ping_commands_mock,
    ) -> None:
        monotonic_mock.side_effect = [100.0, 100.0, 104.2]
        process_ping_commands_mock.return_value = 55

        offset = wait_with_ping_checks(4, 12)

        self.assertEqual(offset, 55)
        process_ping_commands_mock.assert_called_once_with(12, timeout=4)

    @patch('main._process_ping_commands')
    @patch('main.time.monotonic')
    def test_wait_with_ping_checks_splits_long_waits_by_long_poll_timeout(
        self,
        monotonic_mock,
        process_ping_commands_mock,
    ) -> None:
        monotonic_mock.side_effect = [100.0, 100.0, 130.0, 171.0]
        process_ping_commands_mock.side_effect = [20, 21]

        offset = wait_with_ping_checks(70, 19)

        self.assertEqual(offset, 21)
        self.assertEqual(
            process_ping_commands_mock.call_args_list,
            [call(19, timeout=60), call(20, timeout=40)],
        )


if __name__ == '__main__':
    unittest.main()
