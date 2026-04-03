import unittest
from unittest.mock import patch

from notifier import _handle_ping_command, _process_ping_commands, update_health_state


class PingCommandTest(unittest.TestCase):
    @patch('notifier._last_poll_time', None)
    @patch('notifier._live_streamer_count', 0)
    @patch('notifier.send_telegram')
    @patch('notifier._START_TIME', 1000.0)
    @patch('notifier.time.time', return_value=1365.0)
    def test_ping_shows_health_status(self, mock_time, mock_send):
        """Ping response includes uptime, last poll, and live count."""
        update_health_state(3)
        _handle_ping_command()

        mock_send.assert_called_once()
        text = mock_send.call_args[0][0]
        self.assertIn('<b>Pong!</b>', text)
        self.assertIn('Uptime: 6m 5s', text)
        self.assertIn('Live streamers: 3', text)

    @patch('notifier.send_telegram')
    @patch('notifier._START_TIME', 1000.0)
    @patch('notifier.time.time', return_value=7261.0)
    def test_ping_shows_hours_in_uptime(self, mock_time, mock_send):
        """Uptime over an hour shows hours format."""
        with patch('notifier._last_poll_time', 7200.0):
            with patch('notifier._live_streamer_count', 5):
                _handle_ping_command()

        text = mock_send.call_args[0][0]
        self.assertIn('Uptime: 1h 44m 21s', text)
        self.assertIn('Last poll: 1m ago', text)
        self.assertIn('Live streamers: 5', text)

    @patch('notifier.send_telegram')
    @patch('notifier.time.time', return_value=1100.0)
    def test_ping_last_poll_recent(self, mock_time, mock_send):
        """Last poll shows seconds ago for recent polls."""
        with patch('notifier._last_poll_time', 1090.0):
            with patch('notifier._live_streamer_count', 0):
                with patch('notifier._START_TIME', 1000.0):
                    _handle_ping_command()

        text = mock_send.call_args[0][0]
        self.assertIn('Last poll: 10s ago', text)


class UpdateHealthStateTest(unittest.TestCase):
    @patch('notifier._last_poll_time', None)
    @patch('notifier._live_streamer_count', 0)
    @patch('notifier.time.time', return_value=12345.0)
    def test_update_health_state_sets_values(self, mock_time):
        update_health_state(7)
        from notifier import _last_poll_time, _live_streamer_count
        self.assertEqual(_last_poll_time, 12345.0)
        self.assertEqual(_live_streamer_count, 7)


class ProcessPingCommandsTest(unittest.TestCase):
    @patch('notifier.get_telegram_updates', return_value=([], 42))
    def test_process_ping_commands_passes_timeout(self, get_telegram_updates_mock):
        offset = _process_ping_commands(41, timeout=17)

        self.assertEqual(offset, 42)
        get_telegram_updates_mock.assert_called_once_with(offset=41, timeout=17)


if __name__ == '__main__':
    unittest.main()
