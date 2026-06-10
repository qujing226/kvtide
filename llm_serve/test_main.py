import unittest

import main
from request_state import request_state


class MockTimingTest(unittest.TestCase):
    def test_prefill_latency_scales_with_scheduled_tokens(self):
        self.assertEqual(main.prefill_latency_ms(1, 30, 300), 330)
        self.assertEqual(main.prefill_latency_ms(16, 50, 500), 1300)
        self.assertEqual(main.prefill_latency_ms(64, 70, 800), 5280)

    def test_prefix_cache_reduces_latency_by_skipping_cached_tokens(self):
        cold_latency_ms = main.prefill_latency_ms(32, 50, 500)
        cached_tail_latency_ms = main.prefill_latency_ms(4, 50, 500)

        self.assertEqual(cold_latency_ms, 2100)
        self.assertEqual(cached_tail_latency_ms, 700)

    def test_decode_latency_stays_between_seventy_and_one_hundred_thirty_ms(self):
        self.assertEqual(main.decode_latency_ms(0, 0, 70), 70)
        self.assertEqual(main.decode_latency_ms(1024, 32, 130), 130)


class MockResponseTest(unittest.TestCase):
    def test_markdown_response_is_complete_within_default_output_budget(self):
        chunks = request_state.MOCK_RESPONSE_CHUNKS

        self.assertLessEqual(len(chunks), 128)
        self.assertEqual("".join(chunks), request_state.MOCK_RESPONSE_TEXT)
        self.assertIn("# Paged Attention", request_state.MOCK_RESPONSE_TEXT)
        self.assertIn("\n\n", request_state.MOCK_RESPONSE_TEXT)


if __name__ == "__main__":
    unittest.main()
