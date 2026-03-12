import unittest

from auth import parse_cookie_string


class ParseCookieStringTest(unittest.TestCase):
    def test_parses_plain_cookie_string(self) -> None:
        cookies = parse_cookie_string('acf_uid=123; dy_did=abc; malformed')

        self.assertEqual(
            cookies,
            {
                'acf_uid': '123',
                'dy_did': 'abc',
            },
        )

    def test_parses_telegram_code_block_cookie_string(self) -> None:
        cookies = parse_cookie_string('```\nCookie: acf_uid=123; dy_did=abc\n```')

        self.assertEqual(
            cookies,
            {
                'acf_uid': '123',
                'dy_did': 'abc',
            },
        )


if __name__ == '__main__':
    unittest.main()
