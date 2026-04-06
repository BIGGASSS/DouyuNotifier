import unittest
from unittest.mock import Mock, patch, call

from models import Room, TelegramPollingConflict
from notifier import get_telegram_updates, notify_stream_end


class TelegramPollingTest(unittest.TestCase):
    @patch('notifier.prepare_telegram_updates', return_value=None)
    @patch('notifier.requests.get')
    def test_get_updates_raises_clear_conflict_on_409(
        self,
        requests_get_mock,
        prepare_telegram_updates_mock,
    ) -> None:
        del prepare_telegram_updates_mock

        response = Mock()
        response.status_code = 409
        response.ok = False
        response.json.return_value = {
            'ok': False,
            'description': (
                'Conflict: terminated by other getUpdates request; '
                'make sure that only one bot instance is running'
            ),
        }
        requests_get_mock.return_value = response

        with self.assertRaises(TelegramPollingConflict) as context:
            get_telegram_updates()

        self.assertIn('already being consumed', str(context.exception))


class NotifyStreamEndTest(unittest.TestCase):
    def _make_room(self, room_id: str, streamer: str, is_live: bool) -> Room:
        return Room(
            room_id=room_id,
            room_name=f"{streamer}'s room",
            streamer_name=streamer,
            cover="",
            avatar="",
            is_live=is_live,
            area_name="Gaming",
            url=f"https://www.douyu.com/{room_id}",
        )

    @patch('notifier.send_telegram')
    def test_no_notification_on_first_run(self, send_mock) -> None:
        rooms = [self._make_room("dy_1", "Alice", is_live=True)]
        result = notify_stream_end(rooms, None)
        self.assertEqual(result, {"dy_1"})
        send_mock.assert_not_called()

    @patch('notifier.send_telegram')
    def test_no_notification_when_still_live(self, send_mock) -> None:
        rooms = [self._make_room("dy_1", "Alice", is_live=True)]
        result = notify_stream_end(rooms, {"dy_1"})
        self.assertEqual(result, {"dy_1"})
        send_mock.assert_not_called()

    @patch('notifier.send_telegram')
    def test_notifies_when_streamer_goes_offline(self, send_mock) -> None:
        rooms = [self._make_room("dy_1", "Alice", is_live=False)]
        result = notify_stream_end(rooms, {"dy_1"})
        self.assertEqual(result, set())
        send_mock.assert_called_once_with(
            '<b>Alice</b> has ended their stream.'
        )

    @patch('notifier.send_telegram')
    def test_notifies_multiple_streamers_end(self, send_mock) -> None:
        rooms = [
            self._make_room("dy_1", "Alice", is_live=False),
            self._make_room("dy_2", "Bob", is_live=False),
        ]
        result = notify_stream_end(rooms, {"dy_1", "dy_2"})
        self.assertEqual(result, set())
        self.assertEqual(send_mock.call_count, 2)

    @patch('notifier.send_telegram')
    def test_no_notification_for_newly_live(self, send_mock) -> None:
        rooms = [self._make_room("dy_1", "Alice", is_live=True)]
        result = notify_stream_end(rooms, set())
        self.assertEqual(result, {"dy_1"})
        send_mock.assert_not_called()

    @patch('notifier.send_telegram')
    def test_mixed_live_and_ended(self, send_mock) -> None:
        rooms = [
            self._make_room("dy_1", "Alice", is_live=False),
            self._make_room("dy_2", "Bob", is_live=True),
        ]
        result = notify_stream_end(rooms, {"dy_1", "dy_2"})
        self.assertEqual(result, {"dy_2"})
        send_mock.assert_called_once_with(
            '<b>Alice</b> has ended their stream.'
        )


if __name__ == '__main__':
    unittest.main()
