from __future__ import annotations

import os
import re
from dataclasses import dataclass
from datetime import date, datetime, timedelta, timezone
from pathlib import Path
from typing import Iterable, List, Optional, Tuple

LOG_ENV_VAR = "PYTIMELOG_PATH"


def utc_now() -> datetime:
    """Return current UTC time with seconds precision for consistent storage."""
    return datetime.now(timezone.utc).replace(microsecond=0)


def default_log_path() -> Path:
    """Resolve the log path from env or fallback to ~/.pytimelog/log.txt."""
    env_value = os.environ.get(LOG_ENV_VAR)
    if env_value:
        path = Path(env_value).expanduser()
    else:
        path = Path.home() / ".pytimelog" / "log.txt"

    path.parent.mkdir(parents=True, exist_ok=True)
    return path


@dataclass
class Entry:
    start: datetime
    end: Optional[datetime]
    text: str

    def duration(self, now: Optional[datetime] = None) -> timedelta:
        reference = now or utc_now()
        final = self.end or reference
        return final - self.start

    def tags(self) -> List[str]:
        return [
            word[1:]
            for word in self.text.split()
            if word.startswith("#") and len(word) > 1
        ]


def _ensure_aware(value: datetime) -> datetime:
    if value.tzinfo is None:
        return value.replace(tzinfo=timezone.utc)
    return value


def format_entry(entry: Entry) -> str:
    start = _ensure_aware(entry.start).astimezone(timezone.utc)
    end = entry.end
    end_str = "-" if end is None else _ensure_aware(end).astimezone(
        timezone.utc
    ).isoformat()
    return f"{start.isoformat()} {end_str}|{entry.text.strip()}"


def parse_entry(raw: str) -> Entry:
    if "|" not in raw:
        raise ValueError("Entry must contain '|' separator")
    times_part, text = raw.split("|", 1)
    times = times_part.strip().split()
    if len(times) != 2:
        raise ValueError("Entry must have start and end column")
    start_raw, end_raw = times
    start = _ensure_aware(datetime.fromisoformat(start_raw))
    end = None if end_raw == "-" else _ensure_aware(datetime.fromisoformat(end_raw))
    return Entry(start=start, end=end, text=text.strip())


def read_entries(path: Optional[Path] = None) -> List[Entry]:
    path = path or default_log_path()
    if not path.exists():
        return []
    entries: List[Entry] = []
    for line in path.read_text().splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        entries.append(parse_entry(stripped))
    return entries


def write_entries(entries: Iterable[Entry], path: Optional[Path] = None) -> None:
    path = path or default_log_path()
    path.parent.mkdir(parents=True, exist_ok=True)
    lines = [format_entry(entry) for entry in entries]
    path.write_text("\n".join(lines) + ("\n" if lines else ""))


def append_entry(entry: Entry, path: Optional[Path] = None) -> None:
    path = path or default_log_path()
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8") as handle:
        handle.write(format_entry(entry) + "\n")


def find_open(entries: List[Entry]) -> Optional[int]:
    for idx in reversed(range(len(entries))):
        if entries[idx].end is None:
            return idx
    return None


def check_overlap(entries: List[Entry], candidate: Entry, now: Optional[datetime] = None) -> Optional[Tuple[Entry, timedelta]]:
    """Return overlapping entry and overlap duration if found."""
    now = now or utc_now()
    for existing in entries:
        existing_end = existing.end or now
        candidate_end = candidate.end or now
        if candidate.start < existing_end and candidate_end > existing.start:
            overlap_duration = min(existing_end, candidate_end) - max(candidate.start, existing.start)
            if overlap_duration > timedelta(0):
                return existing, overlap_duration
    return None


def parse_date(value: str) -> date:
    return date.fromisoformat(value)


def parse_time_of_day(value: str) -> Tuple[int, int]:
    match = re.match(r"^(?P<hour>\d{1,2}):(?P<minute>\d{2})$", value)
    if not match:
        raise ValueError(f"Invalid time format: {value}")
    hour = int(match.group("hour"))
    minute = int(match.group("minute"))
    if not 0 <= hour <= 23 or not 0 <= minute <= 59:
        raise ValueError(f"Invalid time value: {value}")
    return hour, minute
