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
        self.message = ""
        self.message_error = False
        self.focus_section = "day"  # "day" or "week"
        self.day_offset = 0
        self.week_offset = 0

        curses.curs_set(0)
        curses.start_color()
        curses.use_default_colors()
        curses.init_pair(1, curses.COLOR_BLACK, curses.COLOR_CYAN)  # selection
        curses.init_pair(2, curses.COLOR_RED, -1)  # errors
        curses.init_pair(3, curses.COLOR_GREEN, -1)  # success/info
        curses.init_pair(4, curses.COLOR_BLACK, curses.COLOR_GREEN)  # running badge
        curses.init_pair(5, curses.COLOR_BLACK, curses.COLOR_YELLOW)  # idle badge
        curses.init_pair(6, curses.COLOR_YELLOW, -1)  # idle text
        self.timeout_ms = 1000
        self.stdscr.timeout(self.timeout_ms)
        self.reload_entries()

    def reload_entries(self) -> None:
        self.entries = read_entries()

    def draw_box(self, y: int, x: int, height: int, width: int, title: str, highlight: bool = False) -> None:
        if height < 2 or width < 2:
            return
        right = x + width - 1
        bottom = y + height - 1
        ul = getattr(curses, "ACS_ULCORNER", ord("+"))
        ur = getattr(curses, "ACS_URCORNER", ord("+"))
        ll = getattr(curses, "ACS_LLCORNER", ord("+"))
        lr = getattr(curses, "ACS_LRCORNER", ord("+"))
        hline = getattr(curses, "ACS_HLINE", ord("-"))
        vline = getattr(curses, "ACS_VLINE", ord("|"))
        try:
            attr = curses.color_pair(3) | curses.A_BOLD if highlight else curses.A_NORMAL

            self.stdscr.addch(y, x, ul, attr)
            for col in range(x + 1, right):
                self.stdscr.addch(y, col, hline, attr)
            self.stdscr.addch(y, right, ur, attr)

            for row in range(y + 1, bottom):
                self.stdscr.addch(row, x, vline, attr)
                self.stdscr.addch(row, right, vline, attr)

            self.stdscr.addch(bottom, x, ll, attr)
            for col in range(x + 1, right):
                self.stdscr.addch(bottom, col, hline, attr)
            self.stdscr.addch(bottom, right, lr, attr)
            title_text = f" {title} "
            if len(title_text) < width - 2:
                self.stdscr.addstr(y, x + 2, title_text, attr)
        except curses.error:
            # Ignore drawing errors on very small terminals/resizes.
            pass

    def draw(self) -> None:
        self.stdscr.erase()
        now = utc_now()
        height, width = self.stdscr.getmaxyx()
        footer_height = 2
        content_height = max(0, height - footer_height)
        left_width = max(int(width * 0.38), 28)
        right_width = max(width - left_width - 1, 20)
        status_height = 3

        left_remaining = max(content_height - status_height, 0)
        day_height, week_height, top_height = self.allocate_left_heights(left_remaining)

        current_y = 0
        self.draw_status_box(current_y, 0, status_height, left_width, now)
        current_y += status_height
        self.draw_day_summary(current_y, 0, day_height, left_width, now)
        current_y += day_height
        self.draw_week_summary(current_y, 0, week_height, left_width, now)
        current_y += week_height
        self.draw_top_tasks(current_y, 0, top_height, left_width, now)

        right_x = left_width + 1
        min_current_height = 6
        comment_box_height = min(top_height, max(content_height - min_current_height, 0))
        current_task_height = max(content_height - comment_box_height, min_current_height)

        self.draw_current_task_box(0, right_x, current_task_height, right_width, now)
        self.draw_comment_log(current_task_height, right_x, comment_box_height, right_width, now)
        self.draw_footer(content_height, width)
        self.stdscr.refresh()

    def draw_status_box(self, y: int, x: int, height: int, width: int, now: datetime) -> None:
        if height < 2:
            return
        idx = find_open(self.entries)
        if idx is None:
            status = "IDLE"
            status_attr = curses.color_pair(6) | curses.A_BOLD
        else:
            status = "WORKING"
            status_attr = curses.color_pair(3) | curses.A_BOLD
        title = "[1]-Status"
        self.draw_box(y, x, height, width, title)
        self.addstr(y + 1, x + 2, status, status_attr, width - 3)

    def draw_day_summary(self, y: int, x: int, height: int, width: int, now: datetime) -> None:
        if height < 2:
            return
        start_utc, end_utc = self.day_window(now)
        summaries = self.summarize_entries(start_utc, end_utc, now)
        title = "[2]-Day summary"
        highlight = self.focus_section == "day"
        if highlight:
            title += " (scroll)"
        self.draw_box(y, x, height, width, title, highlight=highlight)
        inner_width = width - 2
        max_lines = max(0, height - 2)
        if max_lines <= 0:
            return
        clamped_offset = min(self.day_offset, max(0, len(summaries) - max_lines))
        self.day_offset = clamped_offset
        view = summaries[clamped_offset : clamped_offset + max_lines]
        for idx, (start_local, end_local, duration, text) in enumerate(view):
            line = f"{start_local:%H:%M}-{end_local:%H:%M} {format_duration(duration)} {text}"
            self.addstr(y + 1 + idx, x + 1, line, curses.A_NORMAL, inner_width)
        if not summaries:
            self.addstr(y + 1, x + 1, "No entries yet today.", curses.A_DIM, inner_width)

    def draw_week_summary(self, y: int, x: int, height: int, width: int, now: datetime) -> None:
        if height < 2:
            return
        start_utc, end_utc = self.week_window(now)
        summaries = self.summarize_entries(start_utc, end_utc, now)
        title = "[3]-Week summary"
        highlight = self.focus_section == "week"
        if highlight:
            title += " (scroll)"
        self.draw_box(y, x, height, width, title, highlight=highlight)
        inner_width = width - 2
        max_lines = max(0, height - 2)
        if max_lines <= 0:
            return
        clamped_offset = min(self.week_offset, max(0, len(summaries) - max_lines))
        self.week_offset = clamped_offset
        view = summaries[clamped_offset : clamped_offset + max_lines]
        for idx, (start_local, end_local, duration, text) in enumerate(view):
            line = f"{start_local:%a %H:%M}-{end_local:%H:%M} {format_duration(duration)} {text}"
            self.addstr(y + 1 + idx, x + 1, line, curses.A_NORMAL, inner_width)
        if not summaries:
            self.addstr(y + 1, x + 1, "No entries this week yet.", curses.A_DIM, inner_width)

    def draw_top_tasks(self, y: int, x: int, height: int, width: int, now: datetime) -> None:
        if height < 2:
            return
        week_start, week_end = self.week_window(now)
        top = self.top_tasks_for_range(week_start, week_end, now)
        self.draw_box(y, x, height, width, "[4]-Top tags")
        inner_width = width - 2
        max_lines = max(0, height - 2)
        for idx, (text, duration) in enumerate(top[:max_lines]):
            line = f"{format_duration(duration)} {text}"
            self.addstr(y + 1 + idx, x + 1, line, curses.A_NORMAL, inner_width)
        if not top:
            self.addstr(y + 1, x + 1, "No tracked time yet.", curses.A_DIM, inner_width)

    def draw_current_task_box(self, y: int, x: int, height: int, width: int, now: datetime) -> None:
        if height < 2:
            return
        self.draw_box(y, x, height, width, "[0]-Current task")
        inner_width = max(0, width - 2)
        inner_height = max(0, height - 2)
        idx = find_open(self.entries)
        tzinfo = now.astimezone().tzinfo
        if idx is None:
            lines = ["No active task."]
            closed = next((entry for entry in reversed(self.entries) if entry.end is not None), None)
            if closed:
                lines.append("")
                lines.append(f"Last: {closed.text}")
                lines.append(f"Ended: {closed.end.astimezone(tzinfo):%Y-%m-%d %H:%M}")
                lines.append(f"Length: {format_duration(closed.duration(closed.end))}")
        else:
            entry = self.entries[idx]
            start_local = entry.start.astimezone(tzinfo)
            elapsed = entry.duration(now)
            lines = [
                f"Task: {entry.text}",
                f"Start: {start_local:%Y-%m-%d %H:%M}",
                f"Elapsed: {format_duration(elapsed)}",
            ]
            tags = ", ".join(entry.tags()) or "(untagged)"
            lines.append(f"Tags: {tags}")
        self.render_wrapped(lines, y + 1, x + 1, inner_height, inner_width)

    def draw_comment_log(self, y: int, x: int, height: int, width: int, now: datetime) -> None:
        if height < 2:
            return
        self.draw_box(y, x, height, width, "Comment log")
        today_start, today_end = self.day_window(now)
        week_start, week_end = self.week_window(now)
        today_total = self.total_for_range(today_start, today_end, now)
        week_total = self.total_for_range(week_start, week_end, now)
        target_today = timedelta(hours=8)
        target_week = timedelta(hours=40)
        delta_today = target_today - today_total
        delta_week = target_week - week_total

        today_attr = curses.color_pair(2) if delta_today > timedelta(0) else curses.color_pair(3)
        week_attr = curses.color_pair(2) if delta_week > timedelta(0) else curses.color_pair(3)

        today_text = (
            f"Remaining today: {format_duration(delta_today)} to hit 08:00"
            if delta_today > timedelta(0)
            else f"Remaining today: +{format_duration(-delta_today)} over 08:00"
        )
        week_text = (
            f"Remaining week: {format_duration(delta_week)} to hit 40:00"
            if delta_week > timedelta(0)
            else f"Remaining week: +{format_duration(-delta_week)} over 40:00"
        )

        inner_width = width - 2
        self.addstr(y + 1, x + 1, today_text, today_attr, inner_width)
        if height >= 3:
            self.addstr(y + 2, x + 1, week_text, week_attr, inner_width)
        if height >= 4:
            message_attr = curses.color_pair(2) if self.message_error else curses.color_pair(3)
            self.addstr(y + 3, x + 1, self.message, message_attr, inner_width)

    def draw_footer(self, y: int, width: int) -> None:
        help_line = "[Tab] switch scroll target  [↑/↓] scroll  [n] start  [x] stop  [r] reload  [q] quit"
        action_line = "Actions: start new task, stop current, reload log"
        self.addstr(y, 0, help_line, curses.A_DIM, width)
        self.addstr(y + 1, 0, action_line, curses.A_DIM, width)

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

    def allocate_left_heights(self, total: int) -> Tuple[int, int, int]:
        """Allocate height across day/week/top with bias to week/top."""
        if total <= 0:
            return 0, 0, 0
        mins = [3, 5, 4]  # day, week, top
        base_sum = sum(mins)
        if total <= base_sum:
            # Fit into available space while keeping order.
            return tuple(self._shrink_to_total(mins, total))
        remaining = total - base_sum
        weights = [2, 3, 3]  # favor week/top
        total_weight = sum(weights)
        extra_day = int(remaining * weights[0] / total_weight)
        extra_week = int(remaining * weights[1] / total_weight)
        extra_top = remaining - extra_day - extra_week
        return (
            mins[0] + extra_day,
            mins[1] + extra_week,
            mins[2] + extra_top,
        )

    def _shrink_to_total(self, values: List[int], total: int) -> List[int]:
        if total <= 0:
            return [0] * len(values)
        result = values[:]
        while sum(result) > total and any(v > 1 for v in result):
            for idx in range(len(result)):
                if sum(result) <= total:
                    break
                if result[idx] > 1:
                    result[idx] -= 1
        return result

    def summarize_entries(
        self, start_utc: datetime, end_utc: datetime, now: datetime
    ) -> List[Tuple[datetime, datetime, timedelta, str]]:
        tzinfo = now.astimezone().tzinfo
        rows: List[Tuple[datetime, datetime, timedelta, str]] = []
        for entry in self.entries:
            if entry.end is None:
                continue
            overlap = clamp_duration(entry, start_utc, end_utc, now)
            if overlap <= timedelta(0):
                continue
            local_start = max(entry.start, start_utc).astimezone(tzinfo)
            local_end = min(entry.end, end_utc).astimezone(tzinfo)
            rows.append((local_start, local_end, overlap, entry.text))
        rows.sort(key=lambda row: row[0], reverse=True)
        return rows

    def top_tasks_for_range(
        self, start_utc: datetime, end_utc: datetime, now: datetime, limit: int = 6
    ) -> List[Tuple[str, timedelta]]:
        totals: dict[str, timedelta] = {}
        for entry in self.entries:
            duration = clamp_duration(entry, start_utc, end_utc, now)
            if duration <= timedelta(0):
                continue
            tags = entry.tags() or ["(untagged)"]
            for tag in tags:
                totals[tag] = totals.get(tag, timedelta(0)) + duration
        ordered = sorted(totals.items(), key=lambda item: item[1], reverse=True)
        return ordered[:limit]

    def render_wrapped(
        self,
        lines: List[str],
        y: int,
        x: int,
        max_height: int,
        max_width: int,
        attr: int = curses.A_NORMAL,
    ) -> None:
        if max_height <= 0 or max_width <= 0:
            return
        current_row = y
        for line in lines:
            for segment in textwrap.wrap(line, width=max_width):
                if current_row >= y + max_height:
                    return
                self.addstr(current_row, x, segment, attr, max_width)
                current_row += 1

    def prompt(self, prompt_text: str) -> tuple[str, bool]:
        """Prompt for input; returns (text, cancelled). Esc cancels."""
        height, width = self.stdscr.getmaxyx()
        buffer: List[str] = []
        box_width = max(30, min(width - 4, len(prompt_text) + 20))
        box_height = 5
        start_y = max((height - box_height) // 2, 0)
        start_x = max((width - box_width) // 2, 0)
        win = curses.newwin(box_height, box_width, start_y, start_x)
        win.keypad(True)

        curses.curs_set(1)
        self.stdscr.timeout(-1)  # block while typing so we don't auto-cancel
        try:
            while True:
                win.erase()
                win.box()
                win.addstr(1, 2, prompt_text[: box_width - 4], curses.A_BOLD)
                display = "".join(buffer)
                win.addstr(2, 2, display[: box_width - 4])
                win.move(2, min(2 + len(display), box_width - 3))
                win.refresh()
                ch = win.getch()
                if ch in (27,):  # Esc
                    return "", True
                if ch in (10, 13):  # Enter
                    return display.strip(), False
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

    def scroll_active(self, delta: int) -> None:
        if self.focus_section == "day":
            self.day_offset = max(0, self.day_offset + delta)
        else:
            self.week_offset = max(0, self.week_offset + delta)

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
                self.scroll_active(-1)
            elif key in (curses.KEY_DOWN, ord("j")):
                self.scroll_active(1)
            elif key in (ord("\t"),):
                self.focus_section = "week" if self.focus_section == "day" else "day"
            if key == ord("n"):
                self.start_entry()
            elif key == ord("x"):
                self.stop_entry()
            elif key == ord("r"):
                self.reload_entries()
                self.notify("Reloaded log.", False)


def launch_tui() -> None:
    def _run(stdscr: curses.window) -> None:
        ui = TerminalUI(stdscr)
        ui.loop()

    curses.wrapper(_run)
