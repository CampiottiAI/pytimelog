from __future__ import annotations

import curses
import textwrap
from datetime import datetime, time, timedelta, timezone
from typing import List, Tuple

from .storage import Entry, append_entry, find_open, read_entries, write_entries, utc_now


def local_now() -> datetime:
    return datetime.now().astimezone().replace(microsecond=0)


def to_utc(value: datetime) -> datetime:
    if value.tzinfo is None:
        value = value.replace(tzinfo=timezone.utc)
    return value.astimezone(timezone.utc)


def format_duration(delta: timedelta) -> str:
    total_seconds = int(delta.total_seconds())
    hours, remainder = divmod(total_seconds, 3600)
    minutes, _ = divmod(remainder, 60)
    return f"{hours:02d}:{minutes:02d}"


def clamp_duration(entry: Entry, start: datetime, end: datetime, now: datetime) -> timedelta:
    entry_end = entry.end or now
    latest_start = max(entry.start, start)
    earliest_end = min(entry_end, end)
    if earliest_end <= latest_start:
        return timedelta(0)
    return earliest_end - latest_start


class TerminalUI:
    def __init__(self, stdscr: curses.window) -> None:
        self.stdscr = stdscr
        self.entries: List[Entry] = []
        self.selected = 0
        self.range_mode = "today"  # or "week"
        self.message = ""
        self.message_error = False

        curses.curs_set(0)
        curses.start_color()
        curses.use_default_colors()
        curses.init_pair(1, curses.COLOR_BLACK, curses.COLOR_CYAN)  # selection
        curses.init_pair(2, curses.COLOR_RED, -1)  # errors
        curses.init_pair(3, curses.COLOR_GREEN, -1)  # success/info
        self.timeout_ms = 1000
        self.stdscr.timeout(self.timeout_ms)
        self.reload_entries()

    def reload_entries(self) -> None:
        self.entries = read_entries()
        visible = self.visible_entries(utc_now())
        if visible:
            self.selected = min(self.selected, len(visible) - 1)
        else:
            self.selected = 0

    def visible_entries(self, now: datetime) -> List[Entry]:
        start, end = self.window_for_mode(now)
        return [entry for entry in self.entries if (entry.end or now) > start and entry.start < end]

    def window_for_mode(self, now: datetime) -> Tuple[datetime, datetime]:
        tzinfo = now.astimezone().tzinfo
        today = now.astimezone().date()
        if self.range_mode == "week":
            weekday = today.weekday()
            week_start = today - timedelta(days=weekday)
            week_end = week_start + timedelta(days=7)
            start_local = datetime.combine(week_start, time.min, tzinfo=tzinfo)
            end_local = datetime.combine(week_end, time.min, tzinfo=tzinfo)
        else:
            start_local = datetime.combine(today, time.min, tzinfo=tzinfo)
            end_local = datetime.combine(today + timedelta(days=1), time.min, tzinfo=tzinfo)
        return start_local.astimezone(timezone.utc), end_local.astimezone(timezone.utc)

    def draw(self) -> None:
        self.stdscr.erase()
        now = utc_now()
        visible = self.visible_entries(now)
        height, width = self.stdscr.getmaxyx()

        self.draw_header(now, width)
        self.draw_summary(now, width)
        self.draw_entries(visible, now, height, width)
        self.draw_detail(visible, now, height, width)
        self.draw_footer(width)
        self.stdscr.refresh()

    def draw_header(self, now: datetime, width: int) -> None:
        idx = find_open(self.entries)
        if idx is None:
            status = "Status: idle"
            color = curses.color_pair(3)
        else:
            entry = self.entries[idx]
            elapsed = format_duration(entry.duration(now))
            status = f"Status: running '{entry.text}' ({elapsed})"
            color = curses.color_pair(1)
        self.addstr(0, 0, f"pytimelog  [{self.range_mode.upper()} VIEW]", color, width)
        self.addstr(1, 0, status, color, width)

    def draw_summary(self, now: datetime, width: int) -> None:
        today_start, today_end = self.window_for_mode(now) if self.range_mode == "today" else self.day_window(now)
        week_start, week_end = self.week_window(now)
        today_total = self.total_for_range(today_start, today_end, now)
        week_total = self.total_for_range(week_start, week_end, now)
        remaining_today = max(timedelta(hours=8) - today_total, timedelta(0))
        remaining_week = max(timedelta(hours=40) - week_total, timedelta(0))
        line = (
            f"Today {format_duration(today_total)} / 08:00 "
            f"(remaining {format_duration(remaining_today)})   "
            f"Week {format_duration(week_total)} / 40:00 "
            f"(remaining {format_duration(remaining_week)})"
        )
        self.addstr(2, 0, line, curses.A_BOLD, width)

    def day_window(self, now: datetime) -> Tuple[datetime, datetime]:
        tzinfo = now.astimezone().tzinfo
        today = now.astimezone().date()
        start_local = datetime.combine(today, time.min, tzinfo=tzinfo)
        end_local = datetime.combine(today + timedelta(days=1), time.min, tzinfo=tzinfo)
        return start_local.astimezone(timezone.utc), end_local.astimezone(timezone.utc)

    def week_window(self, now: datetime) -> Tuple[datetime, datetime]:
        tzinfo = now.astimezone().tzinfo
        today = now.astimezone().date()
        weekday = today.weekday()
        week_start = today - timedelta(days=weekday)
        week_end = week_start + timedelta(days=7)
        start_local = datetime.combine(week_start, time.min, tzinfo=tzinfo)
        end_local = datetime.combine(week_end, time.min, tzinfo=tzinfo)
        return start_local.astimezone(timezone.utc), end_local.astimezone(timezone.utc)

    def total_for_range(self, start_utc: datetime, end_utc: datetime, now: datetime) -> timedelta:
        total = timedelta(0)
        for entry in self.entries:
            total += clamp_duration(entry, start_utc, end_utc, now)
        return total

    def draw_entries(self, visible: List[Entry], now: datetime, height: int, width: int) -> None:
        list_top = 4
        list_height = max(height - 6, 3)
        list_width = max(int(width * 0.6), 20)
        tzinfo = now.astimezone().tzinfo
        header = "Entries (current view)".ljust(list_width - 1)
        self.addstr(list_top - 1, 0, header, curses.A_UNDERLINE, list_width)

        start_index = max(0, self.selected - list_height + 1)
        for idx, entry in enumerate(visible[start_index : start_index + list_height]):
            actual_index = start_index + idx
            line = self.format_entry_line(entry, now, tzinfo)
            attr = curses.color_pair(1) if actual_index == self.selected else curses.A_NORMAL
            self.addstr(list_top + idx, 0, line, attr, list_width)

        if not visible:
            self.addstr(list_top, 0, "No entries in this view.", curses.A_DIM, list_width)

    def format_entry_line(self, entry: Entry, now: datetime, tzinfo) -> str:
        start_local = entry.start.astimezone(tzinfo)
        end_local = (entry.end or now).astimezone(tzinfo)
        duration = format_duration(entry.duration(now))
        end_label = end_local.strftime("%H:%M") if entry.end else "…"
        return f"{start_local:%H:%M}-{end_label} {duration} {entry.text}"

    def draw_detail(self, visible: List[Entry], now: datetime, height: int, width: int) -> None:
        detail_left = max(int(width * 0.6) + 1, 22)
        detail_width = max(width - detail_left - 1, 20)
        self.addstr(3, detail_left, "Details", curses.A_UNDERLINE, detail_width)
        if not visible:
            return
        entry = visible[self.selected]
        tzinfo = now.astimezone().tzinfo
        start_local = entry.start.astimezone(tzinfo).strftime("%Y-%m-%d %H:%M")
        end_local = (entry.end or now).astimezone(tzinfo).strftime("%Y-%m-%d %H:%M") if entry.end else "Running"
        duration = format_duration(entry.duration(now))
        tags = ", ".join(entry.tags()) or "(untagged)"

        lines = [
            f"Start   {start_local}",
            f"End     {end_local}",
            f"Length  {duration}",
            f"Tags    {tags}",
            "",
            "Text:",
        ]
        for offset, line in enumerate(lines):
            self.addstr(4 + offset, detail_left, line, curses.A_NORMAL, detail_width)

        wrapped = textwrap.wrap(entry.text, width=detail_width - 1)
        for idx, segment in enumerate(wrapped):
            if 4 + len(lines) + idx >= height - 3:
                break
            self.addstr(4 + len(lines) + idx, detail_left, segment, curses.A_NORMAL, detail_width)

    def draw_footer(self, width: int) -> None:
        height, _ = self.stdscr.getmaxyx()
        help_line = "[↑/k ↓/j] move  [n] start  [x] stop  [r] reload  [v] toggle view  [q] quit"
        message_attr = curses.color_pair(2) if self.message_error else curses.color_pair(3)
        self.addstr(height - 2, 0, self.message, message_attr, width)
        self.addstr(height - 1, 0, help_line, curses.A_DIM, width)

    def addstr(self, y: int, x: int, text: str, attr: int, width: int) -> None:
        height, total_width = self.stdscr.getmaxyx()
        if y < 0 or y >= height:
            return
        capped_width = min(width, total_width - x)
        if capped_width <= 0:
            return
        try:
            self.stdscr.addstr(y, x, text.ljust(capped_width)[:capped_width], attr)
        except curses.error:
            # Ignore drawing errors on very small terminals/resizes.
            pass

    def move_selection(self, delta: int, now: datetime) -> None:
        visible = self.visible_entries(now)
        if not visible:
            self.selected = 0
            return
        self.selected = max(0, min(self.selected + delta, len(visible) - 1))

    def prompt(self, prompt_text: str) -> tuple[str, bool]:
        """Prompt for input; returns (text, cancelled). Esc cancels."""
        height, width = self.stdscr.getmaxyx()
        buffer: List[str] = []
        curses.curs_set(1)
        self.stdscr.timeout(-1)  # block while typing so we don't auto-cancel
        try:
            while True:
                line = f"{prompt_text}{''.join(buffer)}"
                self.addstr(height - 2, 0, line, curses.A_BOLD, width)
                self.stdscr.move(height - 2, min(len(line), width - 1))
                ch = self.stdscr.getch()
                if ch in (27,):  # Esc
                    return "", True
                if ch in (10, 13):  # Enter
                    return "".join(buffer).strip(), False
                if ch in (curses.KEY_BACKSPACE, 127, 8):
                    if buffer:
                        buffer.pop()
                    continue
                if 0 <= ch <= 255:
                    buffer.append(chr(ch))
        finally:
            curses.curs_set(0)
            self.stdscr.timeout(self.timeout_ms)

    def notify(self, text: str, error: bool = False) -> None:
        self.message = text
        self.message_error = error

    def start_entry(self) -> None:
        text, cancelled = self.prompt("Start entry: ")
        if cancelled:
            self.notify("Start cancelled.", False)
            return
        if not text:
            self.notify("Please enter a description.", True)
            return
        entries = read_entries()
        if find_open(entries) is not None:
            self.notify("An entry is already running.", True)
            return
        append_entry(Entry(start=to_utc(local_now()), end=None, text=text))
        self.reload_entries()
        self.notify(f"Started: {text}", False)

    def stop_entry(self) -> None:
        entries = read_entries()
        idx = find_open(entries)
        if idx is None:
            self.notify("No active entry to stop.", True)
            return
        now = to_utc(local_now())
        open_entry = entries[idx]
        if now <= open_entry.start:
            now = open_entry.start + timedelta(minutes=1)
        entries[idx] = Entry(start=open_entry.start, end=now, text=open_entry.text)
        write_entries(entries)
        self.reload_entries()
        self.notify(f"Stopped: {open_entry.text}", False)

    def loop(self) -> None:
        while True:
            now = utc_now()
            self.draw()
            key = self.stdscr.getch()
            if key == -1:
                continue
            if key in (ord("q"), 27):
                break
            if key in (curses.KEY_UP, ord("k")):
                self.move_selection(-1, now)
            elif key in (curses.KEY_DOWN, ord("j")):
                self.move_selection(1, now)
            elif key == ord("n"):
                self.start_entry()
            elif key == ord("x"):
                self.stop_entry()
            elif key == ord("r"):
                self.reload_entries()
                self.notify("Reloaded log.", False)
            elif key == ord("v"):
                self.range_mode = "week" if self.range_mode == "today" else "today"
                self.selected = 0
                self.notify(f"Switched to {self.range_mode} view.", False)


def launch_tui() -> None:
    def _run(stdscr: curses.window) -> None:
        ui = TerminalUI(stdscr)
        ui.loop()

    curses.wrapper(_run)
