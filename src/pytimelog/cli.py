from __future__ import annotations

import argparse
import sys
from datetime import datetime, time, timedelta, timezone
from typing import Iterable, List, Optional, Tuple

from .storage import (
    Entry,
    append_entry,
    check_overlap,
    find_open,
    parse_date,
    parse_time_of_day,
    read_entries,
    utc_now,
    write_entries,
)


def format_duration(delta: timedelta) -> str:
    total_seconds = int(delta.total_seconds())
    hours, remainder = divmod(total_seconds, 3600)
    minutes, _ = divmod(remainder, 60)
    return f"{hours}h{minutes:02d}m"


def local_now() -> datetime:
    return datetime.now().astimezone().replace(microsecond=0)


def parse_when(value: Optional[str], fallback: datetime) -> datetime:
    if value is None:
        return fallback

    # ISO string with optional timezone
    try:
        parsed = datetime.fromisoformat(value)
        if parsed.tzinfo is None:
            parsed = parsed.replace(tzinfo=fallback.tzinfo)
        return parsed
    except ValueError:
        pass

    # HH:MM for today in local timezone
    try:
        hour, minute = parse_time_of_day(value)
    except ValueError:
        raise SystemExit(f"Cannot parse time: {value}") from None

    today = fallback.date()
    return datetime.combine(today, time(hour=hour, minute=minute, tzinfo=fallback.tzinfo))


def to_utc(value: datetime) -> datetime:
    if value.tzinfo is None:
        value = value.replace(tzinfo=timezone.utc)
    return value.astimezone(timezone.utc)


def require_no_open(entries: List[Entry]) -> None:
    open_index = find_open(entries)
    if open_index is not None:
        raise SystemExit("There is already an active entry. Stop it before starting another.")


def command_start(args: argparse.Namespace) -> None:
    entries = read_entries()
    require_no_open(entries)
    when = to_utc(parse_when(args.at, local_now()))
    new_entry = Entry(start=when, end=None, text=args.text)
    append_entry(new_entry)
    print(f"Started: {args.text} @ {when.astimezone().strftime('%Y-%m-%d %H:%M')}")


def command_stop(args: argparse.Namespace) -> None:
    entries = read_entries()
    open_index = find_open(entries)
    if open_index is None:
        raise SystemExit("No active entry to stop.")
    when = to_utc(parse_when(args.at, local_now()))

    open_entry = entries[open_index]
    if when <= open_entry.start:
        raise SystemExit("Stop time must be after the start time.")

    updated = Entry(start=open_entry.start, end=when, text=open_entry.text)
    entries[open_index] = updated
    write_entries(entries)
    elapsed = updated.duration()
    print(f"Stopped '{updated.text}' after {format_duration(elapsed)}.")


def command_add(args: argparse.Namespace) -> None:
    entries = read_entries()
    start = to_utc(parse_when(args.start, local_now()))
    end = to_utc(parse_when(args.end, local_now()))
    if end <= start:
        raise SystemExit("End time must be after start time.")

    new_entry = Entry(start=start, end=end, text=args.text)
    overlap = check_overlap(entries, new_entry, now=end)
    if overlap:
        other, duration = overlap
        local_other_start = other.start.astimezone()
        raise SystemExit(
            f"New entry overlaps with existing entry starting at {local_other_start:%Y-%m-%d %H:%M} "
            f"for {format_duration(duration)}."
        )

    append_entry(new_entry)
    print(
        f"Added {format_duration(new_entry.duration())} entry "
        f"{start.astimezone():%Y-%m-%d %H:%M} -> {end.astimezone():%H:%M} : {new_entry.text}"
    )


def command_status(_: argparse.Namespace) -> None:
    entries = read_entries()
    open_index = find_open(entries)
    if open_index is None:
        print("No active entry.")
        return
    entry = entries[open_index]
    elapsed = entry.duration()
    print(
        f"Active: {entry.text} "
        f"(since {entry.start.astimezone():%H:%M}, {format_duration(elapsed)} elapsed)"
    )


def clamp_duration(entry: Entry, start: datetime, end: datetime, now: datetime) -> timedelta:
    entry_end = entry.end or now
    latest_start = max(entry.start, start)
    earliest_end = min(entry_end, end)
    if earliest_end <= latest_start:
        return timedelta(0)
    return earliest_end - latest_start


def summarize(entries: Iterable[Entry], start: datetime, end: datetime, now: datetime) -> Tuple[timedelta, dict]:
    tag_totals: dict[str, timedelta] = {}
    total = timedelta(0)
    for entry in entries:
        chunk = clamp_duration(entry, start, end, now)
        if chunk <= timedelta(0):
            continue
        total += chunk
        tags = entry.tags() or ["(untagged)"]
        for tag in tags:
            tag_totals[tag] = tag_totals.get(tag, timedelta(0)) + chunk
    return total, tag_totals


def command_report(args: argparse.Namespace) -> None:
    entries = read_entries()
    tz = local_now().tzinfo
    today = local_now().date()

    from_date = parse_date(args.from_date) if args.from_date else today
    to_date = parse_date(args.to_date) if args.to_date else from_date
    if to_date < from_date:
        raise SystemExit("Report end date cannot be before start date.")

    start_local = datetime.combine(from_date, time.min, tzinfo=tz)
    end_local = datetime.combine(to_date + timedelta(days=1), time.min, tzinfo=tz)
    start_utc = start_local.astimezone(timezone.utc)
    end_utc = end_local.astimezone(timezone.utc)
    now = utc_now()

    total, tag_totals = summarize(entries, start_utc, end_utc, now)
    if total == timedelta(0):
        print("No entries in the selected range.")
        return

    print(f"Report {from_date.isoformat()} to {to_date.isoformat()}")
    for tag, duration in sorted(tag_totals.items(), key=lambda item: item[0].lower()):
        print(f"- {tag}: {format_duration(duration)}")
    print(f"Total: {format_duration(total)}")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Tiny time logger inspired by gtimelog.",
    )
    subparsers = parser.add_subparsers(dest="command", required=True)

    start_parser = subparsers.add_parser("start", help="Start a new active entry.")
    start_parser.add_argument("text", help="Description for the entry.")
    start_parser.add_argument(
        "--at",
        help="Start time (ISO or HH:MM in local time). Defaults to now.",
    )
    start_parser.set_defaults(func=command_start)

    stop_parser = subparsers.add_parser("stop", help="Stop the active entry.")
    stop_parser.add_argument(
        "--at",
        help="Stop time (ISO or HH:MM in local time). Defaults to now.",
    )
    stop_parser.set_defaults(func=command_stop)

    add_parser = subparsers.add_parser("add", help="Insert a finished entry.")
    add_parser.add_argument("--start", required=True, help="Start time (ISO or HH:MM).")
    add_parser.add_argument("--end", required=True, help="End time (ISO or HH:MM).")
    add_parser.add_argument("text", help="Description for the entry.")
    add_parser.set_defaults(func=command_add)

    status_parser = subparsers.add_parser("status", help="Show active entry.")
    status_parser.set_defaults(func=command_status)

    report_parser = subparsers.add_parser(
        "report",
        help="Summarize logged time by tag for a date or date range.",
    )
    report_parser.add_argument(
        "--from",
        dest="from_date",
        help="Start date (YYYY-MM-DD). Defaults to today.",
    )
    report_parser.add_argument(
        "--to",
        dest="to_date",
        help="End date (YYYY-MM-DD). Defaults to start date.",
    )
    report_parser.set_defaults(func=command_report)

    return parser


def main(argv: Optional[List[str]] = None) -> None:
    parser = build_parser()
    args = parser.parse_args(argv)
    args.func(args)


if __name__ == "__main__":
    main(sys.argv[1:])
