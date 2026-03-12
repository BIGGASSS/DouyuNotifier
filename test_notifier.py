import unittest
from unittest.mock import Mock, patch

from models import TelegramPollingConflict
from notifier import get_telegram_updates


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


if __name__ == '__main__':
    unittest.main()
