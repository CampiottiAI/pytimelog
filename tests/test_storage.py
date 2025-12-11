from datetime import datetime, timezone
import unittest

from pytimelog.storage import (
    Entry,
    check_overlap,
    format_entry,
    parse_entry,
    parse_time_of_day,
)


class StorageTests(unittest.TestCase):
    def test_format_and_parse_round_trip(self) -> None:
        entry = Entry(
            start=datetime(2024, 1, 1, 12, 0, tzinfo=timezone.utc),
            end=datetime(2024, 1, 1, 13, 30, tzinfo=timezone.utc),
            text="Write docs #project",
        )
        raw = format_entry(entry)
        parsed = parse_entry(raw)
        self.assertEqual(parsed.start, entry.start)
        self.assertEqual(parsed.end, entry.end)
        self.assertEqual(parsed.text, entry.text)

    def test_overlap_detection(self) -> None:
        first = Entry(
            start=datetime(2024, 1, 1, 9, 0, tzinfo=timezone.utc),
            end=datetime(2024, 1, 1, 10, 0, tzinfo=timezone.utc),
            text="Morning work",
        )
        overlapping = Entry(
            start=datetime(2024, 1, 1, 9, 30, tzinfo=timezone.utc),
            end=datetime(2024, 1, 1, 9, 45, tzinfo=timezone.utc),
            text="Conflicts",
        )
        self.assertIsNotNone(check_overlap([first], overlapping, now=first.end))

    def test_parse_time_of_day(self) -> None:
        self.assertEqual(parse_time_of_day("09:05"), (9, 5))
        with self.assertRaises(ValueError):
            parse_time_of_day("99:99")


if __name__ == "__main__":
    unittest.main()
